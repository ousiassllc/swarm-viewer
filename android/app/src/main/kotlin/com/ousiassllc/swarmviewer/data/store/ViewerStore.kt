package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Settings
import java.time.Instant

/** 取得に成功した `report` と、その取得時刻（viewer が取得に成功した時刻）。 */
data class Snapshot(
    val report: Report,
    val fetchedAt: Instant,
)

/**
 * 永続化の窓口（デスクトップ版の `service.Store` に相当。ER §7）。サービス層はこの interface だけに依存し、
 * テストではフェイクに差し替える。token は扱わない（[com.ousiassllc.swarmviewer.data.secret.TokenStore] の責務）。
 * 失敗はデータベースの例外（`SQLiteException` など）のまま伝える。
 */
interface ViewerStore {
    // connections

    /** 保存済みの接続を作成時刻の昇順で返す。 */
    suspend fun connections(): List<Connection>

    /** [id] の接続。無ければ null。 */
    suspend fun connection(id: String): Connection?

    /** [url] の接続。無ければ null。 */
    suspend fun connectionByUrl(url: String): Connection?

    /**
     * 接続を保存する（url で一意の UPSERT）。同じ url の行があれば、その行の id・last_session・baselined_at・created_at を
     * 保ったまま name と updated_at だけを置き換える（再ペアリング。ER §3.1）。無ければ [connection] をそのまま挿入する。
     * [Connection.createdAt]・[Connection.updatedAt] が null なら現在時刻を使う。
     */
    suspend fun saveConnection(connection: Connection)

    /**
     * 接続を削除する。snapshots・notified_states は連鎖削除され、settings.last_connection_id は NULL に戻る。
     * 無い id は何もしない。token は消さない（呼び出し側が先に [com.ousiassllc.swarmviewer.data.secret.TokenStore.delete] する。ER §7.3）。
     */
    suspend fun deleteConnection(id: String)

    /** 最後に選んだセッションを記録する（null は NULL に戻す）。接続が無ければ false。 */
    suspend fun setLastSession(
        id: String,
        session: String?,
    ): Boolean

    /** 初回の取得パスの完了時刻を記録する。記録済みなら最初の時刻を保つ。接続が無ければ false。 */
    suspend fun markBaselined(
        id: String,
        at: Instant,
    ): Boolean

    // settings（1 行固定）

    /** 設定を返す。まだ保存されていなければ既定値。 */
    suspend fun settings(): Settings

    /** 設定を保存する。許容外の値は [com.ousiassllc.swarmviewer.domain.SettingsException]。 */
    suspend fun saveSettings(settings: Settings)

    /** `settings.last_connection_id` だけを更新する（null は NULL）。存在しない接続 ID は外部キー違反。 */
    suspend fun setLastConnection(id: String?)

    // snapshots

    /** 接続×セッションの最後の `report`。無い・版が合わない・JSON が壊れているときは null（起動を止めない。ER §3.3）。 */
    suspend fun snapshot(
        connectionId: String,
        session: String,
    ): Snapshot?

    /** 接続×セッションの最新 1 件を UPSERT する（履歴は持たない）。接続が無ければ外部キー違反。 */
    suspend fun saveSnapshot(
        connectionId: String,
        session: String,
        snapshot: Snapshot,
    )

    /** [keep] に無いセッションのスナップショットを接続から削除する（`sessions_list` から消えたセッション分）。空なら全削除。 */
    suspend fun pruneSnapshots(
        connectionId: String,
        keep: List<String>,
    )

    // notified_states

    suspend fun notifiedKeys(
        connectionId: String,
        session: String,
    ): List<NotifiedKey>

    /**
     * [Diff] の delete・insert を 1 トランザクションで適用し、実際に挿入できたキーだけを返す。
     * INSERT は主キー `(接続, セッション, ペイン, 状態)` が重複すると挿入されない（二重通知の最終防衛線）。
     * 適用後、接続あたり [MAX_NOTIFIED_PER_CONNECTION] 件を超えた分を古い順に削除する。接続が無ければ外部キー違反。
     */
    suspend fun applyNotified(
        connectionId: String,
        session: String,
        diff: Diff,
        at: Instant,
    ): List<NotifiedKey>
}

/** 接続あたりの通知済み記録の上限（NFR-DAT-3）。 */
const val MAX_NOTIFIED_PER_CONNECTION = 1000
