package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.ui.PANE_BUSY_MESSAGE
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
internal class PaneDetailViewModelTest : PaneFixture() {
    @Test
    fun openingReadsReportThenPaneTailAndTheCardOnlyWhileAsking() =
        runPaneTest {
            state = PaneState.Working
            val working = opened(OPS_ID)
            assertEquals(listOf("pane_tail s0 T1 30"), api.calls) // asking ではないので ask_card は呼ばない
            assertEquals(
                "tail-body",
                working.state.value.tail
                    ?.text,
            )
            assertEquals(ConnectionKind.Ops, working.state.value.kind)
            working.onStop()

            api.calls.clear()
            state = PaneState.Asking
            val asking = opened(OPS_ID)
            assertEquals(listOf("pane_tail s0 T1 30", "ask_card s0 T1"), api.calls)
            assertEquals(
                2,
                asking.state.value.card
                    ?.options
                    ?.size,
            )
            asking.onStop()
        }

    @Test
    fun aViewOnlyConnectionReadsTheTailAndCardButRendersNoOperationAndSendsNothing() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(VIEW_ID)

            val ui = viewModel.state.value
            assertEquals(ConnectionKind.View, ui.kind)
            assertEquals(listOf("pane_tail s0 T1 30", "ask_card s0 T1"), api.calls) // 閲覧は通る
            assertTrue(ui.controls.showCardList && ui.controls.showViewOnlyHint)
            assertFalse(ui.controls.anyOperation) // 操作の部品は 1 つも出ない

            // 操作を直接呼んでも何も送らない。
            viewModel.pick(1)
            runCurrent()
            viewModel.onSendTextChange("x")
            viewModel.send()
            runCurrent()
            viewModel.requestEscape()
            viewModel.confirmPending()
            viewModel.requestClose()
            viewModel.confirmPending()
            runCurrent()
            assertNull(viewModel.state.value.pendingConfirm)
            assertNull(viewModel.state.value.notice)
            assertEquals(emptyList<String>(), operationCalls())
            viewModel.onStop()
        }

    @Test
    fun pickingAnOptionSendsAskPickImmediatelyWithoutRefetchingTheCardFirst() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            api.calls.clear()

            viewModel.pick(2)
            runCurrent()

            assertEquals("ask_pick s0 T1 n=2", api.calls.first()) // 送る前に取り直さない
            assertEquals(
                "送りました",
                viewModel.state.value.notice
                    ?.message,
            )
            assertFalse(
                viewModel.state.value.notice
                    ?.error ?: true,
            )
            assertTrue(api.calls.drop(1).any { it.startsWith("pane_tail") }) // 送ったあとに表示を取り直す
            viewModel.onStop()
        }

    @Test
    fun everyButtonIsDisabledWhileAnOperationIsInFlightAndASecondTapSendsNothing() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            api.calls.clear()
            api.gate = CompletableDeferred()

            viewModel.pick(1)
            runCurrent()
            assertTrue(viewModel.state.value.busy)
            viewModel.pick(2)
            viewModel.onSendTextChange("x")
            viewModel.send()
            viewModel.pickCustom()
            runCurrent()
            assertEquals(listOf("ask_pick s0 T1 n=1"), api.calls) // 二重に送らない

            api.gate?.complete(Unit)
            runCurrent()
            assertFalse(viewModel.state.value.busy)
            viewModel.onStop()
        }

    @Test
    fun theServiceRefusesAnOverlappingOperationEvenIfTheViewModelWereBypassed() =
        runPaneTest {
            state = PaneState.Idle
            val first = opened(OPS_ID)
            val second = opened(OPS_ID)
            api.gate = CompletableDeferred()
            api.calls.clear()

            first.onSendTextChange("one")
            first.send()
            second.onSendTextChange("two")
            second.send()
            runCurrent()

            assertEquals(listOf("send_text s0 T1 one"), operationCalls()) // 2 つ目は通信なし
            assertEquals(
                PANE_BUSY_MESSAGE,
                second.state.value.notice
                    ?.message,
            )
            api.gate?.complete(Unit)
            runCurrent()
            first.onStop()
            second.onStop()
        }

    @Test
    fun sendingClearsTheFieldOnlyWhenAccepted() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID)

            viewModel.onSendTextChange("続けて")
            viewModel.send()
            runCurrent()
            assertEquals("", viewModel.state.value.sendText)
            assertEquals(
                "送りました",
                viewModel.state.value.notice
                    ?.message,
            )

            api.send = api.send.copy(accepted = false)
            viewModel.onSendTextChange("again")
            viewModel.send()
            runCurrent()
            assertEquals("again", viewModel.state.value.sendText) // 受理されなければ残す
            assertEquals(
                "受け付けられませんでした",
                viewModel.state.value.notice
                    ?.message,
            )
            assertTrue(
                viewModel.state.value.notice
                    ?.error ?: false,
            )
            viewModel.onStop()
        }

    @Test
    fun anInvalidTextIsRejectedBeforeSendingWithAFixedMessage() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID)
            api.calls.clear()

            viewModel.onSendTextChange("a\u001bb")
            viewModel.send()
            runCurrent()

            assertEquals(
                operationErrorMessage("forbidden_control_char"),
                viewModel.state.value.notice
                    ?.message,
            )
            assertEquals(emptyList<String>(), operationCalls())
            viewModel.onStop()
        }

    @Test
    fun customAnswerIsSentAsTextAndClearedOnSuccess() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            viewModel.openCustom(true)
            viewModel.onCustomTextChange("自分の答え")
            api.calls.clear()

            viewModel.pickCustom()
            runCurrent()

            assertEquals("ask_pick s0 T1 text=自分の答え", api.calls.first())
            assertEquals("", viewModel.state.value.customText)
            assertFalse(viewModel.state.value.customOpen)
            viewModel.onStop()
        }

    @Test
    fun theCardClosedFalseResultIsShownAsAnUnconfirmedError() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            api.pick = api.pick.copy(ok = false, cardClosed = false)

            viewModel.pick(1)
            runCurrent()

            assertEquals(
                "選択を確定できませんでした。もう一度お試しください",
                viewModel.state.value.notice
                    ?.message,
            )
            assertTrue(
                viewModel.state.value.notice
                    ?.error ?: false,
            )
            viewModel.onStop()
        }

    @Test
    fun refetchesTheCardWhenTheOptionIsStaleAndShowsTheFixedMessage() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            api.failure = FetchException(ErrKind.Operation, "option_out_of_range")
            api.card = AskCard(true, "新しい質問", emptyList())
            api.calls.clear()

            viewModel.pick(9)
            runCurrent()
            api.failure = null
            viewModel.refresh()
            runCurrent()

            assertEquals(
                operationErrorMessage("option_out_of_range"),
                viewModel.state.value.notice
                    ?.message,
            )
            assertEquals(
                "新しい質問",
                viewModel.state.value.card
                    ?.question,
            )
            viewModel.onStop()
        }
}
