package com.ousiassllc.swarmviewer.ui.pane

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.DirEntry
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.START_AGENTS
import com.ousiassllc.swarmviewer.domain.normalizeTitle
import com.ousiassllc.swarmviewer.domain.recentFolders
import com.ousiassllc.swarmviewer.service.ConnectionGoneException
import com.ousiassllc.swarmviewer.service.PaneService
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.service.SnapshotService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import com.ousiassllc.swarmviewer.ui.rejectionMessage
import com.ousiassllc.swarmviewer.ui.startToast
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.getAndUpdate
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** フォルダピッカーの 1 階層。[path] が null のときは `allowed_roots`（根の一覧）。 */
data class FolderListing(
    val path: String? = null,
    val entries: List<DirEntry> = emptyList(),
    val truncated: Boolean = false,
)

/**
 * 新しいセッション（`pane_start`）の画面の状態（機能要件 §4.7）。手順: エージェント → フォルダ → タイトル → 確認。
 * [viewOnly] の接続では何も選べず、操作の部品を描画しない（この画面に入る導線も出ない）。
 * [folder] は選択済みのフォルダ（`dir_list` の `path`、または最近使ったフォルダ）。[listing] はピッカーの現在の階層。
 */
data class StartSessionUiState(
    val target: PaneTarget? = null,
    val viewOnly: Boolean = false,
    val agent: String? = null,
    val listing: FolderListing? = null,
    val listingLoading: Boolean = false,
    val canGoUp: Boolean = false,
    val folder: String? = null,
    val recent: List<String> = emptyList(),
    val title: String = "",
    val confirming: Boolean = false,
    val busy: Boolean = false,
    val message: String? = null,
    val needsRepair: Boolean = false,
) {
    /** 確認へ進める: エージェント・フォルダ・タイトルが揃っている（タイトルは検査済み）。 */
    val canConfirm: Boolean get() = !viewOnly && agent != null && folder != null && normalizeTitle(title) != null

    /** 「このフォルダを選ぶ」を出せる階層か（根の一覧では選べない）。 */
    val canChooseListed: Boolean get() = listing?.path != null
}

/**
 * 新しいセッションの ViewModel。フォルダは `dir_list` を 1 階層ずつたどる（[browse]・[up]）。戻る（1 つ上へ）はたどった履歴で戻る。
 * 送信中（[StartSessionUiState.busy]）は全ボタンを無効にする。`pane_start` が成功したら [started] が 1 回届く。
 */
class StartSessionViewModel(
    private val panes: PaneService,
    private val snapshots: SnapshotService,
) : ViewModel() {
    private val mutableState = MutableStateFlow(StartSessionUiState())
    val state: StateFlow<StartSessionUiState> = mutableState.asStateFlow()

    private val mutableStarted = Channel<Unit>(Channel.BUFFERED)

    /** 起動できたとき（1 回だけ届く）。 */
    val started: Flow<Unit> = mutableStarted.receiveAsFlow()

    private val history = ArrayDeque<String?>()

    private var loadJob: Job? = null
    private var startJob: Job? = null

    /** [target] の `shell` ペインで始める画面を開く。同じペインなら何もしない（構成変更）。前の画面で未消費の [started] は捨てる。 */
    fun open(target: PaneTarget) {
        if (mutableState.value.target == target) return
        reset()
        mutableState.value = StartSessionUiState(target = target)
        initialize(target)
    }

    /**
     * 画面を離れたときの後始末（ナビゲーションが呼ぶ）。実行中の読み込み・起動を止め、状態（エージェント・フォルダ・タイトル）と
     * 未消費の [started] を捨てる。再び開いたとき、前の入力を引き継がない。
     */
    fun reset() {
        loadJob?.cancel()
        startJob?.cancel()
        loadJob = null
        startJob = null
        history.clear()
        mutableStarted.drain()
        mutableState.value = StartSessionUiState()
    }

    /** フォルダの一覧を読み込めていないとき（初回の `dir_list` の失敗など）の「再読み込み」。最初から読み直す。 */
    fun retryListing() {
        val state = mutableState.value
        val target = state.target ?: return
        if (state.listing != null || state.listingLoading || state.busy) return
        initialize(target)
    }

    // 種類の確認 → 最近使ったフォルダ → allowed_roots の一覧。
    private fun initialize(target: PaneTarget) {
        loadJob?.cancel()
        loadJob =
            viewModelScope.launch {
                val kind = attempt { panes.kind(target.connectionId) } ?: return@launch
                if (kind != ConnectionKind.Ops) {
                    mutableState.update { it.copy(viewOnly = true) }
                    return@launch
                }
                val recent = attempt { snapshots.cached(target.connectionId, target.session) }?.report?.panes.orEmpty()
                mutableState.update { it.copy(recent = recentFolders(recent)) }
                history.clear()
                load(null)
            }
    }

    fun selectAgent(agent: String) {
        if (agent in START_AGENTS && !mutableState.value.busy) mutableState.update { it.copy(agent = agent) }
    }

    fun onTitleChange(title: String) = mutableState.update { it.copy(title = title) }

    /** 最近使ったフォルダを選ぶ（許可外なら `pane_start` が `forbidden_path` を返し、固定文言で出る）。 */
    fun chooseRecent(path: String) {
        if (!mutableState.value.busy) mutableState.update { it.copy(folder = path) }
    }

    /** 今見ている階層をフォルダとして選ぶ。 */
    fun chooseListed() {
        val path = mutableState.value.listing?.path ?: return
        if (!mutableState.value.busy) mutableState.update { it.copy(folder = path) }
    }

    /** [path]（`dir_list` の項目の `path`）の下へ 1 階層たどる。 */
    fun browse(path: String) {
        if (mutableState.value.busy || mutableState.value.listingLoading) return
        history.addLast(mutableState.value.listing?.path)
        loadJob = viewModelScope.launch { if (!load(path)) history.removeLastOrNull() }
    }

    /** 1 つ上へ（たどった履歴で戻る。最初の階層なら何もしない）。 */
    fun up() {
        if (mutableState.value.busy || mutableState.value.listingLoading || history.isEmpty()) return
        val previous = history.removeLast()
        loadJob =
            viewModelScope.launch {
                if (!load(previous)) history.addLast(previous)
            }
    }

    fun requestConfirm() {
        if (mutableState.value.canConfirm && !mutableState.value.busy) mutableState.update { it.copy(confirming = true) }
    }

    fun dismissConfirm() = mutableState.update { it.copy(confirming = false) }

    /** 確認ダイアログの確定。`pane_start` を送る。 */
    fun start() {
        val current = mutableState.value
        val target = current.target ?: return
        val agent = current.agent
        val folder = current.folder
        if (!current.canConfirm || agent == null || folder == null) return
        if (mutableState.getAndUpdate { it.copy(busy = true, confirming = false, message = null) }.busy) return
        startJob =
            viewModelScope.launch {
                try {
                    val result = panes.start(target, agent, folder, current.title)
                    if (result.ok) mutableStarted.trySend(Unit)
                    mutableState.update { it.copy(message = startToast(result)) }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: FetchException) {
                    failed(e)
                } catch (e: ConnectionGoneException) {
                    mutableState.update { it.copy(message = UNEXPECTED_ERROR_MESSAGE) }
                } catch (e: Exception) {
                    mutableState.update { it.copy(message = rejectionMessage(e) ?: UNEXPECTED_ERROR_MESSAGE) }
                } finally {
                    mutableState.update { it.copy(busy = false) }
                }
            }
    }

    fun messageShown() = mutableState.update { it.copy(message = null) }

    // 1 階層を読む。成功したら true。失敗は固定文言で出し、直前の階層を残す。
    private suspend fun load(path: String?): Boolean {
        val target = mutableState.value.target ?: return false
        mutableState.update { it.copy(listingLoading = true) }
        try {
            val dirs = panes.dirList(target.connectionId, path)
            mutableState.update {
                it.copy(
                    listing = FolderListing(path = path?.let { dirs.path.ifEmpty { path } }, dirs.entries, dirs.truncated),
                    canGoUp = history.isNotEmpty(),
                    message = null,
                )
            }
            return true
        } catch (e: CancellationException) {
            throw e
        } catch (e: FetchException) {
            failed(e)
        } catch (e: Exception) {
            mutableState.update { it.copy(message = rejectionMessage(e) ?: UNEXPECTED_ERROR_MESSAGE) }
        } finally {
            mutableState.update { it.copy(listingLoading = false) }
        }
        return false
    }

    private fun failed(e: FetchException) {
        mutableState.update { it.copy(message = fetchErrorMessage(e), needsRepair = it.needsRepair || e.requiresRepair) }
    }

    private suspend fun <T> attempt(block: suspend () -> T): T? =
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            mutableState.update { it.copy(message = UNEXPECTED_ERROR_MESSAGE) }
            null
        }
}
