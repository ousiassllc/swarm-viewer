package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Settings
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
internal class MainViewModelPollingTest : MainViewModelFixture() {
    @Test
    fun theNextRequestIsNotIssuedWhileThePreviousOneIsRunning() =
        runTest {
            reports.gate = CompletableDeferred()
            val viewModel = started()
            assertEquals(1, reports.calls.size)

            passTime(60 * DEFAULT_INTERVAL_MILLIS) // 取得が終わらない間は、間隔が過ぎても次を出さない
            assertEquals(1, reports.calls.size)

            viewModel.refresh() // プルして更新も、実行中の取得に相乗りする
            runCurrent()
            assertEquals(1, reports.calls.size)

            reports.gate?.complete(Unit)
            runCurrent()
            assertEquals(1, reports.calls.size)
            passTime(DEFAULT_INTERVAL_MILLIS) // 完了から間隔のあとに次
            assertEquals(2, reports.calls.size)
            viewModel.onStop()
        }

    @Test
    fun pollingFollowsTheConfiguredInterval() =
        runTest {
            store.settingsRow = Settings(screenPollSeconds = 30)
            val viewModel = started()
            assertEquals(1, reports.calls.size)

            passTime(29_000)
            assertEquals(1, reports.calls.size)
            passTime(1_000)
            assertEquals(2, reports.calls.size)
            viewModel.onStop()
        }

    @Test
    fun goingToTheBackgroundStopsPollingAndReturningFetchesImmediately() =
        runTest {
            val viewModel = started()
            assertEquals(1, reports.calls.size)

            viewModel.onStop()
            passTime(10 * DEFAULT_INTERVAL_MILLIS)
            assertEquals(1, reports.calls.size) // バックグラウンドでは取得しない

            viewModel.onStart()
            runCurrent()
            assertEquals(2, reports.calls.size) // 復帰時に即時取得
            assertTrue(viewModel.state.value.live)
            viewModel.onStop()
        }

    @Test
    fun goingToTheBackgroundCancelsTheRunningRequest() =
        runTest {
            reports.gate = CompletableDeferred()
            val viewModel = started()
            assertEquals(1, reports.calls.size)

            viewModel.onStop()
            reports.gate?.complete(Unit)
            runCurrent()

            assertNull(viewModel.state.value.snapshot) // 止めた取得の結果は反映しない
            assertFalse(viewModel.state.value.refreshing)
        }
}
