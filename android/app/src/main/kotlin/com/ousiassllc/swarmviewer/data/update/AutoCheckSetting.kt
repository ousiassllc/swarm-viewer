package com.ousiassllc.swarmviewer.data.update

import android.content.Context
import androidx.core.content.edit
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/** 新しい版を自動で確認するか（設定の「自動で確認する」。既定はオン。#109）。 */
interface AutoCheckSetting {
    val enabled: StateFlow<Boolean>

    fun set(enabled: Boolean)
}

/** 設定の保存名（SharedPreferences。バックアップ・端末間転送の対象外）。 */
const val UPDATE_PREFS_NAME = "update_prefs"
const val AUTO_CHECK_KEY = "auto_check"

/** [AutoCheckSetting] の SharedPreferences 実装（プロセスをまたいで保つ。値は真偽値だけで、token・URL は持たない）。 */
class PrefsAutoCheck(
    context: Context,
) : AutoCheckSetting {
    private val prefs = context.applicationContext.getSharedPreferences(UPDATE_PREFS_NAME, Context.MODE_PRIVATE)
    private val state = MutableStateFlow(prefs.getBoolean(AUTO_CHECK_KEY, true))
    override val enabled: StateFlow<Boolean> = state.asStateFlow()

    override fun set(enabled: Boolean) {
        prefs.edit { putBoolean(AUTO_CHECK_KEY, enabled) }
        state.value = enabled
    }
}
