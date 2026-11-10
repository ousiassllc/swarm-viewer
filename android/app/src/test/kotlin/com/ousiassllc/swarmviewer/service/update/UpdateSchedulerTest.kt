package com.ousiassllc.swarmviewer.service.update

import com.ousiassllc.swarmviewer.domain.AppVersion
import com.ousiassllc.swarmviewer.service.UPDATE_CHECK_INTERVAL
import com.ousiassllc.swarmviewer.service.UpdateScheduler
import com.ousiassllc.swarmviewer.service.UpdateService
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.minutes

/** 起動時と 6 時間ごとの確認、自動確認の切り替え（#109）。時間は仮想時間で進める。 */
class UpdateSchedulerTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val source = FakeReleaseSource()
    private val autoCheck = FakeAutoCheck()

    private fun service() =
        UpdateService(source, FakeSigners(), FakeInstaller(), autoCheck, AppVersion(0, 1, 0), File(tmp.root, "u"), Dispatchers.Unconfined)

    @Test
    fun theDefaultIntervalIsSixHours() {
        assertEquals(6.hours, UPDATE_CHECK_INTERVAL)
    }

    @Test
    fun itChecksAtStartAndThenEverySixHours() =
        runTest {
            UpdateScheduler(service(), autoCheck).start(backgroundScope)
            runCurrent()
            assertEquals(1, source.latestCalls)
            advanceTimeBy(6.hours - 1.minutes)
            assertEquals(1, source.latestCalls)
            advanceTimeBy(2.minutes)
            assertEquals(2, source.latestCalls)
            advanceTimeBy(6.hours)
            assertEquals(3, source.latestCalls)
        }

    @Test
    fun startingTwiceDoesNotDoubleTheChecks() =
        runTest {
            val scheduler = UpdateScheduler(service(), autoCheck)
            scheduler.start(backgroundScope)
            scheduler.start(backgroundScope)
            runCurrent()
            assertEquals(1, source.latestCalls)
        }

    // (g)
    @Test
    fun withAutoCheckOffNothingIsChecked() =
        runTest {
            autoCheck.set(false)
            UpdateScheduler(service(), autoCheck).start(backgroundScope)
            runCurrent()
            advanceTimeBy(24.hours)
            assertEquals(0, source.latestCalls)
        }

    @Test
    fun turningItOffStopsAndTurningItOnChecksAgainAtOnce() =
        runTest {
            UpdateScheduler(service(), autoCheck).start(backgroundScope)
            runCurrent()
            assertEquals(1, source.latestCalls)
            autoCheck.set(false)
            runCurrent()
            advanceTimeBy(12.hours)
            assertEquals(1, source.latestCalls)
            autoCheck.set(true)
            runCurrent()
            assertEquals(2, source.latestCalls)
        }
}
