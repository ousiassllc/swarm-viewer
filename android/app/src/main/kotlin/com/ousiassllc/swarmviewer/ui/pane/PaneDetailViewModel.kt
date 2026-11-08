package com.ousiassllc.swarmviewer.ui.pane

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.service.ConnectionGoneException
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.PaneService
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.service.SnapshotService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.closeToast
import com.ousiassllc.swarmviewer.ui.escapeToast
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import com.ousiassllc.swarmviewer.ui.pickToast
import com.ousiassllc.swarmviewer.ui.rejectionMessage
import com.ousiassllc.swarmviewer.ui.sendToast
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.getAndUpdate
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/** 開いている間の自動更新の間隔（機能要件 §4.7）。 */
const val PANE_POLL_MILLIS = 5_000L

/**
 * ペイン詳細（機能要件 §4.7）の ViewModel。開く（[open]）と、画面が見えている間（[onStart]〜[onStop]）だけ [PANE_POLL_MILLIS] ごとに
 * `report` → `pane_tail` →（`asking` のときだけ）`ask_card` を取り直す。401・403 は自動更新を止める。
 * 操作（選択・送信・中断・終了）は同時に 1 つだけ（[busy]）。サービス側にも同じペインの排他がある。
 */
class PaneDetailViewModel(
    private val snapshots: SnapshotService,
    private val panes: PaneService,
    private val connections: ConnectionService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(PaneDetailUiState())
    val state: StateFlow<PaneDetailUiState> = mutableState.asStateFlow()

    private val mutableLeave = Channel<Unit>(Channel.BUFFERED)

    /** 画面を閉じてメインへ戻るとき（ペインを終了できた・接続が無くなった）。1 回だけ届く。 */
    val leave: Flow<Unit> = mutableLeave.receiveAsFlow()

    private var started = false
    private var pollJob: Job? = null
    private var refreshJob: Job? = null
    private var leaving = false
    private var operationJob: Job? = null

    /** [target] のペインを開く。同じペインが表示中なら何もしない（構成変更）。前の画面で未消費の [leave] は捨てる。 */
    fun open(target: PaneTarget) {
        if (mutableState.value.target == target && !leaving) return
        reset()
        mutableState.value = PaneDetailUiState(target = target)
        if (started) startPolling()
    }

    /** 画面を離れたときの後始末（ナビゲーションが呼ぶ）。取得・実行中の操作を止め、状態と未消費の [leave] を捨てる。 */
    fun reset() {
        stopJobs()
        operationJob?.cancel()
        operationJob = null
        leaving = false
        mutableLeave.drain()
        mutableState.value = PaneDetailUiState()
    }

    /** 画面が見える。即時に取得して 5 秒ごとの自動更新を始める。 */
    fun onStart() {
        started = true
        startPolling()
    }

    /** 画面が見えなくなった。自動更新と実行中の取得を止める。 */
    fun onStop() {
        started = false
        stopJobs()
    }

    /** 手動の更新。 */
    fun refresh() {
        if (started && !mutableState.value.needsRepair) ensureRefresh()
    }

    /** 「もっと見る」: 30 → 100 → 200 行（最大 200）。 */
    fun showMore() {
        val next = LINE_STEPS.firstOrNull { it > mutableState.value.lines } ?: return
        mutableState.update { it.copy(lines = next) }
        refresh()
    }

    fun onSendTextChange(text: String) = mutableState.update { it.copy(sendText = text) }

    fun onCustomTextChange(text: String) = mutableState.update { it.copy(customText = text) }

    fun openCustom(open: Boolean) = mutableState.update { it.copy(customOpen = open) }

    fun noticeShown() = mutableState.update { it.copy(notice = null) }

    /** 質問の選択肢を押した。確認なしですぐ `ask_pick` を送る。 */
    fun pick(n: Int) {
        val target = mutableState.value.target ?: return
        if (!mutableState.value.controls.showCardButtons) return // 見るだけの接続・質問が無いときは何もしない
        operate {
            val result = panes.pickOption(target, n)
            closeCardIfAnswered(result.ok && result.cardClosed)
            toast(pickToast(result), error = !(result.ok && result.cardClosed))
        }
    }

    /** 「その他」の自由入力を送る。 */
    fun pickCustom() {
        val target = mutableState.value.target ?: return
        if (!mutableState.value.controls.showCustomEntry) return
        val text = mutableState.value.customText
        operate {
            val result = panes.pickText(target, text)
            if (result.ok && result.cardClosed) mutableState.update { it.copy(customText = "", customOpen = false) }
            closeCardIfAnswered(result.ok && result.cardClosed)
            toast(pickToast(result), error = !(result.ok && result.cardClosed))
        }
    }

    /** 指示を送る。受理されたら入力欄を空にする。 */
    fun send() {
        val target = mutableState.value.target ?: return
        if (!mutableState.value.controls.sendEnabled) return // 見るだけの接続・送れない状態では何もしない
        val text = mutableState.value.sendText
        operate {
            val result = panes.send(target, text)
            if (result.accepted) mutableState.update { it.copy(sendText = "") }
            toast(sendToast(result), error = !result.accepted)
        }
    }

    /** 「中断」を押した。確認ダイアログを出すだけで、送らない。 */
    fun requestEscape() = mutableState.update { if (it.controls.showEscape) it.copy(pendingConfirm = PaneConfirm.Escape) else it }

    /** 「終了」（または「もう一度試す」）を押した。確認ダイアログを出すだけで、送らない。 */
    fun requestClose() =
        mutableState.update {
            if (it.controls.showClose) it.copy(pendingConfirm = PaneConfirm.Close, retryClose = false) else it
        }

    fun dismissConfirm() = mutableState.update { it.copy(pendingConfirm = null) }

    /** 確認ダイアログの確定。`send_escape` に `confirm: true` を付けるのは、この経路だけ。 */
    fun confirmPending() {
        val target = mutableState.value.target ?: return
        val pending = mutableState.value.pendingConfirm ?: return
        mutableState.update { it.copy(pendingConfirm = null) }
        if (!mutableState.value.controls.let { if (pending == PaneConfirm.Escape) it.showEscape else it.showClose }) return
        when (pending) {
            PaneConfirm.Escape ->
                operate {
                    val result = panes.escape(target, confirm = true)
                    toast(escapeToast(result), error = !(result.ok && result.interrupted))
                }
            PaneConfirm.Close ->
                operate {
                    val result = panes.close(target)
                    if (result.ok) finish()
                    toast(closeToast(result), error = !result.ok)
                }
        }
    }

    // 答えが届いて質問が閉じたら、取り直すまで古い選択肢のボタンを出さない（再び押せるようにならない）。
    private fun closeCardIfAnswered(closed: Boolean) {
        if (closed) mutableState.update { it.copy(card = null) }
    }

    // 通知を出す。「もう一度試す」（[PaneDetailUiState.retryClose]）は exit_timeout の失敗だけなので、ほかの通知では必ず落とす。
    private fun toast(
        message: String,
        error: Boolean = false,
    ) = mutableState.update { it.copy(notice = PaneNotice(message, error), retryClose = false) }

    // 操作 1 つ。同じペインの操作中（busy）は何もしない。終わったら表示を取り直す（質問カードが閉じたことなどを反映）。
    private fun operate(block: suspend () -> Unit) {
        if (mutableState.getAndUpdate { it.copy(busy = true) }.busy) return
        operationJob =
            viewModelScope.launch {
                try {
                    block()
                } catch (e: CancellationException) {
                    throw e
                } catch (e: FetchException) {
                    operationFailed(e)
                } catch (e: ConnectionGoneException) {
                    finish()
                } catch (e: Exception) {
                    toast(rejectionMessage(e) ?: UNEXPECTED_ERROR_MESSAGE, error = true) // 例外の本文は使わない
                } finally {
                    mutableState.update { it.copy(busy = false) }
                }
                if (started && !leaving) ensureRefresh(restart = true)
            }
    }

    private fun finish() {
        leaving = true
        stopJobs()
        mutableLeave.trySend(Unit)
    }

    private fun operationFailed(e: FetchException) =
        mutableState.update {
            it.copy(
                notice = PaneNotice(fetchErrorMessage(e), error = true),
                needsRepair = it.needsRepair || e.requiresRepair,
                retryClose = e.kind == ErrKind.Operation && e.code == "exit_timeout",
            )
        }

    private fun stopJobs() {
        pollJob?.cancel()
        refreshJob?.cancel()
        pollJob = null
        refreshJob = null
    }

    private fun startPolling() {
        if (mutableState.value.target == null) return
        pollJob?.cancel()
        pollJob =
            viewModelScope.launch {
                loadConnection()
                while (isActive && !mutableState.value.needsRepair) {
                    ensureRefresh().join()
                    delay(PANE_POLL_MILLIS)
                }
            }
    }

    /** 実行中の取得があればそれを返し、無ければ始める（同時に 1 本だけ）。 */
    private fun ensureRefresh(restart: Boolean = false): Job {
        // [restart]（操作のあと）は、操作より前に始まった取得（答える前の ask_card を読んでいるかもしれない）を取り消して取り直す。
        if (restart) refreshJob?.cancel() else refreshJob?.takeIf { it.isActive }?.let { return it }
        return viewModelScope.launch { fetch() }.also { refreshJob = it }
    }

    // 接続の名前・種類・再ペアリングの要否。種類は URL の path から（DB に列を持たない）。
    private suspend fun loadConnection() {
        val target = mutableState.value.target ?: return
        val item =
            try {
                connections.list().firstOrNull { it.id == target.connectionId }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                mutableState.update { it.copy(failure = UNEXPECTED_ERROR_MESSAGE) }
                return
            }
        if (item == null) {
            finish()
            return
        }
        mutableState.update { it.copy(connectionName = item.name, kind = item.kind, needsRepair = item.needsRepair) }
    }

    // report → pane_tail → ask_card（asking のときだけ）。段階ごとに表示を更新し、失敗したら直前の表示を残して固定文言を出す。
    private suspend fun fetch() {
        val target = mutableState.value.target ?: return
        val lines = mutableState.value.lines
        try {
            val report = snapshots.refresh(target.connectionId, target.session).report
            val pane = report.panes.firstOrNull { it.pos == target.pos }
            if (pane == null) {
                mutableState.update { it.copy(pane = null, failure = operationErrorMessage("pane_not_found"), loaded = true) }
                return
            }
            mutableState.update { if (it.target == target) it.copy(pane = pane) else it }
            val tail = panes.tail(target, lines)
            val card = if (pane.state == PaneState.Asking) panes.askCard(target) else null
            mutableState.update { if (it.target == target) it.copy(tail = tail, card = card, loaded = true, failure = null) else it }
        } catch (e: CancellationException) {
            throw e
        } catch (e: FetchException) {
            if (e.requiresRepair) connections.markNeedsRepair(target.connectionId)
            mutableState.update { it.copy(failure = fetchErrorMessage(e), needsRepair = it.needsRepair || e.requiresRepair, loaded = true) }
        } catch (e: ConnectionGoneException) {
            finish()
        } catch (e: Exception) {
            mutableState.update { it.copy(failure = UNEXPECTED_ERROR_MESSAGE, loaded = true) }
        }
    }
}
