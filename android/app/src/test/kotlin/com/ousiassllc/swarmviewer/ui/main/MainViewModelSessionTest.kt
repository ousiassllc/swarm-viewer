package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.Report
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

@OptIn(ExperimentalCoroutinesApi::class)
internal class MainViewModelSessionTest : MainViewModelFixture() {
    @Test
    fun startingFetchesTheFirstAllowedSessionAndShowsOnlyAllowedTabs() =
        runTest {
            val viewModel = started()

            val state = viewModel.state.value
            assertEquals(listOf("s0", "s1"), state.sessions)
            assertEquals("s0", state.session)
            assertEquals(listOf("a" to "s0"), reports.calls) // session は常に明示する
            assertEquals(
                "first-s0",
                state.snapshot
                    ?.report
                    ?.panes
                    ?.single()
                    ?.title,
            )
            assertTrue(state.live)
            assertFalse(state.showsPreviousFetch)
            viewModel.onStop()
        }

    @Test
    fun theLastChosenSessionIsRememberedPerConnectionAndRestored() =
        runTest {
            addConnection("b", "100.64.0.2")
            val first = started()
            first.selectSession("s1")
            runCurrent()
            assertEquals("s1", store.connectionRows["a"]?.lastSession)
            assertNull(store.connectionRows["b"]?.lastSession) // 接続ごとに記憶する
            first.onStop()

            val restored = started() // 再起動を模す
            assertEquals("s1", restored.state.value.session)
            assertTrue(reports.calls.last() == ("a" to "s1"))
            restored.onStop()
        }

    @Test
    fun withoutAMemoryOrWhenItIsNoLongerAllowedTheFirstAllowedSessionIsUsed() =
        runTest {
            store.connectionRows["a"] = store.connectionRows.getValue("a").copy(lastSession = "gone")
            val viewModel = started()
            assertEquals("s0", viewModel.state.value.session)
            viewModel.onStop()

            assertEquals("s0", pickSession(listOf("s0", "s1"), null))
            assertEquals("s1", pickSession(listOf("s0", "s1"), "s1"))
            assertEquals("s0", pickSession(listOf("s0", "s1"), "other"))
            assertNull(pickSession(emptyList(), "s1"))
        }

    @Test
    fun switchingTheSessionShowsItsSavedSnapshotAndFetchesIt() =
        runTest {
            val saved = Snapshot(Report(panes = listOf(Pane(pos = "T1", id = "%1", title = "s1-cached"))), Instant.EPOCH)
            store.snapshotRows["a" to "s1"] = saved
            val viewModel = started()
            reports.gate = CompletableDeferred()

            viewModel.selectSession("s1")
            runCurrent()

            assertEquals("s1", viewModel.state.value.session)
            assertEquals(saved, viewModel.state.value.snapshot)
            assertEquals("a" to "s1", reports.calls.last())
            reports.gate?.complete(Unit)
            runCurrent()
            assertEquals(
                "first-s1",
                viewModel.state.value.snapshot
                    ?.report
                    ?.panes
                    ?.single()
                    ?.title,
            )
            viewModel.onStop()
        }

    @Test
    fun selectingAnotherConnectionSwitchesWhatIsFetched() =
        runTest {
            addConnection("b", "100.64.0.2")
            val viewModel = started()
            assertEquals(
                listOf("dev-a", "dev-b"),
                viewModel.state.value.connections
                    ?.map { it.name },
            )

            viewModel.selectConnection("b")
            runCurrent()

            assertEquals("b", viewModel.state.value.connectionId)
            assertEquals("b" to "s0", reports.calls.last())
            viewModel.onStop()
        }

    @Test
    fun withNoConnectionsNothingIsFetched() =
        runTest {
            store.connectionRows.clear()
            val viewModel = started()

            assertTrue(viewModel.state.value.noConnections)
            assertEquals(0, reports.calls.size)
            passTime(5 * DEFAULT_INTERVAL_MILLIS)
            assertEquals(0, reports.calls.size)
            viewModel.onStop()
        }

    @Test
    fun theStateNeverHoldsTheTokenOrTheFullUrl() =
        runTest {
            val viewModel = started()
            val shown = viewModel.state.value.toString()
            assertFalse(shown.contains(TOKEN))
            assertFalse(shown.contains("48120"))
            viewModel.onStop()
        }
}
