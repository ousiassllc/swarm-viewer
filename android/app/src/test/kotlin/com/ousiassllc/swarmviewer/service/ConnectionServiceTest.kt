package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Diff
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException
import java.time.Instant

private val NOW: Instant = Instant.parse("2026-11-05T10:00:00Z")

class ConnectionServiceTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val service = ConnectionService(store, tokens)

    private suspend fun add(
        id: String,
        host: String,
        token: String? = "token-$id",
    ) {
        store.saveConnection(Connection(id, "dev-$id", "http://$host:48120/report", createdAt = NOW))
        if (token != null) tokens.put(id, token)
    }

    @Test
    fun listShowsNameAndHostOnlyAndSelectsTheFirstByDefault() =
        runTest {
            add("a", "100.64.0.1")
            add("b", "100.64.0.2")

            val items = service.list()

            assertEquals(listOf("a", "b"), items.map { it.id })
            assertEquals(listOf("dev-a", "dev-b"), items.map { it.name })
            assertEquals(listOf("100.64.0.1", "100.64.0.2"), items.map { it.host })
            assertEquals(listOf(true, false), items.map { it.selected })
            assertFalse(items.toString().contains("token-a")) // token は一覧の型に入らない
            assertFalse(items.toString().contains("48120"))
        }

    @Test
    fun theLastSelectedConnectionIsRestored() =
        runTest {
            add("a", "100.64.0.1")
            add("b", "100.64.0.2")

            assertTrue(service.select("b"))

            assertEquals(listOf(false, true), service.list().map { it.selected })
        }

    @Test
    fun aMissingLastConnectionFallsBackToTheFirst() =
        runTest {
            add("a", "100.64.0.1")
            store.settingsRow = store.settings().copy(lastConnectionId = "gone")

            assertEquals(listOf(true), service.list().map { it.selected })
        }

    @Test
    fun selectingAnUnknownConnectionChangesNothing() =
        runTest {
            add("a", "100.64.0.1")

            assertFalse(service.select("nope"))

            assertNull(store.settings().lastConnectionId)
        }

    @Test
    fun aMissingOrUnreadableTokenNeedsRePairing() =
        runTest {
            add("ok", "100.64.0.1")
            add("none", "100.64.0.2", token = null)
            add("lost", "100.64.0.3")
            tokens.unreadable += "lost" // Keystore の鍵が消えて復号できない

            val repair = service.list().associate { it.id to it.needsRepair }

            assertEquals(mapOf("ok" to false, "none" to true, "lost" to true), repair)
        }

    @Test
    fun a401Or403MarkNeedsRePairingUntilRePaired() =
        runTest {
            add("a", "100.64.0.1")

            service.markNeedsRepair("a")
            assertTrue(service.list().single().needsRepair)
            assertEquals(setOf("a"), service.needsRepair.value)

            service.markRepaired("a")
            assertFalse(service.list().single().needsRepair)
            assertTrue(service.needsRepair.value.isEmpty())
        }

    @Test
    fun deleteRemovesTokenSnapshotsAndNotifiedStatesAndTheMark() =
        runTest {
            add("a", "100.64.0.1")
            add("b", "100.64.0.2")
            for (id in listOf("a", "b")) {
                store.saveSnapshot(id, "0", Snapshot(Report(), NOW))
                store.applyNotified(id, "0", Diff(insert = listOf(NotifiedKey("%1", PaneState.Asking))), NOW)
            }
            service.select("a")
            service.markNeedsRepair("a")

            service.delete("a")

            assertNull(store.connection("a"))
            assertNull(tokens.tokens["a"])
            assertNull(store.snapshot("a", "0"))
            assertTrue(store.notifiedKeys("a", "0").isEmpty())
            assertNull(store.settings().lastConnectionId)
            assertTrue(service.needsRepair.value.isEmpty())
            // ほかの接続は残る
            assertNotNull(store.connection("b"))
            assertEquals("token-b", tokens.tokens["b"])
            assertNotNull(store.snapshot("b", "0"))
            assertEquals(1, store.notifiedKeys("b", "0").size)
            assertEquals(listOf(true), service.list().map { it.selected }) // 先頭に戻る
        }

    @Test
    fun aTokenDeletionFailureKeepsTheConnection() =
        runTest {
            add("a", "100.64.0.1")
            tokens.failDelete = true

            try {
                service.delete("a")
                throw AssertionError("削除の失敗が伝わらなかった")
            } catch (e: IOException) {
                assertNotNull(store.connection("a"))
                assertEquals("token-a", tokens.tokens["a"])
            }
        }
}
