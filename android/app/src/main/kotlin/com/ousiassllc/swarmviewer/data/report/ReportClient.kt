package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.InvalidResponseException
import com.ousiassllc.swarmviewer.domain.InvalidUrlException
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.domain.classify
import com.ousiassllc.swarmviewer.domain.parseReport
import com.ousiassllc.swarmviewer.domain.parseSessions
import com.ousiassllc.swarmviewer.domain.validateUrl
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import okhttp3.OkHttpClient
import java.io.IOException
import java.net.Proxy
import java.util.concurrent.TimeUnit

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
 * swarm-console `/report`（MCP Streamable HTTP、JSON-RPC 2.0）のクライアント。
 *
 * 呼べるツールは `sessions_list` と `report` だけ（[ReportTools]、NFR-SEC-1）。`POST` のみで、`initialize` →
 * `notifications/initialized` → `tools/call` の順に送る（API 仕様 §1.6、デスクトップの `internal/repository/report` と同じ手順）。
 * セッション ID が返ったときだけ終了時に `DELETE` を送るが、その応答（405 を含む）は失敗にしない。`GET`（常駐 SSE）は開かない。
 * token は [tokens] から接続 ID で解決し、戻り値・例外・ログに出さない（NFR-SEC-2・11）。
 *
 * 失敗は常に [FetchException]（`classify` の分類。API 仕様 §1.5）。コルーチンのキャンセルはそのまま伝わる。
 */
class ReportClient(
    private val tokens: (connectionId: String) -> String?,
    private val http: OkHttpClient = newReportHttpClient(),
    private val timeoutMillis: Long = REQUEST_TIMEOUT_MILLIS,
    private val logger: FetchLogger? = null,
) {
    /** 許可されたセッションの一覧を返す（`sessions_list`。応答の順のまま）。 */
    suspend fun sessions(conn: Connection): List<Session> =
        parsed(conn.id, ReportTools.SESSIONS_LIST) {
            parseSessions(callTool(conn.url, { tokens(conn.id) }, ReportTools.SESSIONS_LIST, buildJsonObject {}))
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
            parseReport(callTool(conn.url, { tokens(conn.id) }, ReportTools.REPORT, args))
        }
    }

    /** ペアリングの疎通確認。保存前なので token を直接受け取る。許可されたセッションの一覧を返す。 */
    suspend fun probe(
        url: String,
        token: String,
    ): List<Session> =
        parsed(null, ReportTools.SESSIONS_LIST) {
            parseSessions(callTool(url, { token }, ReportTools.SESSIONS_LIST, buildJsonObject {}))
        }

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

    /** セッションを開き、ツールを 1 回呼び、閉じる。応答の先頭のテキストを返す。 */
    private suspend fun callTool(
        rawUrl: String,
        token: () -> String?,
        tool: String,
        args: JsonObject,
    ): String {
        if (tool !in ReportTools.allowed) throw FetchException(ErrKind.Protocol, "tool_not_allowed")
        val endpoint =
            try {
                validateUrl(rawUrl)
            } catch (e: InvalidUrlException) {
                throw FetchException(ErrKind.Protocol, "invalid_url")
            }
        // 保護領域（ファイル・Keystore）の読み出しは呼び出し元のスレッドで行わない。
        val bearer =
            try {
                withContext(Dispatchers.IO) { token() }
            } catch (e: IOException) {
                throw FetchException(ErrKind.Unreachable, "secret_store") // 保護領域を読めない（原因・パスは出さない）
            } ?: throw FetchException(ErrKind.Unauthorized, "token_missing")
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis)
        val client = http.newBuilder().addInterceptor(GuardInterceptor(endpoint, bearer)).build()
        try {
            return McpExchange(client, endpoint, deadline).callTool(tool, args)
        } catch (e: IOException) {
            currentCoroutineContext().ensureActive() // キャンセルによる失敗は分類せず、キャンセルとして伝える
            throw if (e is RequestRejectedException) {
                FetchException(ErrKind.Protocol, "request_rejected")
            } else {
                classify(e) ?: FetchException(ErrKind.Unreachable)
            }
        }
    }
}
