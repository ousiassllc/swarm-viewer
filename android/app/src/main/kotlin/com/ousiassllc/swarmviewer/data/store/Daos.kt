package com.ousiassllc.swarmviewer.data.store

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Upsert

@Dao
interface ConnectionDao {
    @Query("SELECT * FROM connections ORDER BY created_at ASC, rowid ASC")
    suspend fun all(): List<ConnectionEntity>

    @Query("SELECT * FROM connections WHERE id = :id")
    suspend fun byId(id: String): ConnectionEntity?

    @Query("SELECT * FROM connections WHERE url = :url")
    suspend fun byUrl(url: String): ConnectionEntity?

    /**
     * url で一意の UPSERT。同じ url の行があれば、その行の id・last_session・baselined_at・created_at を保ったまま
     * name と updated_at だけを置き換える（再ペアリング。ER §3.1）。
     */
    @Query(
        """
        INSERT INTO connections (id, name, url, last_session, baselined_at, created_at, updated_at)
        VALUES (:id, :name, :url, :lastSession, :baselinedAt, :createdAt, :updatedAt)
        ON CONFLICT (url) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at
        """,
    )
    suspend fun save(
        id: String,
        name: String,
        url: String,
        lastSession: String?,
        baselinedAt: String?,
        createdAt: String,
        updatedAt: String,
    )

    /** 外部キーにより、snapshots・notified_states は連鎖削除され、settings.last_connection_id は NULL に戻る。 */
    @Query("DELETE FROM connections WHERE id = :id")
    suspend fun delete(id: String): Int

    @Query("UPDATE connections SET last_session = :session, updated_at = :now WHERE id = :id")
    suspend fun setLastSession(
        id: String,
        session: String?,
        now: String,
    ): Int

    @Query("UPDATE connections SET baselined_at = COALESCE(baselined_at, :at) WHERE id = :id")
    suspend fun markBaselined(
        id: String,
        at: String,
    ): Int
}

@Dao
interface SettingsDao {
    @Query("SELECT * FROM settings WHERE id = $SINGLETON_SETTINGS_ID")
    suspend fun get(): SettingsEntity?

    @Upsert
    suspend fun save(entity: SettingsEntity)
}

@Dao
interface SnapshotDao {
    @Query("SELECT * FROM snapshots WHERE connection_id = :connectionId AND session_id = :sessionId")
    suspend fun get(
        connectionId: String,
        sessionId: String,
    ): SnapshotEntity?

    @Upsert
    suspend fun save(entity: SnapshotEntity)

    @Query("DELETE FROM snapshots WHERE connection_id = :connectionId AND session_id NOT IN (:keep)")
    suspend fun pruneExcept(
        connectionId: String,
        keep: List<String>,
    )

    @Query("DELETE FROM snapshots WHERE connection_id = :connectionId")
    suspend fun deleteAll(connectionId: String)
}

@Dao
interface NotifiedStateDao {
    @Query("SELECT * FROM notified_states WHERE connection_id = :connectionId AND session_id = :sessionId")
    suspend fun forSession(
        connectionId: String,
        sessionId: String,
    ): List<NotifiedStateEntity>

    @Query(
        """
        DELETE FROM notified_states
        WHERE connection_id = :connectionId AND session_id = :sessionId AND pane_id = :paneId AND state = :state
        """,
    )
    suspend fun delete(
        connectionId: String,
        sessionId: String,
        paneId: String,
        state: String,
    )

    /** 主キーが重複する行は挿入せず -1 を返す（二重通知の最終防衛線）。 */
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insertIfAbsent(entity: NotifiedStateEntity): Long

    /** 接続あたりの件数が [keep] を超えた分を、古い順に削除する（NFR-DAT-3）。 */
    @Query(
        """
        DELETE FROM notified_states WHERE rowid IN (
            SELECT rowid FROM notified_states WHERE connection_id = :connectionId
            ORDER BY notified_at DESC, rowid DESC
            LIMIT -1 OFFSET :keep
        )
        """,
    )
    suspend fun trim(
        connectionId: String,
        keep: Int,
    )
}
