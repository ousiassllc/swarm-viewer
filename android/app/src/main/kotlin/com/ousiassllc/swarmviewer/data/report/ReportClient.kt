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
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonObject
import okhttp3.Call
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.net.Proxy
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

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
 * 環境のプロキシを使わず、リダイレクトに追従せず、接続失敗でも再試行しない [OkHttpClient]。
 * token を意図しない宛先へ流さないため（デスクトップ版と同じ）。
 */
fun newReportHttpClient(): OkHttpClient =
    OkHttpClient
        .Builder()
        .proxy(Proxy.NO_PROXY)
        .followRedirects(false)
        .followSslRedirects(false)
        .retryOnConnectionFailure(false)
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
        val bearer = token() ?: throw FetchException(ErrKind.Unauthorized, "token_missing")
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

/** 1 回のツール呼び出しの間だけ生きる MCP のやり取り。全体の期限 [deadline]（System.nanoTime 基準）を共有する。 */
private class McpExchange(
    private val client: OkHttpClient,
    private val endpoint: String,
    private val deadline: Long,
) {
    private var sessionId: String? = null
    private var protocolVersion: String? = null

    suspend fun callTool(
        tool: String,
        args: JsonObject,
    ): String {
        try {
            initialize()
            notifyInitialized()
            return toolText(tool, args)
        } finally {
            closeSession()
        }
    }

    private suspend fun initialize() {
        val params =
            buildJsonObject {
                put("protocolVersion", MCP_PROTOCOL_VERSION)
                putJsonObject("capabilities") {}
                putJsonObject("clientInfo") {
                    put("name", CLIENT_NAME)
                    put("version", CLIENT_VERSION)
                }
            }
        val result = resultOf(post(rpcRequest(ID_INITIALIZE, ReportTools.RPC_INITIALIZE, params), ID_INITIALIZE, captureSession = true))
        // サーバーが別の対応版へ下げるのは許容する。以降のリクエストはその版を名乗る。
        protocolVersion = result.stringOrNull("protocolVersion") ?: MCP_PROTOCOL_VERSION
    }

    private suspend fun notifyInitialized() {
        post(rpcNotification(ReportTools.RPC_INITIALIZED), expectId = null)
    }

    private suspend fun toolText(
        tool: String,
        args: JsonObject,
    ): String {
        val params =
            buildJsonObject {
                put("name", tool)
                put("arguments", args)
            }
        val result = resultOf(post(rpcRequest(ID_TOOLS_CALL, ReportTools.RPC_TOOLS_CALL, params), ID_TOOLS_CALL))
        val text = firstText(result)
        if (result.booleanOrFalse("isError")) {
            // ツールの失敗は `{"error": {"code": ...}}`（HTTP 200）。コードを分類する。本文は使わない。
            throw classify(null, HTTP_OK, errorCode(text)) ?: FetchException(ErrKind.Protocol)
        }
        if (text.isEmpty()) throw FetchException(ErrKind.Protocol)
        return text
    }

    /** セッション ID が返っていれば `DELETE` で閉じる。応答（405 を含む）・失敗は見ない。 */
    private suspend fun closeSession() {
        val id = sessionId ?: return
        runCatching {
            val request =
                Request
                    .Builder()
                    .url(endpoint)
                    .delete()
                    .header(HEADER_SESSION_ID, id)
                    .apply { protocolVersion?.let { header(HEADER_PROTOCOL_VERSION, it) } }
                    .build()
            newCall(request).executeAndRead { }
        }
    }

    /**
     * JSON-RPC を 1 回 `POST` する。[expectId] が null なら通知（応答の本文は見ない）。
     * 4xx/5xx は HTTP ステータスと `error.code` で分類した [FetchException]、3xx は protocol。
     */
    private suspend fun post(
        body: JsonObject,
        expectId: Int?,
        captureSession: Boolean = false,
    ): JsonObject? {
        val request =
            Request
                .Builder()
                .url(endpoint)
                .post(body.toString().toRequestBody(JSON_MEDIA_TYPE))
                .header("Accept", "application/json, text/event-stream")
                .apply {
                    protocolVersion?.let { header(HEADER_PROTOCOL_VERSION, it) }
                    sessionId?.let { header(HEADER_SESSION_ID, it) }
                }.build()
        return newCall(request).executeAndRead { response ->
            val status = response.code
            if (status >= HTTP_BAD_REQUEST) {
                throw classify(null, status, errorCode(response.errorSnippet())) ?: FetchException(ErrKind.Protocol, "http_$status")
            }
            if (status >= HTTP_MULTIPLE_CHOICES) throw FetchException(ErrKind.Protocol, "http_$status")
            if (captureSession) sessionId = response.header(HEADER_SESSION_ID)?.takeIf { it.isNotEmpty() }
            expectId?.let { readMessage(response, it) }
        }
    }

    private fun newCall(request: Request): Call {
        val remaining = deadline - System.nanoTime()
        if (remaining <= 0) throw FetchException(ErrKind.Unreachable)
        return client.newCall(request).also { it.timeout().timeout(remaining, TimeUnit.NANOSECONDS) }
    }
}

private const val MCP_PROTOCOL_VERSION = "2025-06-18"
private const val CLIENT_NAME = "swarm-viewer"
private const val CLIENT_VERSION = "0.1.0"
private const val ID_INITIALIZE = 1
private const val ID_TOOLS_CALL = 2
private const val HTTP_OK = 200
private const val HTTP_MULTIPLE_CHOICES = 300
private const val HTTP_BAD_REQUEST = 400
private const val HEADER_SESSION_ID = "Mcp-Session-Id"
private const val HEADER_PROTOCOL_VERSION = "MCP-Protocol-Version"

/**
 * 呼び出しをブロッキングで実行して [block] で応答を読み、閉じる。コルーチンのキャンセルで [Call.cancel] する。
 * [block] は応答を閉じる前（このスコープの中）で本文を読み切ること。
 */
internal suspend fun <T> Call.executeAndRead(block: (Response) -> T): T =
    withContext(Dispatchers.IO) {
        suspendCancellableCoroutine { continuation ->
            continuation.invokeOnCancellation { cancel() }
            try {
                continuation.resume(execute().use(block))
            } catch (e: Throwable) {
                continuation.resumeWithException(e)
            }
        }
    }
