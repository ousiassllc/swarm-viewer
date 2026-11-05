package com.ousiassllc.swarmviewer.ui.connections

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.service.ConnectionItem
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * 接続管理画面（S1）の状態。[connections] は null の間は読み込み中。[pendingDelete] は削除の確認ダイアログの対象。
 * 表示するのは表示名と URL の host のみ（token・完全な URL は持たない）。
 */
data class ConnectionsUiState(
    val connections: List<ConnectionItem>? = null,
    val pendingDelete: ConnectionItem? = null,
    val message: String? = null,
)

class ConnectionsViewModel(
    private val service: ConnectionService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(ConnectionsUiState())
    val state: StateFlow<ConnectionsUiState> = mutableState.asStateFlow()

    init {
        refresh()
        // 401・403 の記録が変わったら一覧の表示を更新する（初期値の通知は refresh と重なるので読み飛ばす）。
        viewModelScope.launch { service.needsRepair.drop(1).collect { reload() } }
    }

    /** 一覧を読み直す（画面を開いたとき・ペアリング後）。 */
    fun refresh() {
        viewModelScope.launch { reload() }
    }

    /** 接続を切り替える。 */
    fun select(id: String) {
        mutableState.update { it.copy(message = null) }
        viewModelScope.launch {
            attempt { service.select(id) }
            reload()
        }
    }

    /** 削除の確認ダイアログを出す。 */
    fun requestDelete(item: ConnectionItem) = mutableState.update { it.copy(pendingDelete = item) }

    fun dismissDelete() = mutableState.update { it.copy(pendingDelete = null) }

    /** 確認ダイアログの「削除」。token・スナップショット・通知済み状態が消える。失敗したら接続は残る。 */
    fun confirmDelete() {
        val target = mutableState.value.pendingDelete ?: return
        mutableState.update { it.copy(pendingDelete = null, message = null) }
        viewModelScope.launch {
            attempt { service.delete(target.id) }
            reload()
        }
    }

    private suspend fun reload() {
        attempt { mutableState.update { it.copy(connections = service.list()) } }
    }

    private suspend fun attempt(block: suspend () -> Unit) {
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // 例外の本文（パス・鍵の情報を含みうる）は使わない。
            mutableState.update { it.copy(message = UNEXPECTED_ERROR_MESSAGE) }
        }
    }
}
