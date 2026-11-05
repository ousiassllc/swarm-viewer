package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.secret.TokenStore
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.PairingPayload
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.domain.detect
import com.ousiassllc.swarmviewer.domain.hostOf
import com.ousiassllc.swarmviewer.domain.parsePairing
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.util.UUID

/** ペアリング下書きの有効期間（作成から。確定に失敗しても延長しない）。 */
val DRAFT_TTL: Duration = Duration.ofMinutes(5)

/** 疎通確認（`sessions_list`）。保存前なので token を直接受け取る。失敗は domain の `FetchException`。 */
fun interface ConnectionProbe {
    suspend fun probe(
        url: String,
        token: String,
    ): List<Session>
}

/** 1 セッションの `report` の取得（新規ペアリング直後の最初の一巡で使う）。失敗は domain の `FetchException`。 */
fun interface ReportFetcher {
    suspend fun report(
        connection: Connection,
        session: String,
    ): Report
}

/**
 * 確認画面に出す下書きの情報。**token を持たない**（token は [PairingService] の中だけにある）。
 * [host] は URL の host のみ。[sessionsHint] は「ホストが申告した許可セッション」で表示のヒントにすぎない（権限の正は `sessions_list`）。
 * [replacesExisting] は同じ接続先が保存済みで、確定すると token の差し替えになること。
 */
data class PairingDraft(
    val id: String,
    val name: String,
    val host: String,
    val sessionsHint: List<String>,
    val replacesExisting: Boolean,
)

/** 下書きが無い・期限切れ・別の下書きに置き換わった。 */
class DraftExpiredException : Exception("pairing draft expired")

/** token の保存に失敗した（保存しないまま終わる。原因・token は含めない）。 */
class PairingSaveException : Exception("saving the connection failed")

private class Draft(
    val id: String,
    val payload: PairingPayload,
    val expiresAt: Instant,
)

/**
 * 接続文字列の解析・下書きの管理・疎通確認・接続の保存（FR-PAIR-1〜4、機能要件 §12.3）。
 * 貼り付け・ディープリンク・QR のどれも [parse] に入る。下書き（token を含む）はメモリだけに置き、同時に 1 件。
 * 保存するのは、疎通確認に成功したときだけ（[confirm]）。
 */
class PairingService(
    private val probe: ConnectionProbe,
    private val reports: ReportFetcher,
    private val store: ViewerStore,
    private val tokens: TokenStore,
    private val connections: ConnectionService,
    private val clock: Clock = Clock.systemUTC(),
    private val newId: () -> String = { UUID.randomUUID().toString() },
    private val io: CoroutineDispatcher = Dispatchers.IO,
) {
    private val lock = Any()
    private var draft: Draft? = null
    private val confirmMutex = Mutex()

    /**
     * 接続文字列を検証して下書きを作る（古い下書きは破棄する）。形式・版・URL・token の不正は
     * [com.ousiassllc.swarmviewer.domain.PairingException]（入力値を含まない）。
     * 呼び出し側は [raw] をすぐ捨てること（保存・ログ出力しない。NFR-SEC-7）。
     */
    suspend fun parse(raw: String): PairingDraft {
        val payload = parsePairing(raw)
        val replaces = store.connectionByUrl(payload.url) != null
        val created = Draft(newId(), payload, clock.instant().plus(DRAFT_TTL))
        synchronized(lock) { draft = created }
        return PairingDraft(created.id, payload.name, hostOf(payload.url), payload.sessionsHint, replaces)
    }

    /** 下書きを即時に破棄する（token ごとメモリから消える）。別の下書きに置き換わっていれば何もしない。 */
    fun discard(id: String) {
        synchronized(lock) { if (draft?.id == id) draft = null }
    }

    /**
     * 疎通確認に成功したときだけ接続を保存する。失敗しても下書きは残る（再試行できる。期限は延長しない）。
     *  - 疎通確認の失敗は `FetchException`（保存しない）
     *  - 同じ URL の接続があれば id・`baselined_at` を保ち、名前と token を差し替える。無ければ新しい ID で `baselined_at` なしで作る
     *  - 保存のあと、`baselined_at` が未記録なら最初の一巡（全許可セッションを順に取得し、今ある `asking` / `stalled` を通知せず
     *    通知済みとして記録する。ER §3.4・機能要件 §8.2）を行い、全セッションが取れたときだけ `baselined_at` を記録する。
     *    取れなかったセッションがあれば未記録のまま残り、通知側（Issue #21）の次の取得が続きを行う。一巡の失敗でペアリングは失敗にしない
     *  - token の保存に失敗したら、新規に作った行は削除して [PairingSaveException]
     *  - 最後に使った接続が未設定なら、この接続にする
     */
    suspend fun confirm(draftId: String): Connection =
        confirmMutex.withLock {
            val current = lookup(draftId)
            val payload = current.payload
            val sessions = probe.probe(payload.url, payload.token)
            lookup(draftId) // 疎通確認の間に破棄・置換されていたら保存しない
            val (connection, created) = save(payload)
            try {
                withContext(io) { tokens.put(connection.id, payload.token) }
            } catch (e: CancellationException) {
                if (created) rollback(connection.id)
                throw e
            } catch (e: Exception) {
                if (created) rollback(connection.id)
                throw PairingSaveException() // 例外の本文（鍵・パスの情報）は捨てる
            }
            connections.markRepaired(connection.id)
            if (store.settings().lastConnectionId == null) store.setLastConnection(connection.id)
            discard(draftId)
            if (connection.baselinedAt == null) baseline(connection, sessions)
            store.connection(connection.id) ?: connection
        }

    /**
     * 新規ペアリング直後の最初の一巡。各セッションの今の状態を通知せずに記録し（`detect` の baselining）、
     * 全セッションで成功したときだけ `baselined_at` を記録する。個々の失敗は飛ばして次へ進む（通知済み記録は変えない）。
     */
    private suspend fun baseline(
        connection: Connection,
        sessions: List<Session>,
    ) {
        var complete = true
        for (session in sessions) {
            try {
                val report = reports.report(connection, session.id)
                val diff = detect(store.notifiedKeys(connection.id, session.id), report.panes, baselining = true)
                store.applyNotified(connection.id, session.id, diff, clock.instant())
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                complete = false // 取得・保存の失敗。原因（URL・本文を含みうる）は使わない
            }
        }
        if (complete) store.markBaselined(connection.id, clock.instant())
    }

    private fun lookup(id: String): Draft =
        synchronized(lock) {
            val current = draft?.takeIf { it.id == id } ?: throw DraftExpiredException()
            if (!clock.instant().isBefore(current.expiresAt)) {
                draft = null
                throw DraftExpiredException()
            }
            current
        }

    /** URL で接続を引き、あれば名前と更新時刻だけ差し替え、無ければ新規に保存する。 */
    private suspend fun save(payload: PairingPayload): Pair<Connection, Boolean> {
        val now = clock.instant()
        val existing = store.connectionByUrl(payload.url)
        val connection =
            existing?.copy(name = payload.name, updatedAt = now)
                ?: Connection(newId(), payload.name, payload.url, createdAt = now, updatedAt = now)
        store.saveConnection(connection)
        return connection to (existing == null)
    }

    // 行だけ残る不整合を防ぐ。呼び出し元が中断していても戻す。
    private suspend fun rollback(id: String) {
        withContext(NonCancellable) { runCatching { store.deleteConnection(id) } }
    }
}
