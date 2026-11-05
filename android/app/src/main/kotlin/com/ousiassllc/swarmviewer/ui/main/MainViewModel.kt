package com.ousiassllc.swarmviewer.ui.main

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.service.ConnectionGoneException
import com.ousiassllc.swarmviewer.service.ConnectionItem
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.service.SnapshotService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

private const val MILLIS_PER_SECOND = 1000L

/**
 * メイン画面（A2）の状態。表示するのは `report` の内容だけで、token・完全な URL は持たない。
 *  - [connections] が null の間は読み込み中。複数あるときだけ接続の切替を出す
 *  - [sessions] は許可セッション（`sessions_list` の順）。取得できていない間（[sessionsLoaded] が false）は空で、[session] は記憶したセッション
 *  - [snapshot] は最後に取得できた `report`。起動直後は保存済みの前回取得。取得に成功すると [live] になる
 *  - [failure] は直近の取得の失敗の文言。あるとき [snapshot] はグレーアウトして残す（機能要件 §10）
 *  - [needsRepair] は 401・403 など。ポーリングを止めて再ペアリングへ誘導する
 *  - [refreshing] はプルして更新の取得中だけ true（自動のポーリングでは点滅させない）
 */
data class MainUiState(
    val connections: List<ConnectionItem>? = null,
    val connectionId: String? = null,
    val sessions: List<String> = emptyList(),
    val sessionsLoaded: Boolean = false,
    val session: String? = null,
    val snapshot: Snapshot? = null,
    val live: Boolean = false,
    val failure: String? = null,
    val needsRepair: Boolean = false,
    val refreshing: Boolean = false,
) {
    /** 接続が 1 件も無い（読み込み済み）。 */
    val noConnections: Boolean get() = connections?.isEmpty() == true

    /** 表示中の内容がグレーアウトの対象（取得に失敗している、または再ペアリングが必要）。 */
    val stale: Boolean get() = snapshot != null && (failure != null || needsRepair)

    /** 「前回取得 HH:MM」を出す（保存済みの前回の内容、または失敗して直前の内容を残しているとき）。 */
    val showsPreviousFetch: Boolean get() = snapshot != null && (!live || stale)
}

/**
 * メイン画面の ViewModel。選択中の接続・セッションと取得済みの表示を持ち、構成変更（折りたたみ⇔展開）で失われない。
 *
 * フォアグラウンドのポーリング（機能要件 §12.1 FR-AND-8）: 画面が見えている間（[onStart]〜[onStop]）だけ、
 * 選択中セッションの `report` 1 本を設定の間隔で取得する。前の取得が終わるまで次を出さず（間隔は取得の完了から数える）、
 * [onStop] で取得ごと止め、[onStart] で即時に取得する。401・403 の接続は再ペアリングまで止める（[ConnectionService.markNeedsRepair]）。
 */
class MainViewModel(
    private val snapshots: SnapshotService,
    private val connections: ConnectionService,
    private val settings: SettingsService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(MainUiState())
    val state: StateFlow<MainUiState> = mutableState.asStateFlow()

    private var pollJob: Job? = null
    private var fetchJob: Job? = null
    private var started = false
    private var sessionsStale = true
    private var pendingOpen: Pair<String, String>? = null

    /** 画面が見える（ライフサイクルが STARTED 以上）。接続の一覧を読み直し、即時に取得してポーリングを始める。 */
    fun onStart() {
        started = true
        sessionsStale = true
        startPolling(reload = true)
    }

    /** 画面が見えなくなった。ポーリングと、実行中の取得を止める。 */
    fun onStop() {
        started = false
        stopJobs()
    }

    /** プルして更新。実行中の取得があればそれに相乗りする。再ペアリングが必要な間は何もしない。 */
    fun refresh() {
        if (!started || mutableState.value.needsRepair || mutableState.value.connectionId == null) return
        sessionsStale = true
        mutableState.update { it.copy(refreshing = true) }
        ensureFetch()
    }

    /** セッションのタブを選ぶ。接続ごとに記憶し、保存済みの前回取得を出して取得し直す。 */
    fun selectSession(id: String) {
        val current = mutableState.value
        val connectionId = current.connectionId ?: return
        if (id == current.session || (current.sessions.isNotEmpty() && id !in current.sessions)) return
        stopJobs()
        viewModelScope.launch {
            attempt { snapshots.remember(connectionId, id) }
            showSession(connectionId, id)
            if (started) startPolling(reload = false)
        }
    }

    /**
     * 通知のタップなどで、接続×セッションを開く。その接続を選択し、セッションを記憶して表示する。画面が見えるようになってから
     * （または、すでに見えていれば直ちに）読み直して適用する。存在しない接続は無視し、許可されていないセッションは先頭の許可セッションになる。
     */
    fun openSession(
        connectionId: String,
        session: String,
    ) {
        pendingOpen = connectionId to session
        if (started) startPolling(reload = true)
    }

    /** ヘッダーのセレクタで接続を切り替える。 */
    fun selectConnection(id: String) {
        if (id == mutableState.value.connectionId) return
        stopJobs()
        viewModelScope.launch {
            attempt { connections.select(id) }
            if (started) startPolling(reload = true) else reload()
        }
    }

    private fun stopJobs() {
        pollJob?.cancel()
        fetchJob?.cancel()
        pollJob = null
        fetchJob = null
        mutableState.update { it.copy(refreshing = false) }
    }

    private fun startPolling(reload: Boolean) {
        pollJob?.cancel()
        pollJob =
            viewModelScope.launch {
                if (reload) reload()
                while (isActive && !mutableState.value.needsRepair && mutableState.value.connectionId != null) {
                    ensureFetch().join()
                    if (mutableState.value.needsRepair) break
                    delay(settings.get().screenPollSeconds * MILLIS_PER_SECOND)
                }
            }
    }

    /** 実行中の取得があればそれを返し、無ければ始める（同時に 1 本だけ）。 */
    private fun ensureFetch(): Job {
        fetchJob?.takeIf { it.isActive }?.let { return it }
        return viewModelScope.launch { fetch() }.also { fetchJob = it }
    }

    /** 接続の一覧と選択を読み直す。選択中の接続が変わっていれば、その接続の記憶したセッションと前回取得を出す。 */
    private suspend fun reload() {
        val opened = pendingOpen?.also { pendingOpen = null }?.let { (connectionId, session) -> open(connectionId, session) } == true
        val items =
            try {
                connections.list()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                mutableState.update { it.copy(failure = UNEXPECTED_ERROR_MESSAGE) }
                return
            }
        val selected = items.firstOrNull { it.selected }
        val changed = opened || selected?.id != mutableState.value.connectionId
        mutableState.update { it.copy(connections = items, needsRepair = selected?.needsRepair == true) }
        if (!changed) return
        sessionsStale = true
        val id = selected?.id
        if (id == null) {
            mutableState.update {
                it.copy(connectionId = null, sessions = emptyList(), sessionsLoaded = false, session = null, snapshot = null, live = false)
            }
            return
        }
        mutableState.update { it.copy(connectionId = id, sessions = emptyList(), sessionsLoaded = false) }
        val last = attempt { snapshots.lastSession(id) }
        showSession(id, last)
    }

    /** 通知から開く接続を選択してセッションを記憶する。接続が無ければ false。 */
    private suspend fun open(
        connectionId: String,
        session: String,
    ): Boolean {
        if (attempt { connections.select(connectionId) } != true) return false
        attempt { snapshots.remember(connectionId, session) }
        return true
    }

    /** [session] を表示対象にし、保存済みの前回取得（あれば）を出す。 */
    private suspend fun showSession(
        connectionId: String,
        session: String?,
    ) {
        val cached = session?.let { attempt { snapshots.cached(connectionId, it) } }
        mutableState.update { it.copy(session = session, snapshot = cached, live = false, failure = null) }
    }

    private suspend fun fetch() {
        val connectionId = mutableState.value.connectionId ?: return
        try {
            if (sessionsStale) loadSessions(connectionId)
            val session = mutableState.value.session ?: return
            val snapshot = snapshots.refresh(connectionId, session)
            mutableState.update {
                if (it.connectionId == connectionId && it.session == session) {
                    it.copy(snapshot = snapshot, live = true, failure = null)
                } else {
                    it
                }
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: FetchException) {
            failed(connectionId, e)
        } catch (e: ConnectionGoneException) {
            reload()
        } catch (e: Exception) {
            mutableState.update { it.copy(failure = UNEXPECTED_ERROR_MESSAGE) } // 例外の本文（パス・URL を含みうる）は使わない
        } finally {
            mutableState.update { it.copy(refreshing = false) }
        }
    }

    /** `sessions_list` で許可セッションを取り直し、タブと表示対象（記憶したセッション、なければ先頭）を更新する。 */
    private suspend fun loadSessions(connectionId: String) {
        val allowed = snapshots.allowedSessions(connectionId)
        sessionsStale = false
        val session = pickSession(allowed, mutableState.value.session)
        mutableState.update { it.copy(sessions = allowed, sessionsLoaded = true) }
        if (session != mutableState.value.session) showSession(connectionId, session)
    }

    private fun failed(
        connectionId: String,
        e: FetchException,
    ) {
        when (e.kind) {
            ErrKind.Unauthorized, ErrKind.ForbiddenScope -> connections.markNeedsRepair(connectionId)
            ErrKind.SessionForbidden, ErrKind.SessionMissing -> sessionsStale = true // 次の取得で許可セッションを取り直す
            else -> Unit
        }
        val repair = e.kind == ErrKind.Unauthorized || e.kind == ErrKind.ForbiddenScope
        mutableState.update { it.copy(failure = fetchErrorMessage(e), needsRepair = it.needsRepair || repair) }
    }

    private suspend fun <T> attempt(block: suspend () -> T): T? =
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            mutableState.update { it.copy(failure = UNEXPECTED_ERROR_MESSAGE) }
            null
        }
}
