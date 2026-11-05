package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.secret.TokenStore
import com.ousiassllc.swarmviewer.data.store.ConnectionRepository
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.withContext

/**
 * 接続の一覧の 1 行。token・完全な URL を持たない（画面に出すのは表示名と URL の host のみ。NFR-SEC-11）。
 * [needsRepair] は再ペアリングが必要（401・403 を受けた、または token が無い・復号できない）。
 */
data class ConnectionItem(
    val id: String,
    val name: String,
    val host: String,
    val selected: Boolean,
    val needsRepair: Boolean,
)

/**
 * 接続の一覧・切替・削除と、再ペアリングが必要な接続の管理（機能要件 §4.2、FR-PAIR-3・4）。
 * 削除は token → 接続の行の順（[ConnectionRepository.delete]）で、スナップショット・通知済み状態は外部キーで連鎖削除される。
 *
 * 画面の取得で 401・403 を受けた接続（[markNeedsRepair]）は DB に状態を持たず、メモリ上の集合だけで保持する（API 仕様 §1.5）。
 * アプリの再起動後は、初回の取得で再度判定される。バックグラウンドの確認が 401・403 と判定した接続は [flags] にも残り（プロセスをまたぐ）、
 * 再ペアリングか削除まで一覧で「再ペアリングが必要」になる。
 */
class ConnectionService(
    private val store: ViewerStore,
    private val tokens: TokenStore,
    private val io: CoroutineDispatcher = Dispatchers.IO,
    private val flags: RepairFlags = InMemoryRepairFlags(),
) {
    private val repository = ConnectionRepository(store, tokens)
    private val repairIds = MutableStateFlow<Set<String>>(emptySet())

    /** 401・403 で再ペアリングが必要になった接続 ID。 */
    val needsRepair: StateFlow<Set<String>> = repairIds.asStateFlow()

    /**
     * 保存済みの接続を作成時刻の昇順で返す。選択中は `settings.last_connection_id`（未設定・存在しないときは先頭）。
     * token が無い・復号できない接続は [ConnectionItem.needsRepair]（Keystore の鍵の消失など。ER §7.3）。
     */
    suspend fun list(): List<ConnectionItem> {
        val connections = store.connections()
        val selectedId = selectedId(connections)
        val flagged = repairIds.value
        return withContext(io) {
            connections.map { it.toItem(selectedId, it.id in flagged || flags.isFlagged(it.id) || tokens.get(it.id) == null) }
        }
    }

    /** [id] の接続を選択（切替）する。無い接続は false。 */
    suspend fun select(id: String): Boolean {
        if (store.connection(id) == null) return false
        store.setLastConnection(id)
        return true
    }

    /**
     * 接続を削除する（確認は呼び出し側）。token・スナップショット・通知済み状態が消える。
     * token の削除に失敗したら接続は残したまま例外を伝える（再試行できる）。無い id は何もしない。
     */
    suspend fun delete(id: String) {
        withContext(io) { repository.delete(id) }
        markRepaired(id)
    }

    /** 401・403 を受けた接続を、再ペアリングが必要として記録する。 */
    fun markNeedsRepair(id: String) = repairIds.update { it + id }

    /** 再ペアリングに成功した接続（または削除した接続）の記録を外す。バックグラウンドの確認が付けた記録（[flags]）も外す。 */
    fun markRepaired(id: String) {
        repairIds.update { it - id }
        flags.clear(id)
    }

    private suspend fun selectedId(connections: List<Connection>): String? {
        val last = store.settings().lastConnectionId
        return connections.firstOrNull { it.id == last }?.id ?: connections.firstOrNull()?.id
    }
}

private fun Connection.toItem(
    selectedId: String?,
    needsRepair: Boolean,
) = ConnectionItem(id = id, name = name, host = host, selected = id == selectedId, needsRepair = needsRepair)
