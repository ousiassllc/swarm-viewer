package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.report.PaneApi
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.DEFAULT_TAIL_LINES
import com.ousiassllc.swarmviewer.domain.DirList
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneTail
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.START_AGENTS
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.StartResult
import com.ousiassllc.swarmviewer.domain.normalizeTitle
import com.ousiassllc.swarmviewer.domain.sendTextProblem
import com.ousiassllc.swarmviewer.domain.validCwd

/** ペインの特定。[pos] は report の `panes[].pos`（セッション内の位置）。 */
data class PaneTarget(
    val connectionId: String,
    val session: String,
    val pos: String,
)

/** 見るだけの接続（URL の path が `/report`）に操作を求めた。通信せずに拒否する。 */
class OpsRequiredException : Exception("this connection is view-only")

/** 同じペインへの別の操作が実行中。通信せずに拒否する（二重送信の防止）。 */
class PaneBusyException : Exception("another operation is running on this pane")

/**
 * ペインの閲覧と操作（機能要件 §4.7、FR-AND-9〜）。
 *  - 閲覧（[tail]・[askCard]）は見るだけの接続でも呼べる
 *  - 操作（[pickOption]・[pickText]・[send]・[escape]・[start]・[close]・[dirList]）は接続の種類（URL の path から導く
 *    [ConnectionKind]）を確かめ、見るだけなら**通信せず** [OpsRequiredException]
 *  - 同じペイン（接続・セッション・位置）への操作は同時に 1 つだけ。実行中に次が来たら通信せず [PaneBusyException]
 *  - 送る本文・タイトル・フォルダ・エージェントは、送る前に検査する（違えば通信せず `FetchException(Operation, <code>)`）
 *  - 401・403 は再ペアリングが必要な接続として記録し（[ConnectionService.markNeedsRepair]）、その接続への操作は通信せず止める
 *
 * 失敗は domain の `FetchException`（本文・token を含まない）。ペインの本文・送信本文はログに出さない。
 */
class PaneService(
    private val api: PaneApi,
    private val store: ViewerStore,
    private val connections: ConnectionService,
) {
    private val lock = Any()
    private val running = HashSet<PaneTarget>()

    /** [connectionId] の種類。接続が無ければ [ConnectionGoneException]。 */
    suspend fun kind(connectionId: String): ConnectionKind = connection(connectionId).kind

    suspend fun tail(
        target: PaneTarget,
        lines: Int = DEFAULT_TAIL_LINES,
    ): PaneTail = viewing(target.connectionId) { api.paneTail(it, target.session, target.pos, lines) }

    suspend fun askCard(target: PaneTarget): AskCard = viewing(target.connectionId) { api.askCard(it, target.session, target.pos) }

    /** 選択肢を番号で選ぶ（押したらすぐ送る）。 */
    suspend fun pickOption(
        target: PaneTarget,
        n: Int,
    ): PickResult = operating(target) { api.askPick(it, target.session, target.pos, n) }

    /** 自由入力で答える（8000 字まで）。 */
    suspend fun pickText(
        target: PaneTarget,
        text: String,
    ): PickResult {
        checkText(text)
        return operating(target) { api.askPick(it, target.session, target.pos, text) }
    }

    /** 指示を送る（8000 字まで、改行・タブ以外の制御文字は不可）。 */
    suspend fun send(
        target: PaneTarget,
        text: String,
    ): SendResult {
        checkText(text)
        return operating(target) { api.sendText(it, target.session, target.pos, text) }
    }

    /** 中断（Esc）。[confirm] は確認ダイアログを経たときだけ true（false は通信せず `confirm_required`）。 */
    suspend fun escape(
        target: PaneTarget,
        confirm: Boolean,
    ): EscapeResult {
        if (!confirm) throw FetchException(ErrKind.Operation, "confirm_required")
        return operating(target) { api.sendEscape(it, target.session, target.pos, confirm = true) }
    }

    /** 新しいセッションを始める。[title] は前後の空白を除いて検査する。 */
    suspend fun start(
        target: PaneTarget,
        agent: String,
        cwd: String,
        title: String,
    ): StartResult {
        if (agent !in START_AGENTS) throw FetchException(ErrKind.Operation, "unsupported_agent")
        if (!validCwd(cwd)) throw FetchException(ErrKind.Operation, "invalid_args")
        val normalized = normalizeTitle(title) ?: throw FetchException(ErrKind.Operation, "invalid_args")
        return operating(target) { api.paneStart(it, target.session, target.pos, agent, cwd, normalized) }
    }

    /** ペインを終了する（確認ダイアログは呼び出し側）。 */
    suspend fun close(target: PaneTarget): CloseResult = operating(target) { api.paneClose(it, target.session, target.pos) }

    /** フォルダの 1 階層。[path] が null なら `allowed_roots`。ペインを取らないので排他の対象外。 */
    suspend fun dirList(
        connectionId: String,
        path: String?,
    ): DirList = ops(connectionId) { api.dirList(it, path) }

    private fun checkText(text: String) {
        sendTextProblem(text)?.let { throw FetchException(ErrKind.Operation, it) }
    }

    private suspend fun connection(id: String): Connection = store.connection(id) ?: throw ConnectionGoneException()

    private suspend fun <T> viewing(
        connectionId: String,
        call: suspend (Connection) -> T,
    ): T = guarded(connection(connectionId), call)

    private suspend fun <T> ops(
        connectionId: String,
        call: suspend (Connection) -> T,
    ): T {
        val conn = connection(connectionId)
        if (conn.kind != ConnectionKind.Ops) throw OpsRequiredException()
        return guarded(conn, call)
    }

    private suspend fun <T> operating(
        target: PaneTarget,
        call: suspend (Connection) -> T,
    ): T {
        val conn = connection(target.connectionId)
        if (conn.kind != ConnectionKind.Ops) throw OpsRequiredException()
        synchronized(lock) { if (!running.add(target)) throw PaneBusyException() }
        try {
            return guarded(conn, call)
        } finally {
            synchronized(lock) { running.remove(target) }
        }
    }

    // 再ペアリングが必要な接続へは通信しない。401・403 は記録して伝える。
    private suspend fun <T> guarded(
        conn: Connection,
        call: suspend (Connection) -> T,
    ): T {
        if (conn.id in connections.needsRepair.value) throw FetchException(ErrKind.Unauthorized, "needs_repair")
        try {
            return call(conn)
        } catch (e: FetchException) {
            if (e.kind == ErrKind.Unauthorized || e.kind == ErrKind.ForbiddenScope) connections.markNeedsRepair(conn.id)
            throw e
        }
    }
}
