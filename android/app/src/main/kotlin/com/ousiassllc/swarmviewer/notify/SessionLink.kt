package com.ousiassllc.swarmviewer.notify

import android.content.Context
import android.content.Intent
import com.ousiassllc.swarmviewer.MainActivity

private const val ACTION_OPEN_SESSION = "com.ousiassllc.swarmviewer.action.OPEN_SESSION"
private const val EXTRA_CONNECTION_ID = "connection_id"
private const val EXTRA_SESSION = "session"
private const val MAX_CONNECTION_ID_LENGTH = 64
private const val MAX_SESSION_LENGTH = 256

/** 通知から開く接続×セッション。 */
data class SessionTarget(
    val connectionId: String,
    val session: String,
) {
    companion object {
        /** 外部から来た値を検証する。どちらも空でなく、長さが上限以内のときだけ対象を返す（それ以外は null）。 */
        fun validated(
            connectionId: String?,
            session: String?,
        ): SessionTarget? =
            if (connectionId.isNullOrEmpty() ||
                connectionId.length > MAX_CONNECTION_ID_LENGTH ||
                session.isNullOrEmpty() ||
                session.length > MAX_SESSION_LENGTH
            ) {
                null
            } else {
                SessionTarget(connectionId, session)
            }
    }
}

/**
 * 通知のタップで、該当接続・セッションの画面を開く Intent（機能要件 §12.1 FR-AND-5）。Intent に載せるのは接続 ID とセッション ID だけ
 * （token・URL・ペイン本文は載せない）。同じ Intent と見なされて extras が上書きされないよう、組ごとに identifier を変える。
 * `MainActivity` は外部のアプリからも起動できるため、受け取る側は [parse] で信頼できない入力として検証する。
 */
object SessionLink {
    fun intent(
        context: Context,
        target: SessionTarget,
    ): Intent =
        Intent(context, MainActivity::class.java)
            .setAction(ACTION_OPEN_SESSION)
            .setIdentifier("${target.connectionId}/${target.session}")
            .putExtra(EXTRA_CONNECTION_ID, target.connectionId)
            .putExtra(EXTRA_SESSION, target.session)

    /** [intent] が通知タップの Intent で、中身が妥当なら対象を返す。それ以外は null。 */
    fun parse(intent: Intent): SessionTarget? {
        if (intent.action != ACTION_OPEN_SESSION) return null
        return SessionTarget.validated(intent.getStringExtra(EXTRA_CONNECTION_ID), intent.getStringExtra(EXTRA_SESSION))
    }

    /** 処理した Intent から対象を消す（再生成で再び処理しない）。 */
    fun consume(intent: Intent) {
        intent.action = null
        intent.removeExtra(EXTRA_CONNECTION_ID)
        intent.removeExtra(EXTRA_SESSION)
    }
}
