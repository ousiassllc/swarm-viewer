package com.ousiassllc.swarmviewer.notify

import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import com.ousiassllc.swarmviewer.MainActivity
import com.ousiassllc.swarmviewer.R

private const val PUBLIC_TEXT = "要対応があります"
private const val REPAIR_TEXT = "再ペアリングが必要です"

/**
 * [AttentionNotifier] の Android 実装。通知の本文は接続の表示名・セッション・ペインの位置・状態ラベルだけで、
 * token・URL・ペイン本文（`title`・`activity`・`note`・`attention.detail`）を含めない。
 * ロック画面では内容を伏せ（`VISIBILITY_PRIVATE`）、公開用には「要対応があります」だけを出す。
 */
class SystemNotifier(
    context: Context,
) : AttentionNotifier {
    private val context = context.applicationContext
    private val manager = NotificationManagerCompat.from(this.context)

    init {
        NotificationChannels.create(this.context)
    }

    override fun attention(notice: AttentionNotice) {
        val text = listOf(notice.session, notice.pos, notice.state.label).filter { it.isNotEmpty() }.joinToString(" · ")
        val tap =
            PendingIntent.getActivity(
                context,
                0,
                SessionLink.intent(context, SessionTarget(notice.connectionId, notice.session)),
                PENDING_FLAGS,
            )
        val id = listOf(notice.connectionId, notice.session, notice.paneId, notice.state.id).hashCode()
        post(id, build(NotificationChannels.idFor(notice.state), notice.connectionName, text, tap))
    }

    override fun needsRepair(
        connectionId: String,
        connectionName: String,
    ) {
        val tap = PendingIntent.getActivity(context, 0, Intent(context, MainActivity::class.java), PENDING_FLAGS)
        post(listOf("repair", connectionId).hashCode(), build(NotificationChannels.REPAIR, connectionName, REPAIR_TEXT, tap))
    }

    private fun build(
        channel: String,
        title: String,
        text: String,
        tap: PendingIntent,
    ) = NotificationCompat
        .Builder(context, channel)
        .setSmallIcon(R.drawable.ic_notification)
        .setContentTitle(title)
        .setContentText(text)
        .setContentIntent(tap)
        .setAutoCancel(true)
        .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
        .setPublicVersion(
            NotificationCompat
                .Builder(context, channel)
                .setSmallIcon(R.drawable.ic_notification)
                .setContentText(PUBLIC_TEXT)
                .build(),
        ).build()

    // 権限が無い（Android 13 以降の POST_NOTIFICATIONS が未許可）・通知が無効なときは何も出さない。areNotificationsEnabled が両方を確かめる。
    @SuppressLint("MissingPermission")
    private fun post(
        id: Int,
        notification: android.app.Notification,
    ) {
        if (manager.areNotificationsEnabled()) manager.notify(id, notification)
    }

    private companion object {
        /** `FLAG_IMMUTABLE`: 他のアプリが Intent を書き換えられないようにする。identifier が組ごとに違うので、extras は組ごとに保たれる。 */
        const val PENDING_FLAGS = PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
    }
}
