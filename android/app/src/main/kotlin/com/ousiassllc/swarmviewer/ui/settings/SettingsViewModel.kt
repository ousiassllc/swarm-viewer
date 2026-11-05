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

/**
 * 設定画面（A3）の状態。[settings] は null の間は読み込み中。[notificationsAllowed] は OS が通知を出せる状態か
 * （権限の拒否・システム設定での無効化のとき false）。通知がオンなのに false の間は、案内を出す。
 */
data class SettingsUiState(
    val settings: Settings? = null,
    val notificationsAllowed: Boolean = true,
    val message: String? = null,
) {
    /** 通知がオンだが OS が許可していない（権限の案内を出す）。 */
    val showsPermissionGuide: Boolean get() = settings?.notificationsEnabled == true && !notificationsAllowed
}

class SettingsViewModel(
    private val service: SettingsService,
    private val isNotificationsAllowed: () -> Boolean,
) : ViewModel() {
    private val mutableState = MutableStateFlow(SettingsUiState())
    val state: StateFlow<SettingsUiState> = mutableState.asStateFlow()

    init {
        refresh()
    }

    /** 設定と通知の許可の状態を読み直す（画面を開いたとき・システムの設定から戻ったとき）。 */
    fun refresh() {
        viewModelScope.launch {
            attempt { mutableState.update { it.copy(settings = service.get(), notificationsAllowed = isNotificationsAllowed()) } }
        }
    }

    /** 画面更新間隔（秒。5 / 10 / 30 / 60）を保存する。許容外の値は保存しない。次の取得から新しい間隔になる。 */
    fun setScreenPollSeconds(seconds: Int) = save { service.setScreenPollSeconds(seconds) }

    /** 通知のオン / オフを保存する。オフの間は周期ジョブも止まり、オンに戻すと登録し直す。 */
    fun setNotificationsEnabled(enabled: Boolean) = save { service.setNotificationsEnabled(enabled) }

    /** 通知確認間隔（分。15 / 30 / 60）を保存する。許容外の値は保存しない。 */
    fun setNotifyPollMinutes(minutes: Int) = save { service.setNotifyPollMinutes(minutes) }

    private fun save(block: suspend () -> Settings) {
        mutableState.update { it.copy(message = null) }
        viewModelScope.launch {
            attempt { mutableState.update { it.copy(settings = block(), notificationsAllowed = isNotificationsAllowed()) } }
        }
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
