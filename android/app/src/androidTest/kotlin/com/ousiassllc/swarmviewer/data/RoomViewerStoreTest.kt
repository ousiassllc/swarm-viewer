package com.ousiassllc.swarmviewer.data

import android.content.Context
import android.database.sqlite.SQLiteConstraintException
import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import com.ousiassllc.swarmviewer.data.store.ConnectionRepository
import com.ousiassllc.swarmviewer.data.store.MAX_NOTIFIED_PER_CONNECTION
import com.ousiassllc.swarmviewer.data.store.RoomViewerStore
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.domain.SettingsException
import com.ousiassllc.swarmviewer.domain.parseReport
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.security.KeyStore
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

private const val TOKEN = "test-token-not-secret-0123456789"
private const val KEY_ALIAS = "swarm-viewer-room-test-key"
private const val URL_1 = "http://100.64.0.1:48120/report"
private const val URL_2 = "http://100.64.0.2:48120/report"

/** Room（SQLite）の制約・連鎖削除・保存内容を、実際の SQLite（エミュレータ・実機）で確かめる。 */
@RunWith(AndroidJUnit4::class)
class RoomViewerStoreTest {
    private val context: Context = ApplicationProvider.getApplicationContext()
    private val now = Instant.parse("2026-11-05T10:00:00Z")
    private val clock = Clock.fixed(now, ZoneOffset.UTC)
    private val tokenDirectory = File(context.noBackupFilesDir, "room-test-tokens")
    private val dbName = "room-test.db"

    private lateinit var db: SwarmDatabase
    private lateinit var store: RoomViewerStore
    private lateinit var tokens: FileTokenStore

    @Before
    fun setUp() {
        cleanFiles()
        db = Room.databaseBuilder(context, SwarmDatabase::class.java, dbName).build()
        store = RoomViewerStore(db, clock)
        tokens = FileTokenStore(tokenDirectory, KeystoreTokenCipher(KEY_ALIAS))
    }

    @After
    fun tearDown() {
        db.close()
        cleanFiles()
        KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(KEY_ALIAS)
    }

    private fun cleanFiles() {
        context.deleteDatabase(dbName)
        tokenDirectory.deleteRecursively()
    }

    private fun connection(
        id: String,
        url: String,
    ) = Connection(id = id, name = "dev-$id", url = url)

    private fun report(state: String = "asking"): Report =
        parseReport("""{"layout":{"rows":1,"cols":[1]},"panes":[{"pos":"T1","id":"%1","state":"$state"}]}""")

    private fun key(
        pane: String,
        state: PaneState = PaneState.Asking,
    ) = NotifiedKey(pane, state)

    private fun count(table: String): Int =
        db.openHelper.readableDatabase.query("SELECT COUNT(*) FROM $table").use {
            it.moveToFirst()
            it.getInt(0)
        }

    @Test
    fun savedConnectionsComeBackInCreationOrder() =
        runBlocking {
            store.saveConnection(connection("b", URL_2).copy(createdAt = now.plusSeconds(10)))
            store.saveConnection(connection("a", URL_1).copy(createdAt = now))

            assertEquals(listOf("a", "b"), store.connections().map { it.id })
            assertEquals("dev-a", store.connection("a")?.name)
            assertEquals("b", store.connectionByUrl(URL_2)?.id)
            assertNull(store.connection("nope"))
        }

    @Test
    fun savingTheSameUrlKeepsTheIdAndOnlyReplacesTheName() =
        runBlocking {
            store.saveConnection(connection("a", URL_1).copy(createdAt = now))
            store.setLastSession("a", "2")
            store.markBaselined("a", now.plusSeconds(5))

            store.saveConnection(Connection(id = "other-id", name = "renamed", url = URL_1, updatedAt = now.plusSeconds(60)))

            val saved = store.connections().single()
            assertEquals("a", saved.id)
            assertEquals("renamed", saved.name)
            assertEquals("2", saved.lastSession)
            assertEquals(now.plusSeconds(5), saved.baselinedAt)
            assertEquals(now, saved.createdAt)
            assertEquals(now.plusSeconds(60), saved.updatedAt)
        }

    @Test
    fun markBaselinedKeepsTheFirstTimeAndLastSessionCanBeCleared() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            assertTrue(store.markBaselined("a", now))
            assertTrue(store.markBaselined("a", now.plusSeconds(99)))
            assertEquals(now, store.connection("a")?.baselinedAt)

            assertTrue(store.setLastSession("a", "1"))
            assertTrue(store.setLastSession("a", null))
            assertNull(store.connection("a")?.lastSession)
            assertFalse(store.setLastSession("nope", "1"))
            assertFalse(store.markBaselined("nope", now))
        }

    @Test
    fun theSameUrlCannotBeInsertedTwiceDirectly() {
        runBlocking { store.saveConnection(connection("a", URL_1)) }

        assertThrows(SQLiteConstraintException::class.java) {
            db.openHelper.writableDatabase.execSQL(
                "INSERT INTO connections (id, name, url, created_at, updated_at) VALUES ('x', 'n', '$URL_1', 't', 't')",
            )
        }
    }

    @Test
    fun settingsDefaultToTheSpecValuesAndAreValidated() =
        runBlocking {
            assertEquals(Settings(), store.settings())
            assertEquals(15, store.settings().notifyPollMinutes)

            store.saveConnection(connection("a", URL_1))
            val changed =
                Settings(
                    notificationsEnabled = false,
                    screenPollSeconds = 30,
                    notifyPollMinutes = 60,
                    debugLog = true,
                    lastConnectionId = "a",
                )
            store.saveSettings(changed)
            assertEquals(changed, store.settings())
            assertEquals(1, count("settings")) // 1 行固定

            assertThrows(SettingsException::class.java) { runBlocking { store.saveSettings(changed.copy(notifyPollMinutes = 5)) } }
            assertEquals(changed, store.settings())
        }

    @Test
    fun deletingTheLastConnectionResetsTheSettingsReference() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            store.setLastConnection("a")
            assertEquals("a", store.settings().lastConnectionId)

            store.deleteConnection("a")

            assertNull(store.settings().lastConnectionId)
        }

    @Test
    fun snapshotsKeepOnlyTheLatestPerConnectionAndSession() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            store.saveSnapshot("a", "0", Snapshot(report("asking"), now))
            store.saveSnapshot("a", "0", Snapshot(report("idle"), now.plusSeconds(30)))
            store.saveSnapshot("a", "1", Snapshot(report("working"), now))

            val latest = store.snapshot("a", "0")
            assertEquals(
                PaneState.Idle,
                latest
                    ?.report
                    ?.panes
                    ?.single()
                    ?.state,
            )
            assertEquals(now.plusSeconds(30), latest?.fetchedAt)
            assertEquals(2, count("snapshots"))
            assertNull(store.snapshot("a", "9"))

            store.pruneSnapshots("a", listOf("1"))
            assertNull(store.snapshot("a", "0"))
            assertNotNull(store.snapshot("a", "1"))
            store.pruneSnapshots("a", emptyList())
            assertEquals(0, count("snapshots"))
        }

    @Test
    fun aBrokenOrUnknownVersionSnapshotIsTreatedAsMissing() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            val sql = db.openHelper.writableDatabase
            sql.execSQL("INSERT INTO snapshots VALUES ('a', 'broken', '2026-11-05T10:00:00Z', 1, 'not json')")
            sql.execSQL("INSERT INTO snapshots VALUES ('a', 'future', '2026-11-05T10:00:00Z', 99, '{}')")
            sql.execSQL("INSERT INTO snapshots VALUES ('a', 'badtime', 'yesterday', 1, '{}')")

            assertNull(store.snapshot("a", "broken"))
            assertNull(store.snapshot("a", "future"))
            assertNull(store.snapshot("a", "badtime"))
        }

    @Test
    fun aDuplicateNotifiedStateFailsOnThePrimaryKey() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            val insert = "INSERT INTO notified_states VALUES ('a', '0', '%1', 'asking', '2026-11-05T10:00:00Z')"
            db.openHelper.writableDatabase.execSQL(insert)

            assertThrows(SQLiteConstraintException::class.java) { db.openHelper.writableDatabase.execSQL(insert) }
            assertEquals(1, count("notified_states"))
        }

    @Test
    fun applyNotifiedReportsOnlyTheRowsThatWereReallyInserted() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            val diff = Diff(insert = listOf(key("%1"), key("%2", PaneState.Stalled)))

            assertEquals(diff.insert, store.applyNotified("a", "0", diff, now))
            assertEquals(emptyList<NotifiedKey>(), store.applyNotified("a", "0", diff, now)) // 二重通知にならない
            assertEquals(diff.insert.toSet(), store.notifiedKeys("a", "0").toSet())
            assertEquals(emptyList<NotifiedKey>(), store.notifiedKeys("a", "1"))

            val change = Diff(insert = listOf(key("%1", PaneState.Stalled)), delete = listOf(key("%1")))
            assertEquals(change.insert, store.applyNotified("a", "0", change, now))
            assertEquals(setOf(key("%1", PaneState.Stalled), key("%2", PaneState.Stalled)), store.notifiedKeys("a", "0").toSet())
        }

    @Test
    fun applyNotifiedRefusesStatesThatAreNotNotifiable() {
        runBlocking { store.saveConnection(connection("a", URL_1)) }

        assertThrows(IllegalArgumentException::class.java) {
            runBlocking { store.applyNotified("a", "0", Diff(insert = listOf(key("%1", PaneState.Working))), now) }
        }
        assertEquals(0, count("notified_states"))
    }

    @Test
    fun applyNotifiedForAnUnknownConnectionFailsOnTheForeignKey() {
        assertThrows(SQLiteConstraintException::class.java) {
            runBlocking { store.applyNotified("ghost", "0", Diff(insert = listOf(key("%1"))), now) }
        }
    }

    @Test
    fun notifiedStatesAreTrimmedToTheLimitOldestFirst() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            val sql = db.openHelper.writableDatabase
            sql.beginTransaction()
            try {
                // 1,000 件を古い時刻で埋める（%0 が最も古い）。
                for (i in 0 until MAX_NOTIFIED_PER_CONNECTION) {
                    val at = now.minusSeconds((MAX_NOTIFIED_PER_CONNECTION - i).toLong())
                    sql.execSQL("INSERT INTO notified_states VALUES ('a', '0', '%$i', 'asking', '$at')")
                }
                sql.setTransactionSuccessful()
            } finally {
                sql.endTransaction()
            }
            assertEquals(MAX_NOTIFIED_PER_CONNECTION, count("notified_states"))
            val fresh = (0 until 5).map { key("%new$it") }

            assertEquals(fresh, store.applyNotified("a", "0", Diff(insert = fresh), now))

            assertEquals(MAX_NOTIFIED_PER_CONNECTION, count("notified_states"))
            val kept = store.notifiedKeys("a", "0").map { it.paneId }.toSet()
            assertFalse("%0" in kept) // 最も古い 5 件が消える
            assertFalse("%4" in kept)
            assertTrue("%5" in kept)
            assertTrue(fresh.all { it.paneId in kept })
        }

    @Test
    fun deletingAConnectionRemovesItsTokenSnapshotsAndNotifiedStates() =
        runBlocking {
            val repository = ConnectionRepository(store, tokens)
            for ((id, url) in listOf("a" to URL_1, "b" to URL_2)) {
                store.saveConnection(connection(id, url))
                tokens.put(id, "$TOKEN-$id")
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
            assertEquals("$TOKEN-b", tokens.get("b"))
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
            tokens.put("a", TOKEN)
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
                assertFalse(file.name, String(file.readBytes(), Charsets.ISO_8859_1).contains(TOKEN))
            }
            // token は保護領域のファイルにも平文では無い
            for (file in tokenDirectory.listFiles().orEmpty()) {
                assertFalse(file.name, String(file.readBytes(), Charsets.ISO_8859_1).contains(TOKEN))
            }
            db = Room.databaseBuilder(context, SwarmDatabase::class.java, dbName).build() // tearDown の close のため
        }
}
