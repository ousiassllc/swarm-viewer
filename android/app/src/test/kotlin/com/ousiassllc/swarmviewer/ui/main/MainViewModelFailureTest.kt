package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

@OptIn(ExperimentalCoroutinesApi::class)
internal class MainViewModelFailureTest : MainViewModelFixture() {
    @Test
    fun theSavedSnapshotIsShownAsPreviousFetchBeforeTheFirstFetchFinishes() =
        runTest {
            val saved =
                Snapshot(Report(panes = listOf(Pane(pos = "T1", id = "%1", title = "cached"))), Instant.parse("2026-11-05T01:02:03Z"))
            store.connectionRows["a"] = store.connectionRows.getValue("a").copy(lastSession = "s0")
            store.snapshotRows["a" to "s0"] = saved
            reports.gate = CompletableDeferred()

            val viewModel = started()

            val waiting = viewModel.state.value
            assertEquals(
                "cached",
                waiting.snapshot
                    ?.report
                    ?.panes
                    ?.single()
                    ?.title,
            )
            assertFalse(waiting.live)
            assertTrue(waiting.showsPreviousFetch) // 起動直後は「前回取得」
            assertFalse(waiting.stale) // 失敗していないのでグレーアウトしない

            reports.gate?.complete(Unit)
            runCurrent()
            assertTrue(viewModel.state.value.live)
            assertFalse(viewModel.state.value.showsPreviousFetch)
            assertNotNull(store.snapshotRows["a" to "s0"]) // 取得の結果で上書きされる（最新 1 件）
            assertEquals(
                "first-s0",
                store.snapshotRows["a" to "s0"]
                    ?.report
                    ?.panes
                    ?.single()
                    ?.title,
            )
            viewModel.onStop()
        }

    @Test
    fun aFailureKeepsThePreviousContentGrayedOutWithTheReason() =
        runTest {
            val viewModel = started()
            val before = viewModel.state.value.snapshot

            reports.failure = FetchException(ErrKind.Unreachable)
            passTime(DEFAULT_INTERVAL_MILLIS)

            val state = viewModel.state.value
            assertEquals(before, state.snapshot) // 直前の表示を残す
            assertTrue(state.stale) // グレーアウト
            assertTrue(state.showsPreviousFetch) // 「前回取得」
            assertEquals(fetchErrorMessage(ErrKind.Unreachable), state.failure)
            assertTrue(state.failure.orEmpty().contains("Tailscale")) // Tailscale が切れている可能性を示す
            assertFalse(state.needsRepair)

            // 失敗しても次の周期で再試行し、成功すれば元に戻る（間隔は延ばさない）
            reports.failure = null
            passTime(DEFAULT_INTERVAL_MILLIS)
            assertFalse(viewModel.state.value.stale)
            assertNull(viewModel.state.value.failure)
            viewModel.onStop()
        }

    @Test
    fun unauthorizedStopsPollingForThatConnectionAndPointsToRepairing() =
        runTest {
            val viewModel = started()
            reports.failure = FetchException(ErrKind.Unauthorized, "unauthorized")
            passTime(DEFAULT_INTERVAL_MILLIS)
            val calls = reports.calls.size

            val state = viewModel.state.value
            assertTrue(state.needsRepair)
            assertTrue(state.stale)
            assertTrue(connections.needsRepair.value.contains("a"))

            passTime(10 * DEFAULT_INTERVAL_MILLIS)
            assertEquals(calls, reports.calls.size) // ポーリングが止まる
            viewModel.refresh()
            runCurrent()
            assertEquals(calls, reports.calls.size) // 手動更新でも再試行しない

            viewModel.onStop()
            viewModel.onStart() // 再ペアリングまで、戻っても取得しない
            runCurrent()
            assertEquals(calls, reports.calls.size)
            assertTrue(viewModel.state.value.needsRepair)
            viewModel.onStop()
        }

    @Test
    fun forbiddenAlsoStopsPollingAndRepairingResumesIt() =
        runTest {
            val viewModel = started()
            reports.failure = FetchException(ErrKind.ForbiddenScope, "forbidden_scope")
            passTime(DEFAULT_INTERVAL_MILLIS)
            assertTrue(viewModel.state.value.needsRepair)
            val calls = reports.calls.size
            viewModel.onStop()

            connections.markRepaired("a") // 再ペアリングの成功
            reports.failure = null
            viewModel.onStart()
            runCurrent()

            assertFalse(viewModel.state.value.needsRepair)
            assertEquals(calls + 1, reports.calls.size)
            assertTrue(viewModel.state.value.live)
            viewModel.onStop()
        }

    @Test
    fun aMissingTokenMeansRepairingWithoutAnyRequest() =
        runTest {
            tokens.tokens.clear()
            val viewModel = started()

            assertTrue(viewModel.state.value.needsRepair)
            assertEquals(0, reports.calls.size)
            viewModel.onStop()
        }

    @Test
    fun aSessionThatDisappearedTriggersARefetchOfTheSessionList() =
        runTest {
            store.connectionRows["a"] = store.connectionRows.getValue("a").copy(lastSession = "s1")
            val viewModel = started()
            assertEquals("s1", viewModel.state.value.session)

            reports.failure = FetchException(ErrKind.SessionMissing, "invalid_args")
            allowed = listOf(Session("s0", true))
            passTime(DEFAULT_INTERVAL_MILLIS)
            assertEquals(fetchErrorMessage(ErrKind.SessionMissing), viewModel.state.value.failure)

            reports.failure = null
            passTime(DEFAULT_INTERVAL_MILLIS)
            val state = viewModel.state.value
            assertEquals(listOf("s0"), state.sessions)
            assertEquals("s0", state.session)
            assertNull(state.failure)
            assertTrue(sessionsCalls >= 2)
            viewModel.onStop()
        }

    @Test
    fun whenTheSessionListCannotBeFetchedTheSavedSnapshotStaysVisible() =
        runTest {
            val saved = Snapshot(Report(panes = listOf(Pane(pos = "T1", id = "%1", title = "cached"))), Instant.EPOCH)
            store.connectionRows["a"] = store.connectionRows.getValue("a").copy(lastSession = "s0")
            store.snapshotRows["a" to "s0"] = saved
            sessionsFailure = FetchException(ErrKind.Unreachable)

            val viewModel = started()

            val state = viewModel.state.value
            assertEquals(saved, state.snapshot)
            assertTrue(state.stale)
            assertTrue(state.showsPreviousFetch)
            assertEquals(0, reports.calls.size)
            viewModel.onStop()
        }
}
