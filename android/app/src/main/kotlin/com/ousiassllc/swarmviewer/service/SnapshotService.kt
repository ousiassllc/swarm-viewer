package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Session
import java.time.Clock

/** 許可されたセッションの一覧の取得（`sessions_list`）。失敗は domain の `FetchException`。 */
fun interface SessionLister {
    suspend fun sessions(connection: Connection): List<Session>
}

/** 取得の途中で、対象の接続が削除された。 */
class ConnectionGoneException : Exception("connection no longer exists")

/**
 * 画面更新の取得（選択中セッションの `report` 1 本）と、その結果の保存（機能要件 §12.1、FR-AND-8）。
 * 取得に成功したら、接続×セッションごとの最新 1 件のスナップショットを保存する（履歴は持たない。ER §3.3）。
 * 失敗は何も変えずに domain の `FetchException` を伝える。
 */
class SnapshotService(
    private val lister: SessionLister,
    private val reports: ReportFetcher,
    private val store: ViewerStore,
    private val clock: Clock = Clock.systemUTC(),
) {
    /**
     * 接続の許可セッション（`allowed: true`）の ID を、`sessions_list` の順で返す。
     * 一覧から消えたセッションのスナップショットは削除する。
     */
    suspend fun allowedSessions(connectionId: String): List<String> {
        val connection = store.connection(connectionId) ?: throw ConnectionGoneException()
        val allowed = lister.sessions(connection).filter { it.allowed }.map { it.id }
        store.pruneSnapshots(connectionId, allowed)
        return allowed
    }

    /** 最後に選んだセッション（接続ごとの記憶）。未設定・接続が無いときは null。 */
    suspend fun lastSession(connectionId: String): String? = store.connection(connectionId)?.lastSession

    /** 選んだセッションを接続ごとに記憶する。 */
    suspend fun remember(
        connectionId: String,
        session: String,
    ) {
        store.setLastSession(connectionId, session)
    }

    /** 前回取得したスナップショット。無い・読めないときは null。 */
    suspend fun cached(
        connectionId: String,
        session: String,
    ): Snapshot? = store.snapshot(connectionId, session)

    /** [session] の `report` を取得して保存し、保存したスナップショットを返す。 */
    suspend fun refresh(
        connectionId: String,
        session: String,
    ): Snapshot {
        val connection = store.connection(connectionId) ?: throw ConnectionGoneException()
        val snapshot = Snapshot(reports.report(connection, session), clock.instant())
        store.saveSnapshot(connectionId, session, snapshot)
        return snapshot
    }
}
