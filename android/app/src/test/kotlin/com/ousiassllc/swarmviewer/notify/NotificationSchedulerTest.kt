package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.ALLOWED_NOTIFY_POLL_MINUTES
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.service.SettingsService
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

private class FakeJobs : PeriodicJobs {
    val log = ArrayList<String>()
    var scheduledMinutes: Int? = null

    override fun schedule(intervalMinutes: Int) {
        scheduledMinutes = intervalMinutes
        log += "schedule:$intervalMinutes"
    }

    override fun cancel() {
        scheduledMinutes = null
        log += "cancel"
    }
}

class NotificationSchedulerTest {
    private val jobs = FakeJobs()
    private val scheduler = NotificationScheduler(jobs)

    @Test
    fun theDefaultIsEnabledWithFifteenMinutes() {
        scheduler.apply(Settings())

        assertEquals(15, jobs.scheduledMinutes)
    }

    @Test
    fun everyAllowedIntervalIsRegisteredAsIs() {
        for (minutes in ALLOWED_NOTIFY_POLL_MINUTES) {
            scheduler.apply(Settings(notifyPollMinutes = minutes))
            assertEquals(minutes, jobs.scheduledMinutes)
        }
        assertEquals(listOf(15, 30, 60), ALLOWED_NOTIFY_POLL_MINUTES) // 5・10 分は選択肢にない（最小は WorkManager の 15 分）
    }

    @Test
    fun turningNotificationsOffStopsTheJobAndTurningThemOnRegistersItAgain() {
        scheduler.apply(Settings(notifyPollMinutes = 30))
        scheduler.apply(Settings(notificationsEnabled = false, notifyPollMinutes = 30))
        assertNull(jobs.scheduledMinutes) // オフの間はジョブを登録しない

        scheduler.apply(Settings(notificationsEnabled = true, notifyPollMinutes = 30))
        assertEquals(30, jobs.scheduledMinutes)
        assertEquals(listOf("schedule:30", "cancel", "schedule:30"), jobs.log)
    }

    @Test
    fun savingTheSettingsReRegistersTheJobThroughTheService() =
        runBlocking {
            val service = SettingsService(FakeViewerStore(), onSaved = scheduler::apply)

            service.setNotifyPollMinutes(60)
            assertEquals(60, jobs.scheduledMinutes)
            service.setNotificationsEnabled(false)
            assertNull(jobs.scheduledMinutes)
            service.setNotificationsEnabled(true)
            assertEquals(60, jobs.scheduledMinutes) // オンに戻すと、保存済みの間隔で登録し直す
        }

    @Test
    fun aRejectedValueNeitherSavesNorTouchesTheJob() {
        val service = SettingsService(FakeViewerStore(), onSaved = scheduler::apply)

        runCatching { runBlocking { service.setNotifyPollMinutes(10) } } // 10 分は許容外

        assertEquals(emptyList<String>(), jobs.log)
    }
}
