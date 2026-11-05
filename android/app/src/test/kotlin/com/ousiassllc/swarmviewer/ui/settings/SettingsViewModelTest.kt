package com.ousiassllc.swarmviewer.ui.settings

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class SettingsViewModelTest {
    private val store = FakeViewerStore()
    private var allowed = true

    @Before
    fun setUp() = Dispatchers.setMain(UnconfinedTestDispatcher())

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun theScreenShowsTheSavedIntervalAndChangingItSavesIt() {
        val viewModel = SettingsViewModel(SettingsService(store)) { allowed }
        assertEquals(
            10,
            viewModel.state.value.settings
                ?.screenPollSeconds,
        )

        viewModel.setScreenPollSeconds(60)

        assertEquals(
            60,
            viewModel.state.value.settings
                ?.screenPollSeconds,
        )
        assertEquals(60, store.settingsRow?.screenPollSeconds)
        assertNull(viewModel.state.value.message)
    }

    @Test
    fun aRejectedValueLeavesTheSettingsAndShowsAFixedMessage() {
        val viewModel = SettingsViewModel(SettingsService(store)) { allowed }

        viewModel.setScreenPollSeconds(7)

        assertEquals(
            10,
            viewModel.state.value.settings
                ?.screenPollSeconds,
        )
        assertEquals(UNEXPECTED_ERROR_MESSAGE, viewModel.state.value.message)
        assertNotNull(viewModel.state.value.settings)
    }

    @Test
    fun notificationSettingsAreSavedAndTheSchedulerHookIsCalledOnEverySave() {
        val saved = ArrayList<Settings>()
        val viewModel = SettingsViewModel(SettingsService(store, onSaved = { saved += it })) { allowed }

        viewModel.setNotificationsEnabled(false)
        viewModel.setNotifyPollMinutes(60)
        viewModel.setNotificationsEnabled(true)

        assertEquals(listOf(false, false, true), saved.map { it.notificationsEnabled })
        assertEquals(listOf(15, 60, 60), saved.map { it.notifyPollMinutes })
        assertEquals(60, store.settingsRow?.notifyPollMinutes)
    }

    @Test
    fun theNotificationIntervalOutsideTheAllowedValuesIsRejected() {
        val viewModel = SettingsViewModel(SettingsService(store)) { allowed }

        viewModel.setNotifyPollMinutes(10)

        assertEquals(
            15,
            viewModel.state.value.settings
                ?.notifyPollMinutes,
        )
        assertEquals(UNEXPECTED_ERROR_MESSAGE, viewModel.state.value.message)
    }

    @Test
    fun thePermissionGuideAppearsOnlyWhenNotificationsAreOnButTheOsDoesNotAllowThem() {
        allowed = false
        val viewModel = SettingsViewModel(SettingsService(store)) { allowed }
        assertTrue(viewModel.state.value.showsPermissionGuide)

        viewModel.setNotificationsEnabled(false) // オフなら案内は不要
        assertFalse(viewModel.state.value.showsPermissionGuide)

        allowed = true // システムの設定で許可して戻ってくる
        viewModel.setNotificationsEnabled(true)
        viewModel.refresh()
        assertFalse(viewModel.state.value.showsPermissionGuide)
    }
}
