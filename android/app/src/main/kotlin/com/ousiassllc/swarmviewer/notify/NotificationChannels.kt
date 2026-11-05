package com.ousiassllc.swarmviewer.notify

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import com.ousiassllc.swarmviewer.domain.PaneState

/**
 * 通知チャンネル。`asking`（答え待ち）と `stalled`（止まってるかも）は別のチャンネルにして、利用者が OS の設定で個別に調整できる。
 * [REPAIR] は 401・403 の再ペアリングの案内用。minSdk 29 のためチャンネルは常に使える（API 26 以降は必須）。
 */
object NotificationChannels {
    const val ASKING = "asking"
    const val STALLED = "stalled"
    const val REPAIR = "repair"

    /** [state] の通知を出すチャンネルの ID。通知対象（asking / stalled）以外は呼ばれない。 */
    fun idFor(state: PaneState): String = if (state == PaneState.Asking) ASKING else STALLED

    /** チャンネルを作る（作成済みなら何も変わらない）。アプリ起動時に呼ぶ。 */
    fun create(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java)
        manager.createNotificationChannels(
            listOf(
                NotificationChannel(ASKING, "答え待ち", NotificationManager.IMPORTANCE_HIGH).apply {
                    description = "ペインが答え待ちになったときの通知"
                },
                NotificationChannel(STALLED, "止まってるかも", NotificationManager.IMPORTANCE_HIGH).apply {
                    description = "ペインが止まってるかもになったときの通知"
                },
                NotificationChannel(REPAIR, "再ペアリング", NotificationManager.IMPORTANCE_DEFAULT).apply {
                    description = "認証できなくなった接続の再ペアリングを促す通知"
                },
            ),
        )
    }
}
