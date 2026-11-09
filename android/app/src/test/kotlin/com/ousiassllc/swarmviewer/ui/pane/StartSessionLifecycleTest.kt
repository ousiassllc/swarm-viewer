package com.ousiassllc.swarmviewer.ui.pane

import app.cash.turbine.test
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
internal class StartSessionLifecycleTest : PaneFixture() {
    @Test
    fun aFailedFirstListingCanBeRetriedFromTheReloadButton() =
        runTest {
            api.failure = FetchException(ErrKind.Operation, "not_found")
            val viewModel = openedStart(OPS_ID)
            assertNull(viewModel.state.value.listing) // 最初の dir_list が失敗
            assertFalse(viewModel.state.value.listingLoading)

            api.failure = null
            viewModel.retryListing()
            runCurrent()

            assertEquals(
                1,
                viewModel.state.value.listing
                    ?.entries
                    ?.size,
            )
            assertNull(viewModel.state.value.message)
            assertEquals(listOf("dir_list ", "dir_list "), api.calls) // 再試行は allowed_roots から

            viewModel.retryListing() // 読み込めているときは何もしない
            runCurrent()
            assertEquals(2, api.calls.size)
        }

    @Test
    fun leavingTheScreenDiscardsTheInputsSoReEnteringStartsFromScratch() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            viewModel.selectAgent("claude")
            viewModel.chooseRecent("/work/app")
            viewModel.onTitleChange("old title")
            viewModel.browse("/work")
            runCurrent()

            viewModel.reset() // 画面を離れた
            viewModel.open(target(OPS_ID, "B1")) // 同じペインに入り直す
            runCurrent()

            val ui = viewModel.state.value
            assertNull(ui.agent)
            assertNull(ui.folder)
            assertEquals("", ui.title)
            assertFalse(ui.canGoUp)
            assertNull(ui.listing?.path) // 根の一覧から
            assertFalse(ui.canConfirm)
        }

    @Test
    fun aStartedEventNobodyConsumedIsNotReplayedOnTheNextVisit() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/work/app")
            viewModel.onTitleChange("t")
            viewModel.requestConfirm()
            viewModel.start() // 画面は購読していない
            runCurrent()

            viewModel.reset()
            viewModel.open(target(OPS_ID, "B1"))
            runCurrent()

            viewModel.started.test { expectNoEvents() }
        }

    @Test
    fun leavingWhileStartingCancelsTheWaitSoNothingIsSentToTheNextScreen() =
        runTest {
            val viewModel = openedStart(OPS_ID)
            viewModel.selectAgent("omp")
            viewModel.chooseRecent("/work/app")
            viewModel.onTitleChange("t")
            viewModel.requestConfirm()
            api.gate = CompletableDeferred()
            viewModel.start()
            runCurrent()

            viewModel.reset()
            api.gate?.complete(Unit)
            runCurrent()

            assertFalse(viewModel.state.value.busy)
            assertNull(viewModel.state.value.message)
            viewModel.started.test { expectNoEvents() }
        }
}
