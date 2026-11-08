package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.InvalidResponseException
import com.ousiassllc.swarmviewer.domain.parseJsonObject
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.put
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Response
import okio.BufferedSource

// MCP Streamable HTTP（JSON-RPC 2.0）の組み立てと読み取り。応答の本文は例外・ログに含めない（NFR-LOG-3）。

internal val JSON_MEDIA_TYPE = "application/json".toMediaType()

/** 応答本文の読み取り上限。超えたら protocol（壊れた応答）として扱う。 */
private const val MAX_RESPONSE_BYTES = 4L * 1024 * 1024

/** HTTP エラー本文から `error.code` を読むために見る先頭バイト数。 */
private const val ERROR_SNIPPET_BYTES = 4L * 1024

private const val SSE_DATA_PREFIX = "data:"

internal fun rpcRequest(
    id: Int,
    method: String,
    params: JsonObject,
): JsonObject =
    buildJsonObject {
        put("jsonrpc", "2.0")
        put("id", id)
        put("method", method)
        put("params", params)
    }

internal fun rpcNotification(method: String): JsonObject =
    buildJsonObject {
        put("jsonrpc", "2.0")
        put("method", method)
    }

/** JSON-RPC の応答から `result` を取り出す。`error` 応答・`result` 欠落は protocol。 */
internal fun resultOf(message: JsonObject?): JsonObject {
    if (message == null || "error" in message) throw FetchException(ErrKind.Protocol)
    return message["result"] as? JsonObject ?: throw FetchException(ErrKind.Protocol)
}

/** `tools/call` の結果の先頭のテキスト。無ければ空文字。 */
internal fun firstText(result: JsonObject): String =
    (result["content"] as? JsonArray)
        ?.firstNotNullOfOrNull { element ->
            (element as? JsonObject)?.takeIf { it.stringOrNull("type") == "text" }?.stringOrNull("text")
        }.orEmpty()

internal fun JsonObject.stringOrNull(key: String): String? = (get(key) as? JsonPrimitive)?.takeIf { it.isString }?.content

internal fun JsonObject.booleanOrFalse(key: String): Boolean = (get(key) as? JsonPrimitive)?.takeIf { !it.isString }?.booleanOrNull == true

/**
 * `{"error": {"code": "<code>"}}` の code を返す。形が違えば空文字。値は信頼できないため、分類側（`classify`）が検証する。
 */
internal fun errorCode(body: String): String =
    try {
        ((parseJsonObject(body)["error"] as? JsonObject)?.get("code") as? JsonPrimitive)?.takeIf { it.isString }?.contentOrNull.orEmpty()
    } catch (e: InvalidResponseException) {
        ""
    }

/** ツール失敗の本文 `{"error": {"details": {"field": "<name>"}}}` の `details.field`。形が違うときは空文字（判定にだけ使う）。 */
internal fun errorField(body: String): String =
    try {
        (
            (((parseJsonObject(body)["error"] as? JsonObject)?.get("details") as? JsonObject)?.get("field")) as? JsonPrimitive
        )?.takeIf { it.isString }?.contentOrNull.orEmpty()
    } catch (e: InvalidResponseException) {
        ""
    }

/** HTTP エラー本文の先頭だけを読む（`error.code` 用）。 */
internal fun Response.errorSnippet(): String {
    val source = body.source()
    source.request(ERROR_SNIPPET_BYTES)
    return source.buffer.readUtf8(minOf(source.buffer.size, ERROR_SNIPPET_BYTES))
}

/**
 * 2xx 応答の本文から、[expectId] に対する JSON-RPC 応答を読む。
 * `application/json` は単発の応答、`text/event-stream` は一致するイベントが来た時点で読み終える。
 * どちらでもない・一致する応答が無い・上限超えは protocol（[FetchException]）。
 */
internal fun readMessage(
    response: Response,
    expectId: Int,
): JsonObject {
    val type = response.body.contentType()
    val source = response.body.source()
    val message =
        when {
            type?.type == "text" && type.subtype == "event-stream" -> readEventStream(source, expectId)
            type?.type == "application" && type.subtype == "json" -> readJsonBody(source)?.takeIf { isResponseTo(it, expectId) }
            else -> null
        }
    return message ?: throw FetchException(ErrKind.Protocol)
}

private fun readJsonBody(source: BufferedSource): JsonObject? {
    source.request(MAX_RESPONSE_BYTES + 1)
    if (source.buffer.size > MAX_RESPONSE_BYTES) return null
    return parseOrNull(source.readUtf8())
}

// SSE: 空行でイベントが区切られる。`data:` の行だけを使い（複数行は改行で連結）、ほかのフィールド・コメントは無視する。
private fun readEventStream(
    source: BufferedSource,
    expectId: Int,
): JsonObject? {
    val data = StringBuilder()
    var total = 0L

    fun dispatch(): JsonObject? {
        val message = if (data.isEmpty()) null else parseOrNull(data.toString())
        data.setLength(0)
        return message?.takeIf { isResponseTo(it, expectId) }
    }
    while (true) {
        val line = source.readUtf8Line() ?: return dispatch()
        total += line.length + 1
        if (total > MAX_RESPONSE_BYTES) return null
        when {
            line.isEmpty() -> dispatch()?.let { return it }
            line.startsWith(SSE_DATA_PREFIX) -> {
                if (data.isNotEmpty()) data.append('\n')
                data.append(line.removePrefix(SSE_DATA_PREFIX).removePrefix(" "))
            }
        }
    }
}

private fun parseOrNull(text: String): JsonObject? =
    try {
        parseJsonObject(text)
    } catch (e: InvalidResponseException) {
        null
    }

// id が一致し、result か error を持つ（サーバーからの通知・要求ではない）メッセージか。
private fun isResponseTo(
    message: JsonObject,
    expectId: Int,
): Boolean {
    val id = (message["id"] as? JsonPrimitive)?.takeIf { !it.isString }?.content
    return id == expectId.toString() && ("result" in message || "error" in message)
}
