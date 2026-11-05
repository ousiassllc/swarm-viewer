package com.ousiassllc.swarmviewer.notify

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.core.content.edit

private const val PREFS_NAME = "notification_permission"
private const val KEY_ASKED = "asked"

/**
 * 通知の権限（Android 13 以降の実行時権限 `POST_NOTIFICATIONS`。API 29〜32 は実行時の許可なしで通知できる）。
 * [isAllowed] は OS が通知を出せる状態か（権限の拒否・システム設定での無効化の両方を含む）。
 */
class NotificationPermission(
    context: Context,
) {
    private val context = context.applicationContext
    private val prefs = this.context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    /** OS が通知を出せる状態か。 */
    fun isAllowed(): Boolean = NotificationManagerCompat.from(context).areNotificationsEnabled()

    /** 実行時の権限要求が要る（Android 13 以降で、まだ許可されていない）か。 */
    fun needsRequest(): Boolean = needsRuntimeRequest(context)

    /**
     * 初回起動時の要求をすべきか。通知がオンで、権限の要求が要り、まだ一度も要求していないときだけ true を返し、要求済みに記録する。
     * 拒否された後は、利用者が設定画面の案内（[appNotificationSettings]）から変える。
     */
    fun takeInitialRequest(notificationsEnabled: Boolean): Boolean {
        if (!notificationsEnabled || !needsRequest() || prefs.getBoolean(KEY_ASKED, false)) return false
        prefs.edit { putBoolean(KEY_ASKED, true) }
        return true
    }

    companion object {
        /** Android 13 以降で `POST_NOTIFICATIONS` が未許可か。 */
        fun needsRuntimeRequest(context: Context): Boolean =
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED

        /** アプリの通知設定（チャンネルごとの調整もここ）を開く Intent。 */
        fun appNotificationSettings(context: Context): Intent =
            Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
                .putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
}
