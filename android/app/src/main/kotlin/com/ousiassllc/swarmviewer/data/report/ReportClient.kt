package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.DirList
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.InvalidResponseException
import com.ousiassllc.swarmviewer.domain.PaneTail
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.domain.StartResult
import com.ousiassllc.swarmviewer.domain.normalizeTailLines
import com.ousiassllc.swarmviewer.domain.parseAskCard
import com.ousiassllc.swarmviewer.domain.parseClose
import com.ousiassllc.swarmviewer.domain.parseDirList
import com.ousiassllc.swarmviewer.domain.parseEscape
import com.ousiassllc.swarmviewer.domain.parsePaneTail
import com.ousiassllc.swarmviewer.domain.parsePick
import com.ousiassllc.swarmviewer.domain.parseReport
import com.ousiassllc.swarmviewer.domain.parseSend
import com.ousiassllc.swarmviewer.domain.parseSessions
import com.ousiassllc.swarmviewer.domain.parseStart
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonObjectBuilder
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import okhttp3.OkHttpClient
import java.net.Proxy

/** 1 回の呼び出し（接続・応答の合計）の上限（NFR-PERF-4、API 仕様 §1.1）。リトライはしない（NFR-AVL-2）。 */
const val REQUEST_TIMEOUT_MILLIS = 10_000L

/**
 * `/report` の呼び出し失敗の記録先。接続 ID（あれば）・ツール名・種別・コードだけを受け取る
 * （URL・token・応答本文は渡さない。NFR-LOG-3・NFR-SEC-11）。
 */
fun interface FetchLogger {
    fun failed(
        connectionId: String?,
        tool: String,
        kind: String,
        code: String,
    )
}

/**
 * 環境のプロキシを使わず、リダイレクトに追従しない [OkHttpClient]。token を意図しない宛先へ流さないため（デスクトップ版と同じ）。
 *
 * 接続の再利用に失敗したとき（サーバーが閉じた待機中の接続）だけは、OkHttp が新しい接続で送り直す（既定。デスクトップ版の
 * Go の `http.Transport` と同じ）。これは接続の問題の回復で、取得の失敗（タイムアウト・応答の誤り）の再試行ではない。
 * 呼び出し全体の期限（[REQUEST_TIMEOUT_MILLIS]）の中で行い、取得の失敗を繰り返し試すことはしない（NFR-AVL-2）。
 */
fun newReportHttpClient(): OkHttpClient =
    OkHttpClient
        .Builder()
        .proxy(Proxy.NO_PROXY)
        .followRedirects(false)
        .followSslRedirects(false)
        .build()

/**
 * swarm-console（`/report`・`/ops`。MCP Streamable HTTP、JSON-RPC 2.0）のクライアント。
 *
 * 呼べるツールは接続の種類（URL の path）ごとの許可リストだけ（[ReportTools]、NFR-SEC-1）。見るだけは `sessions_list`・`report`・
 * `pane_tail`・`ask_card`、操作はそれに `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list`。
 * 見るだけの接続で操作ツールを呼ぶと、通信せず `protocol`（`tool_not_allowed`）。
 * `POST` のみで、`initialize` → `notifications/initialized` → `tools/call` の順に送る（API 仕様 §1.6、デスクトップの `internal/repository/report` と同じ手順）。
 * セッション ID が返ったときだけ終了時に `DELETE` を送るが、その応答（405 を含む）は失敗にしない。`GET`（常駐 SSE）は開かない。
 * token は [tokens] から接続 ID で解決し、戻り値・例外・ログに出さない（NFR-SEC-2・11）。送信本文・ペインの本文もログに出さない。
 *
 * 失敗は常に [FetchException]（`classify` の分類。API 仕様 §1.5）。コルーチンのキャンセルはそのまま伝わる。
 */
class ReportClient(
    private val tokens: (connectionId: String) -> String?,
    private val http: OkHttpClient = newReportHttpClient(),
    private val timeoutMillis: Long = REQUEST_TIMEOUT_MILLIS,
    private val logger: FetchLogger? = null,
) : PaneApi {
    private val transport = ToolTransport(http, timeoutMillis)

    /** 許可されたセッションの一覧を返す（`sessions_list`。応答の順のまま）。 */
    suspend fun sessions(conn: Connection): List<Session> =
        parsed(conn.id, ReportTools.SESSIONS_LIST) {
            parseSessions(transport.call(conn.url, { tokens(conn.id) }, ReportTools.SESSIONS_LIST, buildJsonObject {}))
        }

    /**
     * 1 セッションのスナップショットを返す（`report`、format=json）。session は常に明示して送る。
     * 空なら通信せず `session_required`。
     */
    suspend fun report(
        conn: Connection,
        session: String,
    ): Report {
        if (session.isEmpty()) throw FetchException(ErrKind.SessionRequired, "session_required")
        val args =
            buildJsonObject {
                put("format", "json")
                put("session", session)
            }
        return parsed(conn.id, ReportTools.REPORT) {
            parseReport(transport.call(conn.url, { tokens(conn.id) }, ReportTools.REPORT, args))
        }
    }

    /** ペアリングの疎通確認。保存前なので token を直接受け取る。許可されたセッションの一覧を返す。 */
    suspend fun probe(
        url: String,
        token: String,
    ): List<Session> =
        parsed(null, ReportTools.SESSIONS_LIST) {
            parseSessions(transport.call(url, { token }, ReportTools.SESSIONS_LIST, buildJsonObject {}))
        }

    /** 末尾 [lines] 行は 1〜200 に丸める（`raw` は付けない）。見るだけの接続でも呼べる。 */
    override suspend fun paneTail(
        conn: Connection,
        session: String,
        pane: String,
        lines: Int,
    ): PaneTail = tool(conn, ReportTools.PANE_TAIL, paneArgs(session, pane) { put("lines", normalizeTailLines(lines)) }, ::parsePaneTail)

    /** 見るだけの接続でも呼べる。 */
    override suspend fun askCard(
        conn: Connection,
        session: String,
        pane: String,
    ): AskCard = tool(conn, ReportTools.ASK_CARD, paneArgs(session, pane), ::parseAskCard)

    override suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        n: Int,
    ): PickResult = tool(conn, ReportTools.ASK_PICK, paneArgs(session, pane) { put("n", n) }, ::parsePick)

    /** `n` とは同時に指定しない。 */
    override suspend fun askPick(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): PickResult = tool(conn, ReportTools.ASK_PICK, paneArgs(session, pane) { put("text", text) }, ::parsePick)

    /** 本文の検査（8000 字・制御文字）は呼び出し側（`PaneService`）。 */
    override suspend fun sendText(
        conn: Connection,
        session: String,
        pane: String,
        text: String,
    ): SendResult = tool(conn, ReportTools.SEND_TEXT, paneArgs(session, pane) { put("text", text) }, ::parseSend)

    /** [confirm] が true のときだけ送る（false は通信せず `confirm_required`）。 */
    override suspend fun sendEscape(
        conn: Connection,
        session: String,
        pane: String,
        confirm: Boolean,
    ): EscapeResult {
        if (!confirm) throw FetchException(ErrKind.Operation, "confirm_required")
        return tool(conn, ReportTools.SEND_ESCAPE, paneArgs(session, pane) { put("confirm", true) }, ::parseEscape)
    }

    override suspend fun paneStart(
        conn: Connection,
        session: String,
        pane: String,
        agent: String,
        cwd: String,
        title: String,
    ): StartResult =
        tool(
            conn,
            ReportTools.PANE_START,
            paneArgs(session, pane) {
                put("agent", agent)
                put("cwd", cwd)
                put("title", title)
            },
            ::parseStart,
        )

    override suspend fun paneClose(
        conn: Connection,
        session: String,
        pane: String,
    ): CloseResult = tool(conn, ReportTools.PANE_CLOSE, paneArgs(session, pane), ::parseClose)

    /** `session` は取らない（`limit` も指定しない）。 */
    override suspend fun dirList(
        conn: Connection,
        path: String?,
    ): DirList = tool(conn, ReportTools.DIR_LIST, buildJsonObject { path?.let { put("path", it) } }, ::parseDirList)

    // ペインを取るツールの共通呼び出し。session・pane は常に明示する（空なら通信せず拒否）。
    private fun paneArgs(
        session: String,
        pane: String,
        extra: JsonObjectBuilder.() -> Unit = {},
    ): JsonObject {
        if (session.isEmpty()) throw FetchException(ErrKind.SessionRequired, "session_required")
        if (pane.isEmpty()) throw FetchException(ErrKind.Operation, "invalid_args")
        return buildJsonObject {
            put("session", session)
            put("pane", pane)
            extra()
        }
    }

    // 接続 [conn] で [tool] を呼んで結果を [parse] する。`invalid_args` の分類（session_missing か operation か）は McpExchange が行う。
    private suspend fun <T> tool(
        conn: Connection,
        tool: String,
        args: JsonObject,
        parse: (String) -> T,
    ): T = parsed(conn.id, tool) { parse(transport.call(conn.url, { tokens(conn.id) }, tool, args)) }

    // 応答の解析失敗（想定外の形）を protocol に分類し、すべての失敗を記録する。
    private suspend fun <T> parsed(
        connectionId: String?,
        tool: String,
        block: suspend () -> T,
    ): T =
        try {
            block()
        } catch (e: InvalidResponseException) {
            throw logged(connectionId, tool, FetchException(ErrKind.Protocol))
        } catch (e: FetchException) {
            throw logged(connectionId, tool, e)
        }

    private fun logged(
        connectionId: String?,
        tool: String,
        e: FetchException,
    ): FetchException {
        logger?.failed(connectionId, tool, e.kind.id, e.code)
        return e
    }
}
