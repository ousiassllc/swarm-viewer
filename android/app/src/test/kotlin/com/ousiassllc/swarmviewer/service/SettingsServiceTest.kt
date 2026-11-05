package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.SettingsException
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class SettingsServiceTest {
    private val store = FakeViewerStore()
    private val service = SettingsService(store)

    @Test
    fun theDefaultScreenIntervalIsTenSeconds() =
        runBlocking {
            assertEquals(10, service.get().screenPollSeconds)
        }

    @Test
    fun everyAllowedIntervalCanBeSavedAndOnlyThatItemChanges() =
        runBlocking {
            store.settingsRow = store.settings().copy(notifyPollMinutes = 30, lastConnectionId = null)
            for (seconds in listOf(5, 10, 30, 60)) {
                val saved = service.setScreenPollSeconds(seconds)
                assertEquals(seconds, saved.screenPollSeconds)
                assertEquals(seconds, store.settings().screenPollSeconds)
            }
            assertEquals(30, store.settings().notifyPollMinutes)
        }

    @Test
    fun anIntervalOutsideTheAllowedValuesIsRejectedAndNotSaved() =
        runBlocking {
            service.setScreenPollSeconds(30)

            val e = assertThrows(SettingsException::class.java) { runBlocking { service.setScreenPollSeconds(7) } }

            assertEquals(listOf("screen_poll_seconds"), e.fields)
            assertEquals(30, store.settings().screenPollSeconds)
        }
}
