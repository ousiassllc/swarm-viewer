package com.ousiassllc.swarmviewer.domain

/** 設定の既定値（ER §7.2。Android 版の通知確認間隔の既定は WorkManager の最小間隔の 15 分）。 */
const val DEFAULT_SCREEN_POLL_SECONDS = 10
const val DEFAULT_NOTIFY_POLL_MINUTES = 15
const val DEFAULT_NOTIFICATIONS_ENABLED = true

/** 画面更新間隔（秒）の許容値（昇順）。 */
val ALLOWED_SCREEN_POLL_SECONDS: List<Int> = listOf(5, 10, 30, 60)

/**
 * 通知確認間隔（分）の許容値（昇順）。WorkManager の周期ジョブの最小間隔が 15 分のため、デスクトップ版の 5・10 分は持たない。
 */
val ALLOWED_NOTIFY_POLL_MINUTES: List<Int> = listOf(15, 30, 60)

/** 設定項目名（[SettingsException.fields] の要素）。 */
const val FIELD_SCREEN_POLL_SECONDS = "screen_poll_seconds"
const val FIELD_NOTIFY_POLL_MINUTES = "notify_poll_minutes"

/** 許容外の設定値。[fields] は不正な項目名。メッセージは項目名だけで、値は含めない。 */
class SettingsException(
    val fields: List<String>,
) : Exception("invalid settings: ${fields.joinToString(", ")}")

/** アプリ全体の設定（1 行固定）。[lastConnectionId] は未設定（NULL）のとき null。 */
data class Settings(
    val notificationsEnabled: Boolean = DEFAULT_NOTIFICATIONS_ENABLED,
    val screenPollSeconds: Int = DEFAULT_SCREEN_POLL_SECONDS,
    val notifyPollMinutes: Int = DEFAULT_NOTIFY_POLL_MINUTES,
    val debugLog: Boolean = false,
    val lastConnectionId: String? = null,
) {
    /** 許容値を検査する。問題があれば [SettingsException]。 */
    fun validate() {
        val fields =
            buildList {
                if (screenPollSeconds !in ALLOWED_SCREEN_POLL_SECONDS) add(FIELD_SCREEN_POLL_SECONDS)
                if (notifyPollMinutes !in ALLOWED_NOTIFY_POLL_MINUTES) add(FIELD_NOTIFY_POLL_MINUTES)
            }
        if (fields.isNotEmpty()) throw SettingsException(fields)
    }
}
