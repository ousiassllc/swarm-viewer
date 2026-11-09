package com.ousiassllc.swarmviewer.ui.pane

import app.cash.turbine.test
import com.ousiassllc.swarmviewer.domain.DirEntry
import com.ousiassllc.swarmviewer.domain.DirList
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
internal class StartSessionViewModelTest : PaneFixture() {
    @Test
    fun aViewOnlyConnectionShowsNothingToChooseAndSendsNothing() =
        runTest {
            val viewModel = openedStart(VIEW_ID)

            assertTrue(viewModel.state.value.viewOnly)
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/work/app")
            viewModel.onTitleChange("t")
            assertFalse(viewModel.state.value.canConfirm)
            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()

            assertEquals(emptyList<String>(), api.calls) // dir_list も pane_start も呼ばない
            assertFalse(viewModel.state.value.confirming)
        }

    @Test
    fun opensAtTheAllowedRootsAndWalksDownOneLevelAtATimeThenBack() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            assertEquals(listOf("dir_list "), api.calls) // path 省略 = allowed_roots
            assertEquals(
                null,
                viewModel.state.value.listing
                    ?.path,
            ) // 根の一覧
            assertEquals(
                1,
                viewModel.state.value.listing
                    ?.entries
                    ?.size,
            )
            assertFalse(viewModel.state.value.canChooseListed) // 根は選べない
            assertFalse(viewModel.state.value.canGoUp)

            api.dirs = DirList("/work", listOf(DirEntry("app", "/work/app", true)), truncated = true)
            viewModel.browse("/work")
            runCurrent()
            assertEquals(
                "/work",
                viewModel.state.value.listing
                    ?.path,
            )
            assertTrue(viewModel.state.value.canChooseListed)
            assertTrue(viewModel.state.value.canGoUp)
            assertTrue(
                viewModel.state.value.listing
                    ?.truncated ?: false,
            )
            assertTrue(
                viewModel.state.value.listing
                    ?.entries
                    ?.single()
                    ?.git ?: false,
            )

            viewModel.up()
            runCurrent()
            assertEquals(listOf("dir_list ", "dir_list /work", "dir_list "), api.calls)
            assertFalse(viewModel.state.value.canGoUp)
        }

    @Test
    fun theRecentFoldersAreTheCwdsOfAgentPanesWithoutDuplicates() =
        runTest {
            snapshots.refresh(OPS_ID, "s0") // report を保存（cached が読む）
            val viewModel = openedStart(OPS_ID)

            assertEquals(listOf("/work/app"), viewModel.state.value.recent) // agent ありの 2 ペインは同じ cwd、shell の /home/x は除く
        }

    @Test
    fun theFullFlowChoosesAgentFolderAndTitleThenConfirmsAndStarts() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            assertFalse(viewModel.state.value.canConfirm)

            viewModel.selectAgent("claude")
            viewModel.browse("/work")
            runCurrent()
            viewModel.chooseListed()
            viewModel.onTitleChange("  新機能  ")
            assertTrue(viewModel.state.value.canConfirm)

            viewModel.started.test {
                viewModel.requestConfirm()
                assertTrue(viewModel.state.value.confirming)
                assertEquals(listOf("dir_list ", "skill_list claude", "dir_list /work"), api.calls) // 確認だけでは起動しない
                viewModel.start()
                runCurrent()
                assertEquals("pane_start s0 B1 claude /work 新機能", api.calls.last()) // タイトルは前後の空白を除く
                awaitItem()
                cancelAndIgnoreRemainingEvents()
            }
            assertEquals("起動しました", viewModel.state.value.message)
            assertFalse(viewModel.state.value.busy)
        }

    @Test
    fun anUnsuccessfulStartShowsTheReasonAndDoesNotNavigate() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            api.start = api.start.copy(ok = false, state = "shell")
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/home/x")
            viewModel.onTitleChange("t")

            viewModel.started.test {
                viewModel.requestConfirm()
                viewModel.start()
                runCurrent()
                expectNoEvents()
            }
            assertEquals("起動を確認できませんでした（状態: 空き）", viewModel.state.value.message)
        }

    @Test
    fun aForbiddenFolderIsShownWithTheFixedMessage() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            api.failure = FetchException(ErrKind.Operation, "forbidden_path")
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/etc")
            viewModel.onTitleChange("t")

            viewModel.requestConfirm()
            viewModel.start()
            runCurrent()

            assertEquals(operationErrorMessage("forbidden_path"), viewModel.state.value.message)
            assertEquals("このフォルダは選べません", viewModel.state.value.message)
        }

    @Test
    fun theButtonsAreDisabledWhileStartingAndASecondStartSendsNothing() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/work/app")
            viewModel.onTitleChange("t")
            viewModel.requestConfirm()
            api.gate = CompletableDeferred()

            viewModel.start()
            runCurrent()
            assertTrue(viewModel.state.value.busy)
            viewModel.start()
            viewModel.requestConfirm()
            viewModel.selectAgent("codex")
            runCurrent()

            assertEquals(1, api.calls.count { it.startsWith("pane_start") })
            assertEquals("omp", viewModel.state.value.agent)
            api.gate?.complete(Unit)
            runCurrent()
            assertFalse(viewModel.state.value.busy)
        }

    @Test
    fun anInvalidTitleCannotBeConfirmed() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/work/app")

            for (title in listOf("", "   ", "a\nb", "あ".repeat(81))) {
                viewModel.onTitleChange(title)
                assertFalse(title, viewModel.state.value.canConfirm)
            }
            viewModel.onTitleChange("あ".repeat(80))
            assertTrue(viewModel.state.value.canConfirm)
        }

    @Test
    fun aFailedFolderListingKeepsThePreviousLevelWithAFixedMessage() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            api.failure = FetchException(ErrKind.Operation, "not_found")

            viewModel.browse("/gone")
            runCurrent()

            assertEquals("フォルダが見つかりません", viewModel.state.value.message)
            assertNull(
                viewModel.state.value.listing
                    ?.path,
            ) // 根のまま
            assertFalse(viewModel.state.value.canGoUp)
        }
}
