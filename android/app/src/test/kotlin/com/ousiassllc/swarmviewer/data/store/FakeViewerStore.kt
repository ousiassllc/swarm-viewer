package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.Settings
import java.time.Instant

/**
 * メモリ上の [ViewerStore]（JVM のテスト用）。Room 実装と同じ規則（url で一意の UPSERT、接続削除の連鎖）を持つ。
 * [failDelete] を立てると [deleteConnection] が失敗する（行の削除に失敗する場合の再現用）。
 */
internal class FakeViewerStore : ViewerStore {
    val connectionRows = LinkedHashMap<String, Connection>()
    val snapshotRows = HashMap<Pair<String, String>, Snapshot>()
    val notifiedRows = LinkedHashSet<List<String>>() // 接続・セッション・ペイン・状態
    var settingsRow: Settings? = null
    var failDelete = false

    override suspend fun connections(): List<Connection> = connectionRows.values.toList()

    override suspend fun connection(id: String): Connection? = connectionRows[id]

    override suspend fun connectionByUrl(url: String): Connection? = connectionRows.values.firstOrNull { it.url == url }

    override suspend fun saveConnection(connection: Connection) {
        val existing = connectionByUrl(connection.url)
        if (existing == null) {
            connectionRows[connection.id] = connection
        } else {
            connectionRows[existing.id] = existing.copy(name = connection.name, updatedAt = connection.updatedAt)
        }
    }

    override suspend fun deleteConnection(id: String) {
        if (failDelete) error("delete failed")
        connectionRows.remove(id)
        snapshotRows.keys.removeAll { it.first == id }
        notifiedRows.removeAll { it[0] == id }
        if (settingsRow?.lastConnectionId == id) settingsRow = settingsRow?.copy(lastConnectionId = null)
    }

    override suspend fun setLastSession(
        id: String,
        session: String?,
    ): Boolean = connectionRows[id]?.let { connectionRows[id] = it.copy(lastSession = session) } != null

    override suspend fun markBaselined(
        id: String,
        at: Instant,
    ): Boolean = connectionRows[id]?.let { connectionRows[id] = it.copy(baselinedAt = it.baselinedAt ?: at) } != null

    override suspend fun settings(): Settings = settingsRow ?: Settings()

    override suspend fun saveSettings(settings: Settings) {
        settings.validate()
        settingsRow = settings
    }

    override suspend fun setLastConnection(id: String?) {
        settingsRow = settings().copy(lastConnectionId = id)
    }

    override suspend fun snapshot(
        connectionId: String,
        session: String,
    ): Snapshot? = snapshotRows[connectionId to session]

    override suspend fun saveSnapshot(
        connectionId: String,
        session: String,
        snapshot: Snapshot,
    ) {
        snapshotRows[connectionId to session] = snapshot
    }

    override suspend fun pruneSnapshots(
        connectionId: String,
        keep: List<String>,
    ) {
        snapshotRows.keys.removeAll { it.first == connectionId && it.second !in keep }
    }

    override suspend fun notifiedKeys(
        connectionId: String,
        session: String,
    ): List<NotifiedKey> =
        notifiedRows
            .filter { it[0] == connectionId && it[1] == session }
            .map {
                NotifiedKey(
                    it[2],
                    com.ousiassllc.swarmviewer.domain.PaneState
                        .parse(it[3]),
                )
            }

    override suspend fun applyNotified(
        connectionId: String,
        session: String,
        diff: Diff,
        at: Instant,
    ): List<NotifiedKey> {
        diff.delete.forEach { notifiedRows.remove(listOf(connectionId, session, it.paneId, it.state.id)) }
        return diff.insert.filter { notifiedRows.add(listOf(connectionId, session, it.paneId, it.state.id)) }
    }
}
