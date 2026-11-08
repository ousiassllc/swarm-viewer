package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runCurrent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
internal class PaneDetailConfirmTest : PaneFixture() {
    @Test
    fun escapeAndCloseAskForConfirmationFirstAndEscapeCarriesConfirmTrueOnlyAfterIt() =
        runPaneTest {
            state = PaneState.Working
            val viewModel = opened(OPS_ID)
            api.calls.clear()

            viewModel.requestEscape()
            runCurrent()
            assertEquals(PaneConfirm.Escape, viewModel.state.value.pendingConfirm)
            assertEquals(emptyList<String>(), operationCalls()) // ダイアログを出しただけでは送らない

            viewModel.dismissConfirm()
            viewModel.confirmPending() // 取り消したあとに確定は届かない
            runCurrent()
            assertEquals(emptyList<String>(), operationCalls())

            viewModel.requestEscape()
            viewModel.confirmPending()
            runCurrent()
            assertEquals(listOf("send_escape s0 T1 confirm=true"), operationCalls())
            assertEquals(
                "中断しました",
                viewModel.state.value.notice
                    ?.message,
            )
            viewModel.onStop()
        }

    @Test
    fun closingNeedsConfirmationIsOfferedOnlyForIdleAndShellAndLeavesTheScreenOnSuccess() =
        runPaneTest {
            state = PaneState.Working
            val working = opened(OPS_ID)
            working.requestClose()
            assertNull(working.state.value.pendingConfirm) // working には「終了」が無い
            working.onStop()

            state = PaneState.Idle
            val viewModel = newDetail().also { it.open(target(OPS_ID, "T2")) }
            val left = mutableListOf<Unit>()
            val collector = CoroutineScope(Dispatchers.Unconfined).launchCollect(viewModel.leave, left)
            viewModel.onStart()
            runCurrent()
            api.calls.clear()

            viewModel.requestClose()
            assertEquals(PaneConfirm.Close, viewModel.state.value.pendingConfirm)
            assertEquals(emptyList<String>(), operationCalls())
            viewModel.confirmPending()
            runCurrent()

            assertEquals(listOf("pane_close s0 T2"), operationCalls())
            assertEquals(1, left.size) // 終了後はメイン画面へ戻る
            collector.cancel()
            viewModel.onStop()
        }

    @Test
    fun anExitTimeoutOffersARetryAndAFailedCloseDoesNotLeave() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID)
            api.failure = FetchException(ErrKind.Operation, "exit_timeout")

            viewModel.requestClose()
            viewModel.confirmPending()
            runCurrent()

            assertEquals(
                operationErrorMessage("exit_timeout"),
                viewModel.state.value.notice
                    ?.message,
            )
            assertTrue(viewModel.state.value.retryClose)
            viewModel.onStop()
        }
}

private fun CoroutineScope.launchCollect(
    flow: Flow<Unit>,
    into: MutableList<Unit>,
) = launch { flow.collect { into += it } }
