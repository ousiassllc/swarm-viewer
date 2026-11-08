package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.DEFAULT_TAIL_LINES
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.MAX_TAIL_LINES
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneTail
import com.ousiassllc.swarmviewer.service.PaneTarget
import kotlinx.coroutines.channels.Channel

/** 確認ダイアログの対象。確認を経ずに中断・終了を送る経路は無い。 */
enum class PaneConfirm { Escape, Close }

/**
 * ペイン詳細の状態。[pane] は直近の `report` の `panes[]`、[tail]・[card] は直近の取得。取得に失敗しても直前の表示を残し、
 * [failure]（固定文言）をバナーで出す。[busy] の間は同じペインの全ボタンを無効にする。[notice] は 1 回だけ出す通知（トースト）。
 */
data class PaneDetailUiState(
    val target: PaneTarget? = null,
    val connectionName: String = "",
    val kind: ConnectionKind = ConnectionKind.View,
    val pane: Pane? = null,
    val tail: PaneTail? = null,
    val lines: Int = DEFAULT_TAIL_LINES,
    val card: AskCard? = null,
    val loaded: Boolean = false,
    val failure: String? = null,
    val needsRepair: Boolean = false,
    val busy: Boolean = false,
    val notice: PaneNotice? = null,
    val sendText: String = "",
    val customText: String = "",
    val customOpen: Boolean = false,
    val pendingConfirm: PaneConfirm? = null,
    val retryClose: Boolean = false,
) {
    /** 描画する部品（見るだけの接続では操作の部品が 1 つも無い）。 */
    val controls: PaneControls get() = paneControls(kind, pane?.state, card)

    val canShowMore: Boolean get() = lines < MAX_TAIL_LINES
}

/** 操作結果の通知（トースト相当）。[error] は失敗・未確定（色だけでなく、アラートの種類とアイコンでも伝える）。 */
data class PaneNotice(
    val message: String,
    val error: Boolean = false,
)

/** 「もっと見る」で増やす行数（30 → 100 → 200）。 */
internal val LINE_STEPS = listOf(DEFAULT_TAIL_LINES, 100, MAX_TAIL_LINES)

/** 未消費のイベントをすべて捨てる（画面を離れている間に送られたイベントを、次の画面で再生しない）。 */
internal fun <T> Channel<T>.drain() {
    while (tryReceive().isSuccess) continue
}

/** 401・403（再ペアリングが必要）か。 */
internal val FetchException.requiresRepair: Boolean get() = kind == ErrKind.Unauthorized || kind == ErrKind.ForbiddenScope
