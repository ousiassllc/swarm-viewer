package com.ousiassllc.swarmviewer.ui.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** 設定画面（A3）の状態。[settings] は null の間は読み込み中。 */
data class SettingsUiState(
    val settings: Settings? = null,
    val message: String? = null,
)

class SettingsViewModel(
    private val service: SettingsService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(SettingsUiState())
    val state: StateFlow<SettingsUiState> = mutableState.asStateFlow()

    init {
        refresh()
    }

    /** 設定を読み直す（画面を開いたとき）。 */
    fun refresh() {
        viewModelScope.launch { attempt { mutableState.update { it.copy(settings = service.get()) } } }
    }

    /** 画面更新間隔（秒。5 / 10 / 30 / 60）を保存する。許容外の値は保存しない。次の取得から新しい間隔になる。 */
    fun setScreenPollSeconds(seconds: Int) {
        mutableState.update { it.copy(message = null) }
        viewModelScope.launch { attempt { mutableState.update { it.copy(settings = service.setScreenPollSeconds(seconds)) } } }
    }

    private suspend fun attempt(block: suspend () -> Unit) {
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            mutableState.update { it.copy(message = UNEXPECTED_ERROR_MESSAGE) } // 例外の本文は使わない
        }
    }
}
