package com.ousiassllc.swarmviewer.ui.pairing

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PairingException
import com.ousiassllc.swarmviewer.service.DraftExpiredException
import com.ousiassllc.swarmviewer.service.PairingDraft
import com.ousiassllc.swarmviewer.service.PairingSaveException
import com.ousiassllc.swarmviewer.service.PairingService
import com.ousiassllc.swarmviewer.ui.DRAFT_EXPIRED_MESSAGE
import com.ousiassllc.swarmviewer.ui.SAVE_FAILED_MESSAGE
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import com.ousiassllc.swarmviewer.ui.pairingErrorMessage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * ペアリング画面の状態。[input] は貼り付け欄の中身（token を含みうるため、画面では伏せ字にし、確定・失敗のどちらでもすぐ空にする。
 * 保存もログ出力もしない）。[draft] は確認画面の表示用で、token を持たない。[message] は固定文言の失敗表示。
 */
data class PairingUiState(
    val input: String = "",
    val draft: PairingDraft? = null,
    val message: String? = null,
    val busy: Boolean = false,
)

/**
 * 貼り付け・ディープリンク・QR の 3 経路が同じ [receive] に入り、同じ確認画面（[PairingUiState.draft]）に入る
 * （機能要件 §12.3）。確定（[confirm]）は疎通確認に成功したときだけ保存する。
 */
class PairingViewModel(
    private val service: PairingService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(PairingUiState())
    val state: StateFlow<PairingUiState> = mutableState.asStateFlow()

    private val mutableSaved = Channel<Unit>(Channel.BUFFERED)

    /** 接続を保存できたとき（1 回だけ届く）。 */
    val saved: Flow<Unit> = mutableSaved.receiveAsFlow()

    private var job: Job? = null

    fun onInputChange(text: String) = mutableState.update { it.copy(input = text) }

    /** 貼り付け欄の内容を解析して確認画面へ進める。欄はすぐ空にする。 */
    fun submitInput() {
        val raw = mutableState.value.input
        if (raw.isBlank()) return
        receive(raw)
    }

    /**
     * 接続文字列（貼り付け・ディープリンク・QR のどれでも）を解析して確認画面へ進める。
     * 古い下書き・実行中の確定は破棄する。[raw] は保存・ログ出力せず、解析後に捨てる（NFR-SEC-7）。
     */
    fun receive(raw: String) {
        job?.cancel()
        mutableState.update { PairingUiState(busy = true) } // 入力欄・古い下書き・古い失敗を消す
        job =
            viewModelScope.launch {
                val outcome = runCatching { service.parse(raw) }
                outcome.exceptionOrNull()?.rethrowCancellation()
                mutableState.value =
                    outcome.fold(
                        onSuccess = { PairingUiState(draft = it) },
                        onFailure = { PairingUiState(message = messageOf(it)) },
                    )
            }
    }

    /** 確認画面の「接続」。疎通確認に成功したときだけ保存し、[saved] を送る。失敗したら下書きを残して再試行できる。 */
    fun confirm() {
        val draft = mutableState.value.draft ?: return
        if (mutableState.value.busy) return
        mutableState.update { it.copy(busy = true, message = null) }
        job =
            viewModelScope.launch {
                val outcome = runCatching { service.confirm(draft.id) }
                outcome.exceptionOrNull()?.rethrowCancellation()
                outcome.fold(
                    onSuccess = {
                        mutableState.value = PairingUiState()
                        mutableSaved.send(Unit)
                    },
                    onFailure = { e ->
                        // 期限切れ・置換された下書きは、確認画面を閉じて入力からやり直す。
                        val keep = if (e is DraftExpiredException) null else draft
                        mutableState.value = PairingUiState(draft = keep, message = messageOf(e))
                    },
                )
            }
    }

    /** 確認画面・画面を閉じるとき。下書き（token）を捨て、実行中の処理を止める。 */
    fun cancel() {
        job?.cancel()
        mutableState.value.draft?.let { service.discard(it.id) }
        mutableState.value = PairingUiState()
    }

    override fun onCleared() = cancel()

    private fun messageOf(e: Throwable): String =
        when (e) {
            is PairingException -> pairingErrorMessage(e)
            is FetchException -> fetchErrorMessage(e)
            is DraftExpiredException -> DRAFT_EXPIRED_MESSAGE
            is PairingSaveException -> SAVE_FAILED_MESSAGE
            else -> UNEXPECTED_ERROR_MESSAGE // 例外の本文（token・URL を含みうる）は使わない
        }

    private fun Throwable.rethrowCancellation() {
        if (this is CancellationException) throw this
    }
}
