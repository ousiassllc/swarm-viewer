package com.ousiassllc.swarmviewer.data

import android.database.sqlite.SQLiteConstraintException
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.ousiassllc.swarmviewer.data.store.MAX_NOTIFIED_PER_CONNECTION
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/** スナップショット・通知済み状態の保存（実際の SQLite で確かめる）。 */
@RunWith(AndroidJUnit4::class)
internal class RoomSnapshotStoreTest : RoomStoreTestBase() {
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
}
