package com.ousiassllc.swarmviewer.ui.settings

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class SettingsViewModelTest {
    private val store = FakeViewerStore()

    @Before
    fun setUp() = Dispatchers.setMain(UnconfinedTestDispatcher())

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun theScreenShowsTheSavedIntervalAndChangingItSavesIt() {
        val viewModel = SettingsViewModel(SettingsService(store))
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
        val viewModel = SettingsViewModel(SettingsService(store))

        viewModel.setScreenPollSeconds(7)

        assertEquals(
            10,
            viewModel.state.value.settings
                ?.screenPollSeconds,
        )
        assertEquals(UNEXPECTED_ERROR_MESSAGE, viewModel.state.value.message)
        assertNotNull(viewModel.state.value.settings)
    }
}
