package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.domain.PaneState

/**
 * 通知する 1 件。通知に使う項目は接続の表示名・セッション・ペインの位置・状態だけ（token・完全な URL・ペイン本文は含めない）。
 * [paneId] は通知の同一性（同じ組の再投稿は置き換わる）にだけ使い、画面には出さない。
 */
data class AttentionNotice(
    val connectionId: String,
    val connectionName: String,
    val session: String,
    val paneId: String,
    val pos: String,
    val state: PaneState,
)

/**
 * OS 通知の窓口。実装は [SystemNotifier]（Android の通知）。テストではフェイクに差し替える。
 * 通知の権限が無い・無効なときは何も出さない（呼び出し側は通知済みの記録の更新を続ける。機能要件 §12.5）。
 */
interface AttentionNotifier {
    /** `asking` / `stalled` への遷移の通知。 */
    fun attention(notice: AttentionNotice)

    /** 401・403 で再ペアリングが必要になった接続への通知（接続ごとに 1 回。呼び出し側が [RepairFlags] で抑える）。 */
    fun needsRepair(
        connectionId: String,
        connectionName: String,
    )
}
