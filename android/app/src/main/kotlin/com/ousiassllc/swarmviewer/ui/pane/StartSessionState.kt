package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.DirEntry
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.FirstTask
import com.ousiassllc.swarmviewer.domain.Skill
import com.ousiassllc.swarmviewer.domain.normalizeTitle
import com.ousiassllc.swarmviewer.service.FirstTaskStatus
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.service.StartOutcome
import com.ousiassllc.swarmviewer.ui.FIRST_TASK_FAILED_MESSAGE
import com.ousiassllc.swarmviewer.ui.firstTaskNotIdleMessage
import com.ousiassllc.swarmviewer.ui.sendToast
import com.ousiassllc.swarmviewer.ui.skillListFailureMessage

/** フォルダピッカーの 1 階層。[path] が null のときは `allowed_roots`（根の一覧）。 */
data class FolderListing(
    val path: String? = null,
    val entries: List<DirEntry> = emptyList(),
    val truncated: Boolean = false,
)

/**
 * スキル一覧（`skill_list`）の取得状態。取得結果は画面を開いている間のメモリだけに持ち、保存しない。
 * どの状態でも「スキルなし」で起動できる（スキルが選べないことで起動を止めない）。
 */
sealed interface SkillListing {
    /** エージェントを選ぶ前。 */
    data object None : SkillListing

    data object Loading : SkillListing

    data class Loaded(
        val skills: List<Skill>,
        val truncated: Boolean,
    ) : SkillListing

    /** 取得できなかった。[message] は固定文言。[reload] が true のときだけ「再読み込み」を出す（`unknown_tool` は読み直しても変わらない）。 */
    data class Failed(
        val message: String,
        val reload: Boolean,
    ) : SkillListing

    companion object {
        /** [e]（`skill_list` の失敗）の固定文言。`unknown_tool` は、読み直しても変わらないので「再読み込み」を付けない。 */
        fun failed(e: FetchException) = Failed(skillListFailureMessage(e), reload = e.kind != ErrKind.Operation || e.code != "unknown_tool")
    }
}

/**
 * 起動に成功したが、最初のタスクを送っていない・送れなかったときの結果の表示。起動は取り消さない。
 * [reason] は送れなかった理由（固定文言）。[canResend] が true のときだけ「もう一度送る」を出す（`send_text` だけを送り直す）。
 * 最初のタスクの本文は、この表示の間だけ [StartSessionUiState.task] に残る。
 */
data class FirstTaskNotice(
    val message: String,
    val reason: String? = null,
    val canResend: Boolean = false,
)

/**
 * 起動の結果から、最初のタスクの結果の表示を作る（送った・タスクなし・`pane_start` が `ok: false` は null）。
 * [errorText] は `send_text` が失敗したときの理由の固定文言（`accepted: false` は「受け付けられませんでした」）。
 */
internal fun StartOutcome.notice(errorText: String?): FirstTaskNotice? =
    when (task) {
        FirstTaskStatus.NotIdle -> FirstTaskNotice(firstTaskNotIdleMessage(start.state))
        FirstTaskStatus.Failed -> FirstTaskNotice(FIRST_TASK_FAILED_MESSAGE, errorText ?: send?.let(::sendToast), canResend = true)
        FirstTaskStatus.None, FirstTaskStatus.Sent -> null
    }

/** 確認で要約に出す本文の先頭の文字数。 */
private const val TASK_HEAD_CHARS = 80

/**
 * 新しいセッション（`pane_start`）の画面の状態（機能要件 §4.7・§4.8）。手順: エージェント → フォルダ → スキル（任意）→ 本文（任意）→
 * タイトル → 確認。[viewOnly] の接続では何も選べず、操作の部品（スキル・本文を含む）を描画しない（この画面に入る導線も出ない）。
 * [folder] は選択済みのフォルダ（`dir_list` の `path`、または最近使ったフォルダ）。[listing] はピッカーの現在の階層。
 * [skill]・[task] は選択中のスキルと本文で、ViewModel が持つので折りたたみ⇔展開でも失われない（`reset()` で破棄）。
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
    val skills: SkillListing = SkillListing.None,
    val skill: Skill? = null,
    val task: String = "",
    val title: String = "",
    val confirming: Boolean = false,
    val busy: Boolean = false,
    val notice: FirstTaskNotice? = null,
    val message: String? = null,
    val needsRepair: Boolean = false,
) {
    /** 起動のあとに送る最初のタスク（スキルも本文も空なら、起動だけ）。 */
    val firstTask: FirstTask get() = FirstTask(skill?.invoke.orEmpty(), task)

    /** 最初のタスクの問題（[com.ousiassllc.swarmviewer.domain.sendTextProblem] のコード）。無ければ null。 */
    val taskProblem: String? get() = firstTask.problem

    /** 確認の要約に出す本文の先頭（前後の空白を除いて [TASK_HEAD_CHARS] 字まで）。本文が空なら空文字。 */
    val taskHead: String
        get() {
            val body = task.trim()
            val end =
                if (body.codePointCount(0, body.length) >
                    TASK_HEAD_CHARS
                ) {
                    body.offsetByCodePoints(0, TASK_HEAD_CHARS)
                } else {
                    body.length
                }
            return body.substring(0, end) + if (end < body.length) "…" else ""
        }

    /** 確認へ進める: エージェント・フォルダ・タイトルが揃い、最初のタスクに問題が無い（タイトルは検査済み）。結果の表示の間は進めない。 */
    val canConfirm: Boolean
        get() = !viewOnly && notice == null && agent != null && folder != null && normalizeTitle(title) != null && taskProblem == null

    /** 「このフォルダを選ぶ」を出せる階層か（根の一覧では選べない）。 */
    val canChooseListed: Boolean get() = listing?.path != null
}
