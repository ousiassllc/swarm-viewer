package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Test
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

class SnapshotServiceTest {
    private val store = FakeViewerStore()
    private val now = Instant.parse("2026-11-05T01:02:03Z")
    private var failure: FetchException? = null
    private var sessions = listOf(Session("s0", true), Session("s1", false), Session("s2", true))
    private val service =
        SnapshotService(
            lister = { sessions },
            reports = { _, session ->
                failure?.let { throw it }
                Report(panes = listOf(Pane(pos = "T1", id = "%1", title = session)))
            },
            store = store,
            clock = Clock.fixed(now, ZoneOffset.UTC),
        )

    private fun connection(id: String) = Connection(id, "dev-$id", "http://100.64.0.1:48120/report", createdAt = Instant.EPOCH)

    @Test
    fun refreshSavesTheLatestSnapshotPerConnectionAndSession() =
        runBlocking {
            store.connectionRows["a"] = connection("a")

            val first = service.refresh("a", "s0")
            service.refresh("a", "s2")
            val second = service.refresh("a", "s0")

            assertEquals(now, first.fetchedAt)
            assertEquals(setOf("a" to "s0", "a" to "s2"), store.snapshotRows.keys) // 履歴を持たず接続×セッションごとに 1 件
            assertEquals(second, service.cached("a", "s0"))
            assertEquals(
                "s0",
                service
                    .cached("a", "s0")
                    ?.report
                    ?.panes
                    ?.single()
                    ?.title,
            )
        }

    @Test
    fun aFailedRefreshKeepsTheSavedSnapshot() =
        runBlocking {
            store.connectionRows["a"] = connection("a")
            val saved = Snapshot(Report(panes = listOf(Pane(title = "old"))), Instant.EPOCH)
            store.snapshotRows["a" to "s0"] = saved
            failure = FetchException(ErrKind.Unreachable)

            assertThrows(FetchException::class.java) { runBlocking { service.refresh("a", "s0") } }

            assertEquals(saved, service.cached("a", "s0"))
        }

    @Test
    fun refreshOfADeletedConnectionFailsWithoutSavingAnything() {
        assertThrows(ConnectionGoneException::class.java) { runBlocking { service.refresh("gone", "s0") } }
        assertEquals(0, store.snapshotRows.size)
    }

    @Test
    fun allowedSessionsDropsForbiddenOnesAndPrunesVanishedSnapshots() =
        runBlocking {
            store.connectionRows["a"] = connection("a")
            val old = Snapshot(Report(), Instant.EPOCH)
            for (id in listOf("s0", "s1", "gone")) store.snapshotRows["a" to id] = old

            val allowed = service.allowedSessions("a")

            assertEquals(listOf("s0", "s2"), allowed) // allowed: true だけ、sessions_list の順
            assertEquals(setOf("a" to "s0"), store.snapshotRows.keys) // 一覧から消えたセッションの分を削除
        }

    @Test
    fun theLastSessionIsRememberedPerConnection() =
        runBlocking {
            store.connectionRows["a"] = connection("a")
            store.connectionRows["b"] = connection("b").copy(url = "http://100.64.0.2:48120/report")
            assertNull(service.lastSession("a"))

            service.remember("a", "s2")

            assertEquals("s2", service.lastSession("a"))
            assertNull(service.lastSession("b"))
            assertNull(service.lastSession("missing"))
        }
}
