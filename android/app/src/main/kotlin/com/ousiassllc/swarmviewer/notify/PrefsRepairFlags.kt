package com.ousiassllc.swarmviewer.notify

import android.content.Context
import androidx.core.content.edit
import com.ousiassllc.swarmviewer.service.RepairFlags

private const val PREFS_NAME = "repair_flags"
private const val KEY_IDS = "connection_ids"

/** [RepairFlags] の SharedPreferences 実装（プロセスをまたいで保つ）。接続 ID だけを持つ（token・URL は持たない。ファイルはバックアップの対象外）。 */
class PrefsRepairFlags(
    context: Context,
) : RepairFlags {
    private val prefs = context.applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    @Synchronized
    override fun isFlagged(connectionId: String): Boolean = ids().contains(connectionId)

    @Synchronized
    override fun flag(connectionId: String): Boolean {
        val current = ids()
        if (connectionId in current) return false
        prefs.edit { putStringSet(KEY_IDS, current + connectionId) }
        return true
    }

    @Synchronized
    override fun clear(connectionId: String) {
        val current = ids()
        if (connectionId in current) prefs.edit { putStringSet(KEY_IDS, current - connectionId) }
    }

    // getStringSet が返す集合は変更してはいけないため、毎回コピーを使う。
    private fun ids(): Set<String> = prefs.getStringSet(KEY_IDS, emptySet()).orEmpty().toSet()
}
