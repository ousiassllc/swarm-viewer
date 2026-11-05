package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.classify
import kotlinx.coroutines.Dispatchers
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
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** 1 回のツール呼び出しの間だけ生きる MCP のやり取り。全体の期限 [deadline]（System.nanoTime 基準）を共有する。 */
internal class McpExchange(
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
