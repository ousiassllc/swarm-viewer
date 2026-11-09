package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.PaneState

/**
 * ペイン詳細で**何を描画するか**の判定（機能要件 §4.7）。画面はこの値だけを見て部品を出すので、
 * 見るだけの接続で操作 UI が出ないことは、この関数のテストで示せる（Compose の UI テストを持たないため）。
 *
 *  - 見るだけ（[ConnectionKind.View]）: 質問文と選択肢の**一覧**（[showCardList]）と、操作できる接続への案内（[showViewOnlyHint]）だけ。
 *    ボタン・送信欄・中断・終了は false
 *  - 操作（[ConnectionKind.Ops]）: 質問カードは選択肢のボタン（[showCardButtons]）。送信欄は常に出し、`idle`・`stalled` 以外は
 *    無効にして理由（[sendDisabledReason]）を 1 行出す。中断は `working`・`stalled`・`asking`・`idle`、終了は `idle`・`shell`、
 *    「＋ 始める」は `shell`（[showStart]。メインのペインカードも同じ判定を使う）
 */
data class PaneControls(
    val showCardList: Boolean = false,
    val showViewOnlyHint: Boolean = false,
    val showCardButtons: Boolean = false,
    val showCustomEntry: Boolean = false,
    val showSendField: Boolean = false,
    val sendDisabledReason: String? = null,
    val showEscape: Boolean = false,
    val showClose: Boolean = false,
    val showStart: Boolean = false,
) {
    /** 操作の部品が 1 つでも出るか（見るだけの接続では false）。 */
    val anyOperation: Boolean get() = showCardButtons || showCustomEntry || showSendField || showEscape || showClose || showStart

    /** 送信欄を使えるか（出ていて、無効の理由が無い）。 */
    val sendEnabled: Boolean get() = showSendField && sendDisabledReason == null
}

/** 送れない状態の理由（1 行）。`idle`・`stalled` は送れるので null。 */
fun sendDisabledReason(state: PaneState): String? =
    when (state) {
        PaneState.Working -> "作業中のため送れません。終わるまで待つか、中断してください"
        PaneState.Asking -> "質問に答えてから送れます"
        PaneState.Shell -> "エージェントが動いていません。「始める」から起動してください"
        PaneState.Unknown -> "このペインには送れません"
        PaneState.Idle, PaneState.Stalled -> null
    }

/**
 * [kind]・[state]・取得済みの質問カード [card] から、描画する部品を決める。[state] が null（report にペインが無い・未取得）なら
 * 操作の部品は出さない。質問カードは `asking: true` のときだけ。
 */
fun paneControls(
    kind: ConnectionKind,
    state: PaneState?,
    card: AskCard?,
): PaneControls {
    val asking = state == PaneState.Asking && card?.asking == true
    if (kind != ConnectionKind.Ops) return PaneControls(showCardList = asking, showViewOnlyHint = asking)
    if (state == null) return PaneControls()
    return PaneControls(
        showCardList = asking,
        showCardButtons = asking,
        showCustomEntry = asking && card?.allowsCustom == true,
        showSendField = true,
        sendDisabledReason = sendDisabledReason(state),
        showEscape = state in ESCAPABLE,
        showClose = state in CLOSABLE,
        showStart = state == PaneState.Shell,
    )
}

private val ESCAPABLE = setOf(PaneState.Working, PaneState.Stalled, PaneState.Asking, PaneState.Idle)
private val CLOSABLE = setOf(PaneState.Idle, PaneState.Shell)
