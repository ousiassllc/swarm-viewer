package com.ousiassllc.swarmviewer.ui.pane

import app.cash.turbine.test
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.ui.FIRST_TASK_FAILED_MESSAGE
import com.ousiassllc.swarmviewer.ui.SKILL_LOAD_FAILED_MESSAGE
import com.ousiassllc.swarmviewer.ui.SKILL_UNSUPPORTED_MESSAGE
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** 新しいセッションの最初のタスク（FR-OPS-11・FR-AND-12）の ViewModel。 */
@OptIn(ExperimentalCoroutinesApi::class)
internal class StartSessionFirstTaskTest : PaneFixture() {
    private fun TestScope.ready(task: String = "本文"): StartSessionViewModel {
        val viewModel = openedStart(OPS_ID)
        viewModel.selectAgent("omp")
        runCurrent()
        viewModel.chooseRecent("/work/app")
        viewModel.onTitleChange("t")
        viewModel.onTaskChange(task)
        return viewModel
    }

    private fun sendCalls() = api.calls.filter { it.startsWith("send_text") }

    @Test
    fun aViewOnlyConnectionShowsNoSkillOrBodyPartsAndNeverCallsSkillList() =
        runTest {
            val viewModel = openedStart(VIEW_ID)
            viewModel.selectAgent("omp")
            viewModel.selectSkill(api.skills.skills.first())
            viewModel.onTaskChange("x")
            runCurrent()

            assertTrue(viewModel.state.value.viewOnly) // 画面は viewOnly のとき StartForm（スキル・本文の部品）を描画しない
            assertNull(viewModel.state.value.agent)
            assertEquals(emptyList<String>(), api.calls)
        }

    @Test
    fun choosingAnAgentFetchesItsSkillsAndChangingItClearsTheSelectedSkill() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            assertEquals(SkillListing.None, viewModel.state.value.skills)

            viewModel.selectAgent("omp")
            runCurrent()
            val loaded = viewModel.state.value.skills as SkillListing.Loaded
            assertEquals(listOf("impl", "bug-fix"), loaded.skills.map { it.name })
            viewModel.selectSkill(loaded.skills.first())
            assertEquals(
                "/impl",
                viewModel.state.value.skill
                    ?.invoke,
            )

            viewModel.selectAgent("claude")
            runCurrent()
            assertNull(viewModel.state.value.skill) // エージェントが変わったらスキルを外す
            assertEquals(listOf("skill_list omp", "skill_list claude"), api.calls.filter { it.startsWith("skill_list") })

            viewModel.selectSkill(
                api.skills.skills
                    .first()
                    .copy(invoke = "/not-in-the-list"),
            )
            assertNull(viewModel.state.value.skill) // 一覧に無いスキルは選べない
        }

    @Test
    fun unknownToolShowsTheFixedMessageWithoutReloadAndStartingStillWorks() =
        runTest {
            api.skillFailure = FetchException(ErrKind.Operation, "unknown_tool")
            val viewModel = ready()

            val failed = viewModel.state.value.skills as SkillListing.Failed
            assertEquals(SKILL_UNSUPPORTED_MESSAGE, failed.message)
            assertFalse(failed.reload) // 読み直しても変わらない
            viewModel.retrySkills()
            runCurrent()
            assertEquals(1, api.calls.count { it.startsWith("skill_list") })

            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()
            assertEquals(listOf("send_text s0 B1 本文"), sendCalls()) // 本文だけのタスクは送れる
        }

    @Test
    fun otherFailuresOfSkillListOfferAReloadAndTheRetryFetchesAgain() =
        runTest {
            api.skillFailure = FetchException(ErrKind.Unreachable, "")
            val viewModel = ready(task = "")
            val failed = viewModel.state.value.skills as SkillListing.Failed
            assertEquals(SKILL_LOAD_FAILED_MESSAGE, failed.message)
            assertTrue(failed.reload)
            assertTrue(viewModel.state.value.canConfirm) // スキルなし・本文なし = タスクなしの起動はできる

            api.skillFailure = null
            viewModel.retrySkills()
            runCurrent()

            assertTrue(viewModel.state.value.skills is SkillListing.Loaded)
        }

    @Test
    fun startingSendsPaneStartThenTheSkillAndBodyAndNavigatesWithTheToast() =
        runTest {
            val viewModel = ready("#12 を直す")
            viewModel.selectSkill(api.skills.skills.first())

            viewModel.started.test {
                viewModel.requestConfirm()
                viewModel.start()
                runCurrent()
                awaitItem()
                cancelAndIgnoreRemainingEvents()
            }

            assertEquals(
                listOf("pane_start s0 B1 omp /work/app t", "send_text s0 B1 /impl #12 を直す"),
                operationCalls().filterNot {
                    it.startsWith("dir_list") ||
                        it.startsWith("skill_list")
                },
            )
            assertEquals("起動しました", viewModel.state.value.message)
            assertNull(viewModel.state.value.notice)
        }

    @Test
    fun aNonIdleStartDoesNotSendAndShowsTheResultWithoutResend() =
        runTest {
            api.start = api.start.copy(state = "working")
            val viewModel = ready()

            viewModel.started.test {
                viewModel.requestConfirm()
                viewModel.start()
                runCurrent()
                expectNoEvents() // 遷移しない
            }

            assertTrue(sendCalls().isEmpty())
            val notice = checkNotNull(viewModel.state.value.notice)
            assertEquals("起動しました。最初のタスクはまだ送っていません（状態: 作業中）。ペイン詳細から送れます", notice.message)
            assertFalse(notice.canResend) // 「もう一度送る」は出さない
            assertEquals("本文", viewModel.state.value.task) // 本文はこの表示の間だけ残る

            viewModel.started.test {
                viewModel.openPane()
                awaitItem()
                cancelAndIgnoreRemainingEvents()
            }
        }

    @Test
    fun aFailedSendKeepsTheStartAndResendSendsOnlySendTextAfterAnIdleCheck() =
        runTest {
            api.sendFailure = FetchException(ErrKind.Operation, "busy_timeout")
            val viewModel = ready()
            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()

            val notice = checkNotNull(viewModel.state.value.notice)
            assertEquals(FIRST_TASK_FAILED_MESSAGE, notice.message)
            assertEquals("同じペインの別の操作がまだ終わっていません。しばらくしてからお試しください", notice.reason)
            assertTrue(notice.canResend)
            assertTrue(api.calls.none { it.startsWith("pane_close") }) // 起動を取り消さない
            assertEquals(1, api.calls.count { it.startsWith("pane_start") })

            api.sendFailure = null
            startedPaneState = PaneState.Idle
            viewModel.started.test {
                viewModel.resend()
                runCurrent()
                awaitItem()
                cancelAndIgnoreRemainingEvents()
            }
            assertEquals(1, api.calls.count { it.startsWith("pane_start") }) // pane_start は呼ばない
            assertEquals(2, sendCalls().size)
        }

    @Test
    fun resendRefusesWithoutSendingWhenThePaneIsNotIdle() =
        runTest {
            api.send = api.send.copy(accepted = false)
            val viewModel = ready()
            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()
            assertEquals(
                "受け付けられませんでした",
                viewModel.state.value.notice
                    ?.reason,
            ) // accepted: false

            startedPaneState = PaneState.Working
            viewModel.resend()
            runCurrent()

            assertEquals(1, sendCalls().size) // 再送は通信しない
            assertEquals(
                "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください",
                viewModel.state.value.notice
                    ?.reason,
            )
            assertTrue(
                viewModel.state.value.notice
                    ?.canResend ?: false,
            )
        }

    @Test
    fun anInvalidBodyCannotBeConfirmedAndNothingIsSent() =
        runTest {
            val viewModel = ready()

            viewModel.onTaskChange("a\u001bb")
            assertEquals("forbidden_control_char", viewModel.state.value.taskProblem)
            assertFalse(viewModel.state.value.canConfirm)
            viewModel.onTaskChange("あ".repeat(8001))
            assertEquals("text_too_long", viewModel.state.value.taskProblem)
            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()
            assertTrue(api.calls.none { it.startsWith("pane_start") })

            viewModel.onTaskChange("a\r\nb\rc") // 改行は \n に揃える
            assertEquals("a\nb\nc", viewModel.state.value.task)
            assertTrue(viewModel.state.value.canConfirm)
        }

    @Test
    fun theSelectedSkillAndBodySurviveARecreatedScreenButResetDiscardsThem() =
        runTest {
            val viewModel = ready("書きかけ")
            viewModel.selectSkill(api.skills.skills.last())
            viewModel.open(target(OPS_ID, "B1")) // 構成変更（折りたたみ⇔展開）。同じペインなら何もしない
            runCurrent()

            assertEquals(
                "/bug-fix",
                viewModel.state.value.skill
                    ?.invoke,
            )
            assertEquals("書きかけ", viewModel.state.value.task)

            viewModel.reset()
            val ui = viewModel.state.value
            assertNull(ui.skill)
            assertEquals("", ui.task)
            assertEquals(SkillListing.None, ui.skills)
            assertNull(ui.notice)
        }

    @Test
    fun theConfirmationSummarisesTheBodyHeadOnly() =
        runTest {
            val viewModel = ready("  " + "あ".repeat(100) + "  ")

            assertEquals("あ".repeat(80) + "…", viewModel.state.value.taskHead)
            viewModel.onTaskChange(" 短い ")
            assertEquals("短い", viewModel.state.value.taskHead)
        }
}
