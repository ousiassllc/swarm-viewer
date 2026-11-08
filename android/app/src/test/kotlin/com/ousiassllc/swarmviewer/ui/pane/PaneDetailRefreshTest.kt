package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
internal class PaneDetailRefreshTest : PaneFixture() {
    @Test
    fun theTailIsRefreshedEveryFiveSecondsWhileVisibleAndStopsInTheBackground() =
        runPaneTest {
            state = PaneState.Working
            val viewModel = opened(OPS_ID)
            assertEquals(1, reportCalls)

            passTime(PANE_POLL_MILLIS - 1)
            assertEquals(1, reportCalls)
            passTime(1)
            assertEquals(2, reportCalls)

            viewModel.onStop()
            passTime(10 * PANE_POLL_MILLIS)
            assertEquals(2, reportCalls) // 画面が見えていない間は止まる

            viewModel.onStart()
            runCurrent()
            assertEquals(3, reportCalls) // 戻ったら即時に取得
            viewModel.onStop()
        }

    @Test
    fun showMoreStepsThirtyToHundredToTwoHundredLines() =
        runPaneTest {
            state = PaneState.Working
            val viewModel = opened(OPS_ID)
            assertTrue(viewModel.state.value.canShowMore)

            viewModel.showMore()
            runCurrent()
            viewModel.showMore()
            runCurrent()

            assertEquals(
                listOf("pane_tail s0 T1 30", "pane_tail s0 T1 100", "pane_tail s0 T1 200"),
                api.calls.filter { it.startsWith("pane_tail") },
            )
            assertFalse(viewModel.state.value.canShowMore)
            viewModel.onStop()
        }

    @Test
    fun aFailureKeepsThePreviousTailAndShowsAFixedBanner() =
        runPaneTest {
            state = PaneState.Working
            val viewModel = opened(OPS_ID)
            reportFailure = FetchException(ErrKind.Unreachable)

            viewModel.refresh()
            runCurrent()

            assertEquals(
                "tail-body",
                viewModel.state.value.tail
                    ?.text,
            )
            assertNotNull(viewModel.state.value.failure)
            assertFalse(viewModel.state.value.needsRepair)
            viewModel.onStop()
        }

    @Test
    fun anUnauthorizedResponseStopsTheAutomaticRefreshAndAsksForRepairing() =
        runPaneTest {
            state = PaneState.Working
            val viewModel = opened(OPS_ID)
            reportFailure = FetchException(ErrKind.Unauthorized, "unauthorized")

            viewModel.refresh()
            runCurrent()
            assertTrue(viewModel.state.value.needsRepair)
            val calls = reportCalls
            passTime(10 * PANE_POLL_MILLIS)

            assertEquals(calls, reportCalls)
            viewModel.onStop()
        }

    @Test
    fun aPaneThatIsNoLongerInTheReportShowsAFixedMessage() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID, pos = "Z9")

            assertNull(viewModel.state.value.pane)
            assertEquals(operationErrorMessage("pane_not_found"), viewModel.state.value.failure)
            assertFalse(viewModel.state.value.controls.anyOperation)
            viewModel.onStop()
        }
}
