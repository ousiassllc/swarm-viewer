package com.ousiassllc.swarmviewer.data.store

import androidx.room.withTransaction
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.InvalidResponseException
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Settings
import java.time.Clock
import java.time.Instant
import java.time.format.DateTimeParseException
import java.time.temporal.ChronoUnit

/** [ViewerStore] の Room 実装。時刻は UTC の RFC 3339 文字列（秒精度）で保存する。 */
class RoomViewerStore(
    private val db: SwarmDatabase,
    private val clock: Clock = Clock.systemUTC(),
) : ViewerStore {
    private val connectionDao = db.connections()
    private val settingsDao = db.settings()
    private val snapshotDao = db.snapshots()
    private val notifiedDao = db.notifiedStates()

    override suspend fun connections(): List<Connection> = connectionDao.all().map { it.toDomain() }

    override suspend fun connection(id: String): Connection? = connectionDao.byId(id)?.toDomain()

    override suspend fun connectionByUrl(url: String): Connection? = connectionDao.byUrl(url)?.toDomain()

    override suspend fun saveConnection(connection: Connection) {
        val now = clock.instant()
        connectionDao.save(
            id = connection.id,
            name = connection.name,
            url = connection.url,
            lastSession = connection.lastSession,
            baselinedAt = connection.baselinedAt?.format(),
            createdAt = (connection.createdAt ?: now).format(),
            updatedAt = (connection.updatedAt ?: now).format(),
        )
    }

    override suspend fun deleteConnection(id: String) {
        connectionDao.delete(id)
    }

    override suspend fun setLastSession(
        id: String,
        session: String?,
    ): Boolean = connectionDao.setLastSession(id, session, clock.instant().format()) > 0

    override suspend fun markBaselined(
        id: String,
        at: Instant,
    ): Boolean = connectionDao.markBaselined(id, at.format()) > 0

    override suspend fun settings(): Settings = settingsDao.get()?.toDomain() ?: Settings()

    override suspend fun saveSettings(settings: Settings) {
        settings.validate()
        settingsDao.save(settings.toEntity(clock.instant()))
    }

    override suspend fun setLastConnection(id: String?) {
        db.withTransaction {
            saveSettings(settings().copy(lastConnectionId = id))
        }
    }

    override suspend fun snapshot(
        connectionId: String,
        session: String,
    ): Snapshot? {
        val row = snapshotDao.get(connectionId, session) ?: return null
        if (row.schemaVersion != SNAPSHOT_SCHEMA_VERSION) return null
        return try {
            Snapshot(report = decodeReport(row.reportJson), fetchedAt = parseTime(row.fetchedAt))
        } catch (e: InvalidResponseException) {
            null // 壊れた行は「スナップショットなし」として扱う（ER §3.3）
        } catch (e: DateTimeParseException) {
            null
        }
    }

    override suspend fun saveSnapshot(
        connectionId: String,
        session: String,
        snapshot: Snapshot,
    ) {
        snapshotDao.save(
            SnapshotEntity(
                connectionId = connectionId,
                sessionId = session,
                fetchedAt = snapshot.fetchedAt.format(),
                schemaVersion = SNAPSHOT_SCHEMA_VERSION,
                reportJson = encodeReport(snapshot.report),
            ),
        )
    }

    override suspend fun pruneSnapshots(
        connectionId: String,
        keep: List<String>,
    ) {
        if (keep.isEmpty()) snapshotDao.deleteAll(connectionId) else snapshotDao.pruneExcept(connectionId, keep)
    }

    override suspend fun notifiedKeys(
        connectionId: String,
        session: String,
    ): List<NotifiedKey> = notifiedDao.forSession(connectionId, session).mapNotNull { it.toKeyOrNull() }

    override suspend fun applyNotified(
        connectionId: String,
        session: String,
        diff: Diff,
        at: Instant,
    ): List<NotifiedKey> =
        db.withTransaction {
            for (key in diff.delete) notifiedDao.delete(connectionId, session, key.paneId, key.state.id)
            val inserted = ArrayList<NotifiedKey>()
            for (key in diff.insert) {
                require(key.state.notifiable) { "only asking / stalled can be recorded" } // Room は CHECK 制約を宣言できないため、ここで強制する
                val row = NotifiedStateEntity(connectionId, session, key.paneId, key.state.id, at.format())
                if (notifiedDao.insertIfAbsent(row) != IGNORED) inserted.add(key)
            }
            notifiedDao.trim(connectionId, MAX_NOTIFIED_PER_CONNECTION)
            inserted
        }
}

private const val IGNORED = -1L

private fun Instant.format(): String = truncatedTo(ChronoUnit.SECONDS).toString()

private fun parseTime(text: String): Instant = Instant.parse(text)

private fun ConnectionEntity.toDomain(): Connection =
    Connection(
        id = id,
        name = name,
        url = url,
        lastSession = lastSession,
        baselinedAt = baselinedAt?.let(::parseTime),
        createdAt = parseTime(createdAt),
        updatedAt = parseTime(updatedAt),
    )

private fun SettingsEntity.toDomain(): Settings =
    Settings(
        notificationsEnabled = notificationsEnabled,
        screenPollSeconds = screenPollSeconds,
        notifyPollMinutes = notifyPollMinutes,
        debugLog = debugLog,
        lastConnectionId = lastConnectionId,
    )

private fun Settings.toEntity(now: Instant): SettingsEntity =
    SettingsEntity(
        notificationsEnabled = notificationsEnabled,
        screenPollSeconds = screenPollSeconds,
        notifyPollMinutes = notifyPollMinutes,
        debugLog = debugLog,
        lastConnectionId = lastConnectionId,
        updatedAt = now.format(),
    )

// 通知対象外の状態の行は作らない。読めない状態の行（将来の版など）は無視する。
private fun NotifiedStateEntity.toKeyOrNull(): NotifiedKey? =
    PaneState
        .parse(state)
        .takeIf {
            it.notifiable
        }?.let { NotifiedKey(paneId, it) }
