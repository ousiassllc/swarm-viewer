package com.ousiassllc.swarmviewer.ui.pane

import app.cash.turbine.test
import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** ペイン詳細の画面を離れる・戻る・答えたあとの表示の扱い。 */
@OptIn(ExperimentalCoroutinesApi::class)
internal class PaneDetailLifecycleTest : PaneFixture() {
    private suspend fun closeIdlePane(viewModel: PaneDetailViewModel) {
        viewModel.requestClose()
        viewModel.confirmPending()
    }

    @Test
    fun aLeaveEventSentWhileNobodyIsWatchingIsNotReplayedWhenThePaneIsOpenedAgain() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID, "T2")
            closeIdlePane(viewModel) // 画面は購読していない（Channel に溜まる）
            runCurrent()

            viewModel.open(target(OPS_ID, "T1")) // 別のペインを開き直す
            runCurrent()

            viewModel.leave.test { expectNoEvents() }
        }

    @Test
    fun resettingOnLeavingTheScreenDropsAnUnconsumedLeaveEventAndTheOldState() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID, "T2")
            closeIdlePane(viewModel)
            runCurrent()

            viewModel.reset()
            viewModel.open(target(OPS_ID, "T2")) // 同じペインを開き直しても、前の表示を引き継がない
            runCurrent()

            viewModel.leave.test { expectNoEvents() }
            assertNull(viewModel.state.value.notice)
            assertFalse(viewModel.state.value.busy)
        }

    @Test
    fun theOldOptionButtonsStayOffUntilTheCardIsFetchedAgainAfterAnAnswer() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            assertTrue(viewModel.state.value.controls.showCardButtons)
            val refetch = CompletableDeferred<Unit>()
            api.cardGate = refetch // 再取得が終わらない間

            viewModel.pick(1)
            runCurrent()

            assertNull(viewModel.state.value.card) // 古い選択肢は消える
            assertFalse(viewModel.state.value.controls.showCardButtons)
            assertFalse(viewModel.state.value.busy) // 操作は終わっている

            refetch.complete(Unit)
            runCurrent()
            assertTrue(viewModel.state.value.controls.showCardButtons) // 取り直した質問が出る
        }

    @Test
    fun aFailedPickKeepsTheCardSoItCanBeTriedAgain() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            api.pick = api.pick.copy(ok = false, cardClosed = false)

            viewModel.pick(1)
            runCurrent()

            assertTrue(viewModel.state.value.controls.showCardButtons)
        }

    @Test
    fun theRetryButtonAppearsOnlyForAnExitTimeoutAndEveryOtherNoticeClearsIt() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID)
            api.failure = FetchException(ErrKind.Operation, "exit_timeout")
            closeIdlePane(viewModel)
            runCurrent()
            assertTrue(viewModel.state.value.retryClose)

            api.failure = FetchException(ErrKind.Operation, "invalid_state") // ほかの失敗
            closeIdlePane(viewModel)
            runCurrent()
            assertFalse(viewModel.state.value.retryClose)

            api.failure = FetchException(ErrKind.Operation, "exit_timeout")
            closeIdlePane(viewModel)
            runCurrent()
            assertTrue(viewModel.state.value.retryClose)

            api.failure = null // 成功（指示の送信）の通知
            viewModel.onSendTextChange("go")
            viewModel.send()
            runCurrent()
            assertEquals(
                "送りました",
                viewModel.state.value.notice
                    ?.message,
            )
            assertFalse(viewModel.state.value.retryClose)

            api.failure = FetchException(ErrKind.Operation, "exit_timeout")
            closeIdlePane(viewModel)
            runCurrent()
            api.failure = null
            api.close = api.close.copy(ok = false) // 終了の結果が ok:false の通知
            closeIdlePane(viewModel)
            runCurrent()
            assertFalse(viewModel.state.value.retryClose)
        }

    @Test
    fun aFetchStartedBeforeTheAnswerCannotBringTheClosedCardBack() =
        runPaneTest {
            state = PaneState.Asking
            val viewModel = opened(OPS_ID)
            val stale = CompletableDeferred<Unit>()
            api.cardGate = stale
            viewModel.refresh() // 答える前の ask_card を読んだまま、応答を待っている取得
            runCurrent()
            assertTrue(viewModel.state.value.controls.showCardButtons)

            api.card = AskCard(asking = false) // ホストでは答えた質問が閉じる
            viewModel.pick(1)
            runCurrent()
            stale.complete(Unit) // 古い取得が、答える前のカードを返そうとする
            runCurrent()

            assertFalse(
                viewModel.state.value.card
                    ?.asking ?: false,
            ) // 古いカードは戻らない
            assertFalse(viewModel.state.value.controls.showCardButtons)
            viewModel.pick(2) // ボタンが出ていないので、閉じた質問には送れない
            runCurrent()
            assertEquals(1, api.calls.count { it.startsWith("ask_pick") })
        }

    @Test
    fun theFetchAfterAnOperationIsAFreshOneEvenWhenAnotherIsRunning() =
        runPaneTest {
            state = PaneState.Idle
            val viewModel = opened(OPS_ID)
            val slow = CompletableDeferred<Unit>()
            api.tailGate = slow
            viewModel.refresh()
            runCurrent()
            val before = api.calls.count { it.startsWith("pane_tail") }

            viewModel.onSendTextChange("go")
            viewModel.send()
            runCurrent()

            assertTrue(api.calls.count { it.startsWith("pane_tail") } > before) // 実行中の取得に相乗りせず、取り直している
            slow.complete(Unit)
            runCurrent()
        }
}
