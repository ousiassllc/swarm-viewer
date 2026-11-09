package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.report.PaneApi
import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.AskOption
import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.DirEntry
import com.ousiassllc.swarmviewer.domain.DirList
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneTail
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.Skill
import com.ousiassllc.swarmviewer.domain.SkillList
import com.ousiassllc.swarmviewer.domain.StartResult
import kotlinx.coroutines.CompletableDeferred

/**
 * 呼び出しを記録する [PaneApi]（JVM のテスト用）。[calls] は「ツール名 引数…」の行。[gate] があれば、それが開くまで操作ツール（`skill_list` を除く）は応答しない。
 * [failure] があれば、操作ツールはそれを投げる（[sendFailure] は `send_text` だけ、[skillFailure] は `skill_list` だけ）。結果は差し替えられる。
 */
internal class FakePaneApi : PaneApi {
    val calls = ArrayList<String>()
    var gate: CompletableDeferred<Unit>? = null
    var failure: FetchException? = null
    var sendFailure: FetchException? = null
    var skillFailure: FetchException? = null

    /**
     * あれば、次の `ask_card` 1 回だけ、それが開くまで応答しない（質問カードの再取得が終わる前の表示の検査用）。
     * 応答する中身は呼ばれた時点の [card]（待っている間に変えても、その呼び出しには反映しない）。
     */
    var cardGate: CompletableDeferred<Unit>? = null

    /** あれば、次の `pane_tail` 1 回だけ、それが開くまで応答しない。 */
    var tailGate: CompletableDeferred<Unit>? = null
    var card = AskCard(true, "どうしますか？", listOf(AskOption(1, "A", true), AskOption(2, "B")), cursor = 1, allowsCustom = true)
    var pick = PickResult(ok = true, state = "working", pickedN = 1, pickedLabel = "A", cardClosed = true)
    var send = SendResult(ok = true, state = "working", accepted = true, chars = 3)
    var escape = EscapeResult(ok = true, state = "idle", interrupted = true)
    var start = StartResult(ok = true, state = "idle", agent = "omp", cwd = "/work/app", title = "t")
    var close = CloseResult(ok = true, state = "shell", noteArchived = true)
    var dirs = DirList(path = "/work", entries = listOf(DirEntry("app", "/work/app", git = true)), truncated = false)
    var skills =
        SkillList(
            agent = "omp",
            skills = listOf(Skill("impl", "実装する", "/impl"), Skill("bug-fix", "バグを直す", "/bug-fix")),
            truncated = false,
        )
    var tail = PaneTail(text = "tail-body", truncated = false)

    private suspend fun <T> operation(
        line: String,
        result: () -> T,
    ): T {
        calls += line
        gate?.await()
        failure?.let { throw it }
        return result()
    }

    override suspend fun paneTail(
        conn: Connection,
        session: String,
        pane: String,
        lines: Int,
    ): PaneTail {
        calls += "pane_tail $session $pane $lines"
        val gate = tailGate
        tailGate = null
        gate?.await()
        failure?.let { throw it }
        return tail
    }

    override suspend fun askCard(
        conn: Connection,
        session: String,
        pane: String,
    ): AskCard {
        calls += "ask_card $session $pane"
        val answered = card
        val gate = cardGate
        cardGate = null
        gate?.await()
        return answered
    }

    override suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        n: Int,
    ): PickResult = operation("ask_pick $session $pane n=$n") { pick }

    override suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): PickResult = operation("ask_pick $session $pane text=$text") { pick }

    override suspend fun sendText(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): SendResult =
        operation("send_text $session $pane $text") {
            sendFailure?.let { throw it }
            send
        }

    override suspend fun sendEscape(
        conn: Connection,
        session: String,
        pane: String,
        confirm: Boolean,
    ): EscapeResult = operation("send_escape $session $pane confirm=$confirm") { escape }

    override suspend fun paneStart(
        conn: Connection,
        session: String,
        pane: String,
        agent: String,
        cwd: String,
        title: String,
    ): StartResult = operation("pane_start $session $pane $agent $cwd $title") { start }

    override suspend fun paneClose(
        conn: Connection,
        session: String,
        pane: String,
    ): CloseResult = operation("pane_close $session $pane") { close }

    override suspend fun dirList(
        conn: Connection,
        path: String?,
    ): DirList = operation("dir_list ${path.orEmpty()}") { dirs }

    override suspend fun skillList(
        conn: Connection,
        agent: String,
    ): SkillList {
        calls += "skill_list $agent" // 読み取りなので [gate] では止めない
        skillFailure?.let { throw it }
        return skills.copy(agent = agent)
    }
}
