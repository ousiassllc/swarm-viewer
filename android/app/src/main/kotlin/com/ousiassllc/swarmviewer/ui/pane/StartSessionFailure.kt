package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import com.ousiassllc.swarmviewer.ui.rejectionMessage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.update

// [StartSessionViewModel] の失敗の扱い。状態（[StartSessionUiState]）への反映と、画面に出す固定文言への変換をまとめる。

/** 失敗の固定文言。401・403 は再ペアリングの案内を出す（[StartSessionUiState.needsRepair] を立てる）。 */
internal fun MutableStateFlow<StartSessionUiState>.failureText(e: Exception): String {
    if (e is FetchException) {
        update { it.copy(needsRepair = it.needsRepair || e.requiresRepair) }
        return fetchErrorMessage(e)
    }
    return rejectionMessage(e) ?: UNEXPECTED_ERROR_MESSAGE
}

/** 失敗を固定文言にして [StartSessionUiState.message] に出す。 */
internal fun MutableStateFlow<StartSessionUiState>.showFailure(e: Exception) {
    val text = failureText(e)
    update { it.copy(message = text) }
}

/** [block] を実行する。予期しない失敗は固定文言を出して null を返す（取り消しは伝える）。 */
internal suspend fun <T> MutableStateFlow<StartSessionUiState>.attempt(block: suspend () -> T): T? =
    try {
        block()
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        update { it.copy(message = UNEXPECTED_ERROR_MESSAGE) }
        null
    }
