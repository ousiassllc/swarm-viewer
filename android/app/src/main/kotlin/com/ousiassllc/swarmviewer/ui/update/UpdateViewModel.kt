package com.ousiassllc.swarmviewer.ui.update

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.data.update.AutoCheckSetting
import com.ousiassllc.swarmviewer.service.UpdateService
import com.ousiassllc.swarmviewer.service.UpdateState
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

/** 新しい版の通知（メイン画面）と「自動で確認する」の設定（設定画面）の状態と操作（#109）。確認・更新は [UpdateService] が行う。 */
class UpdateViewModel(
    private val service: UpdateService,
    private val autoCheck: AutoCheckSetting,
) : ViewModel() {
    val state: StateFlow<UpdateState> = service.state

    /** 「自動で確認する」の設定（既定はオン）。 */
    val autoCheckEnabled: StateFlow<Boolean> = autoCheck.enabled

    /** 「更新する」。ダウンロード・検証・インストーラの起動までを行う（失敗は [UpdateState.failure]）。 */
    fun update() {
        viewModelScope.launch { service.update() }
    }

    fun setAutoCheck(enabled: Boolean) = autoCheck.set(enabled)
}
