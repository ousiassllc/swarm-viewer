package com.ousiassllc.swarmviewer.ui.pane

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.START_AGENTS
import com.ousiassllc.swarmviewer.domain.Skill
import com.ousiassllc.swarmviewer.domain.recentFolders
import com.ousiassllc.swarmviewer.service.PaneService
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.service.SnapshotService
import com.ousiassllc.swarmviewer.service.StartOutcome
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import com.ousiassllc.swarmviewer.ui.fetchErrorMessage
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import com.ousiassllc.swarmviewer.ui.rejectionMessage
import com.ousiassllc.swarmviewer.ui.sendToast
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

/**
 * 新しいセッションの ViewModel。フォルダは `dir_list` を 1 階層ずつたどる（[browse]・[up]）。戻る（1 つ上へ）はたどった履歴で戻る。
 * エージェントを選ぶと `skill_list` でスキルを取る（[retrySkills]）。取れなくても、スキルなしで本文だけ・タスクなしの起動はできる。
 * 送信中（[StartSessionUiState.busy]）は全ボタンを無効にする。起動して最初のタスクも送れた（または無かった）ら [started] が 1 回届く。
 * 最初のタスクを送っていない・送れなかったときは [StartSessionUiState.notice] を出し、[resend]（`send_text` だけ）か [openPane] へ進める。
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
    private var skillJob: Job? = null

    /** [target] の `shell` ペインで始める画面を開く。同じペインなら何もしない（構成変更）。前の画面で未消費の [started] は捨てる。 */
    fun open(target: PaneTarget) {
        if (mutableState.value.target == target) return
        reset()
        mutableState.value = StartSessionUiState(target = target)
        initialize(target)
    }

    /**
     * 画面を離れたときの後始末（ナビゲーションが呼ぶ）。実行中の読み込み・起動を止め、状態（エージェント・フォルダ・スキル・本文・
     * タイトル・取得済みのスキル一覧・結果の表示）と未消費の [started] を捨てる。再び開いたとき、前の入力を引き継がない。
     */
    fun reset() {
        listOf(loadJob, startJob, skillJob).forEach { it?.cancel() }
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

    /** エージェントを選ぶ。変えたときは、選んでいたスキルを外してスキル一覧を取り直す。 */
    fun selectAgent(agent: String) {
        val current = mutableState.value
        if (agent !in START_AGENTS || current.busy || current.viewOnly || current.notice != null || agent == current.agent) return
        mutableState.update { it.copy(agent = agent, skill = null) }
        loadSkills(agent)
    }

    /** スキル一覧を読み込めなかったとき（`unknown_tool` を除く）の「再読み込み」。 */
    fun retrySkills() {
        val current = mutableState.value
        val agent = current.agent ?: return
        if ((current.skills as? SkillListing.Failed)?.reload == true && !current.busy) loadSkills(agent)
    }

    /** スキルを選ぶ。null は「スキルなし」。一覧に無いスキルは選べない。 */
    fun selectSkill(skill: Skill?) {
        val listing = mutableState.value.skills
        if (mutableState.value.busy || (skill != null && (listing as? SkillListing.Loaded)?.skills?.contains(skill) != true)) return
        mutableState.update { it.copy(skill = skill) }
    }

    /** 本文（任意）。改行は `\n` に揃える（`\r` は制御文字として拒否されるため）。 */
    fun onTaskChange(task: String) = mutableState.update { it.copy(task = task.replace("\r\n", "\n").replace('\r', '\n')) }

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

    /** 確認ダイアログの確定。`pane_start` を送り、最初のタスクがあれば（`idle` のときだけ）続けて `send_text` を送る。 */
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
                    started(panes.start(target, agent, folder, current.title, current.firstTask))
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    showFailure(e)
                } finally {
                    mutableState.update { it.copy(busy = false) }
                }
            }
    }

    /**
     * 「もう一度送る」。最初のタスクを `send_text` だけで送り直す（`pane_start` は呼ばない。自動の再送はしない）。送る前に最新の
     * `report` でペインが `idle` かを確かめ、`idle` でなければ通信せず断る。成功したら [started] が届く。
     */
    fun resend() {
        val current = mutableState.value
        val target = current.target ?: return
        val notice = current.notice?.takeIf { it.canResend } ?: return
        if (mutableState.getAndUpdate { it.copy(busy = true, message = null) }.busy) return
        startJob =
            viewModelScope.launch {
                try {
                    val pane =
                        snapshots
                            .refresh(target.connectionId, target.session)
                            .report.panes
                            .firstOrNull { it.pos == target.pos }
                    val rejected =
                        when {
                            pane == null -> operationErrorMessage("pane_not_found")
                            pane.state != PaneState.Idle -> operationErrorMessage("invalid_state")
                            else -> panes.resend(target, current.firstTask).let { if (it.accepted) null else sendToast(it) }
                        }
                    if (rejected ==
                        null
                    ) {
                        mutableStarted.trySend(Unit)
                    } else {
                        mutableState.update { it.copy(notice = notice.copy(reason = rejected)) }
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    val text = failureText(e)
                    mutableState.update { it.copy(notice = notice.copy(reason = text)) }
                } finally {
                    mutableState.update { it.copy(busy = false) }
                }
            }
    }

    /** 結果の表示の「ペイン詳細を開く」。最初のタスクの本文はここで破棄される（ナビゲーションが [reset] を呼ぶ）。 */
    fun openPane() {
        if (mutableState.value.notice != null && !mutableState.value.busy) mutableStarted.trySend(Unit)
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
        } catch (e: Exception) {
            showFailure(e)
        } finally {
            mutableState.update { it.copy(listingLoading = false) }
        }
        return false
    }

    // エージェントのスキル一覧。古い応答（エージェントを変えたあと）は捨てる。
    private fun loadSkills(agent: String) {
        val target = mutableState.value.target ?: return
        skillJob?.cancel()
        mutableState.update { it.copy(skills = SkillListing.Loading) }
        skillJob =
            viewModelScope.launch {
                val listing =
                    try {
                        panes.skills(target.connectionId, agent).let { SkillListing.Loaded(it.skills, it.truncated) }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        val text = failureText(e)
                        if (e is FetchException) SkillListing.failed(e) else SkillListing.Failed(text, reload = true)
                    }
                mutableState.update { if (it.agent == agent) it.copy(skills = listing) else it }
            }
    }

    // 起動の結果。最初のタスクを送っていない・送れなかったときは、遷移せずに結果の表示を出す（本文は state に残る）。
    private fun started(outcome: StartOutcome) {
        val notice = outcome.notice(outcome.error?.let(::failureText))
        if (outcome.start.ok && notice == null) mutableStarted.trySend(Unit)
        mutableState.update { it.copy(notice = notice, message = if (notice == null) startToast(outcome.start) else null) }
    }

    private fun showFailure(e: Exception) {
        val text = failureText(e)
        mutableState.update { it.copy(message = text) }
    }

    // 失敗の固定文言。401・403 は再ペアリングの案内を出す。
    private fun failureText(e: Exception): String {
        if (e is FetchException) {
            mutableState.update { it.copy(needsRepair = it.needsRepair || e.requiresRepair) }
            return fetchErrorMessage(e)
        }
        return rejectionMessage(e) ?: UNEXPECTED_ERROR_MESSAGE
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
