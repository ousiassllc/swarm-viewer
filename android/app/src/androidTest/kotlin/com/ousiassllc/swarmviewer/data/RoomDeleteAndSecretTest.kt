package com.ousiassllc.swarmviewer.data

import androidx.room.Room
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.ousiassllc.swarmviewer.data.store.ConnectionRepository
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.Settings
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/** 接続の削除と token の非保存（実際の SQLite で確かめる）。 */
@RunWith(AndroidJUnit4::class)
internal class RoomDeleteAndSecretTest : RoomStoreTestBase() {
    @Test
    fun deletingAConnectionRemovesItsTokenSnapshotsAndNotifiedStates() =
        runBlocking {
            val repository = ConnectionRepository(store, tokens)
            for ((id, url) in listOf("a" to URL_1, "b" to URL_2)) {
                store.saveConnection(connection(id, url))
                tokens.put(id, "$ROOM_TOKEN-$id")
                store.saveSnapshot(id, "0", Snapshot(report(), now))
                store.applyNotified(id, "0", Diff(insert = listOf(key("%1"))), now)
            }
            store.setLastConnection("a")

            repository.delete("a")

            assertNull(store.connection("a"))
            assertNull(tokens.get("a"))
            assertNull(store.snapshot("a", "0"))
            assertEquals(emptyList<NotifiedKey>(), store.notifiedKeys("a", "0"))
            assertNull(store.settings().lastConnectionId)
            // ほかの接続は残る
            assertNotNull(store.connection("b"))
            assertEquals("$ROOM_TOKEN-b", tokens.get("b"))
            assertNotNull(store.snapshot("b", "0"))
            assertEquals(1, store.notifiedKeys("b", "0").size)
            assertEquals(1, count("snapshots"))
            assertEquals(1, count("notified_states"))
        }

    @Test
    fun noTableHasATokenColumn() {
        val sql = db.openHelper.readableDatabase
        val tables =
            sql
                .query(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'room_%' AND name != 'android_metadata'",
                ).use { c ->
                    generateSequence { if (c.moveToNext()) c.getString(0) else null }.toList()
                }
        assertEquals(setOf("connections", "settings", "snapshots", "notified_states"), tables.toSet())
        for (table in tables) {
            val columns =
                sql.query("PRAGMA table_info($table)").use { c ->
                    generateSequence { if (c.moveToNext()) c.getString(1) else null }.toList()
                }
            assertTrue(table, columns.none { it.contains("token", ignoreCase = true) || it.contains("secret", ignoreCase = true) })
        }
    }

    @Test
    fun theTokenNeverReachesTheDatabaseFiles() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            tokens.put("a", ROOM_TOKEN)
            store.saveSnapshot("a", "0", Snapshot(report(), now))
            store.applyNotified("a", "0", Diff(insert = listOf(key("%1"))), now)
            store.saveSettings(Settings(lastConnectionId = "a"))
            db.openHelper.writableDatabase
                .query("PRAGMA wal_checkpoint(FULL)")
                .close()
            db.close()

            val files =
                context
                    .getDatabasePath(dbName)
                    .parentFile
                    ?.listFiles { f -> f.name.startsWith(dbName) }
                    .orEmpty()
            assertTrue(files.isNotEmpty())
            for (file in files) {
                assertFalse(file.name, String(file.readBytes(), Charsets.ISO_8859_1).contains(ROOM_TOKEN))
            }
            // token は保護領域のファイルにも平文では無い
            for (file in tokenDirectory.listFiles().orEmpty()) {
                assertFalse(file.name, String(file.readBytes(), Charsets.ISO_8859_1).contains(ROOM_TOKEN))
            }
            db = Room.databaseBuilder(context, SwarmDatabase::class.java, dbName).build() // tearDown の close のため
        }
}
