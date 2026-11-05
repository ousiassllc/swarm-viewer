package com.ousiassllc.swarmviewer.data.store

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.PrimaryKey

// Room のエンティティ（ER §7.2）。token の列は持たない（token は data/secret の保護領域だけ。NFR-SEC-2）。
// 時刻は UTC の RFC 3339 文字列（秒精度）、真偽値は INTEGER（0 / 1）。Room は CHECK 制約を宣言できないため、
// 許容値は domain の検証（Settings.validate、NotifiedKey の状態）で強制する。

@Entity(tableName = "connections", indices = [Index(value = ["url"], unique = true)])
data class ConnectionEntity(
    @PrimaryKey val id: String,
    val name: String,
    val url: String,
    @ColumnInfo(name = "last_session") val lastSession: String?,
    @ColumnInfo(name = "baselined_at") val baselinedAt: String?,
    @ColumnInfo(name = "created_at") val createdAt: String,
    @ColumnInfo(name = "updated_at") val updatedAt: String,
)

/** 1 行固定（[SINGLETON_SETTINGS_ID]）。最後に使った接続の削除で [lastConnectionId] は NULL に戻る。 */
@Entity(
    tableName = "settings",
    foreignKeys = [
        ForeignKey(
            entity = ConnectionEntity::class,
            parentColumns = ["id"],
            childColumns = ["last_connection_id"],
            onDelete = ForeignKey.SET_NULL,
        ),
    ],
    indices = [Index(value = ["last_connection_id"])],
)
data class SettingsEntity(
    @PrimaryKey val id: Int = SINGLETON_SETTINGS_ID,
    @ColumnInfo(name = "notifications_enabled") val notificationsEnabled: Boolean,
    @ColumnInfo(name = "screen_poll_seconds") val screenPollSeconds: Int,
    @ColumnInfo(name = "notify_poll_minutes") val notifyPollMinutes: Int,
    @ColumnInfo(name = "debug_log") val debugLog: Boolean,
    @ColumnInfo(name = "last_connection_id") val lastConnectionId: String?,
    @ColumnInfo(name = "updated_at") val updatedAt: String,
)

const val SINGLETON_SETTINGS_ID = 1

/** 接続×セッションごとに最新 1 件の `report`。[reportJson] は domain の Report を [SnapshotCodec] で JSON にしたもの。 */
@Entity(
    tableName = "snapshots",
    primaryKeys = ["connection_id", "session_id"],
    foreignKeys = [
        ForeignKey(
            entity = ConnectionEntity::class,
            parentColumns = ["id"],
            childColumns = ["connection_id"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
)
data class SnapshotEntity(
    @ColumnInfo(name = "connection_id") val connectionId: String,
    @ColumnInfo(name = "session_id") val sessionId: String,
    @ColumnInfo(name = "fetched_at") val fetchedAt: String,
    @ColumnInfo(name = "schema_version") val schemaVersion: Int,
    @ColumnInfo(name = "report_json") val reportJson: String,
)

/**
 * 通知済みの `(接続, セッション, ペイン, 状態)`。**この主キー制約が二重通知の防止になる**
 * （INSERT が成功したときだけ通知する。Android 版の設計 §3.2）。
 */
@Entity(
    tableName = "notified_states",
    primaryKeys = ["connection_id", "session_id", "pane_id", "state"],
    foreignKeys = [
        ForeignKey(
            entity = ConnectionEntity::class,
            parentColumns = ["id"],
            childColumns = ["connection_id"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
    indices = [Index(value = ["connection_id", "notified_at"])],
)
data class NotifiedStateEntity(
    @ColumnInfo(name = "connection_id") val connectionId: String,
    @ColumnInfo(name = "session_id") val sessionId: String,
    @ColumnInfo(name = "pane_id") val paneId: String,
    val state: String,
    @ColumnInfo(name = "notified_at") val notifiedAt: String,
)
