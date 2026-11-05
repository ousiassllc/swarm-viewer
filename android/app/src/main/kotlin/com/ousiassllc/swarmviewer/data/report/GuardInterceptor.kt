package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.InvalidUrlException
import com.ousiassllc.swarmviewer.domain.validateUrl
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import okhttp3.Interceptor
import okhttp3.Response
import okio.Buffer
import java.io.IOException

/** 送信前の検査（URL・メソッド・許可リスト）で止めたとき。メッセージは固定で、URL・token・本文を含めない。 */
internal class RequestRejectedException : IOException("report: request rejected")

/** リクエスト本文の検査上限。viewer の引数は小さく、swarm-console 側の上限（256 KiB）より十分小さい。 */
private const val MAX_REQUEST_BYTES = 64L * 1024

/**
 * Bearer の付与・接続先の再検証・送信内容の許可リスト検査を行う（NFR-SEC-1・4・5）。1 回の呼び出しごとに作る。
 *  - URL は毎回 [validateUrl] を通し、呼び出し開始時の [endpoint]（検証済み・正規化済み）と一致するものだけ送る
 *  - メソッドは POST（JSON-RPC）と DELETE（セッション終了。応答は見ない）だけ。GET（常駐 SSE）は開かない
 *  - POST の本文は許可リストの JSON-RPC メソッド（`tools/call` は許可リストのツール）だけ。バッチ・不明なメソッドは通さない
 */
internal class GuardInterceptor(
    private val endpoint: String,
    private val token: String,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        checkUrl(request.url.toString())
        when (request.method) {
            "DELETE" -> Unit
            "POST" -> checkRpc(request.body)
            else -> throw RequestRejectedException()
        }
        return chain.proceed(request.newBuilder().header("Authorization", "Bearer $token").build())
    }

    private fun checkUrl(url: String) {
        val normalized =
            try {
                validateUrl(url)
            } catch (e: InvalidUrlException) {
                throw RequestRejectedException()
            }
        if (normalized != endpoint) throw RequestRejectedException()
    }

    // toString は token を含めない。
    override fun toString(): String = "GuardInterceptor"
}

private fun checkRpc(body: okhttp3.RequestBody?) {
    if (body == null || body.contentLength() !in 0L..MAX_REQUEST_BYTES) throw RequestRejectedException()
    val buffer = Buffer()
    body.writeTo(buffer)
    val message = rpcMessage(buffer.readUtf8())
    val method = (message["method"] as? JsonPrimitive)?.takeIf { it.isString }?.content
    if (method !in ReportTools.allowedRpcMethods) throw RequestRejectedException()
    if (method == ReportTools.RPC_TOOLS_CALL) {
        val name = ((message["params"] as? JsonObject)?.get("name") as? JsonPrimitive)?.contentOrNull
        if (name !in ReportTools.allowed) throw RequestRejectedException()
    }
}

private fun rpcMessage(text: String): JsonObject =
    try {
        Json.parseToJsonElement(text).jsonObject
    } catch (e: IllegalArgumentException) {
        // バッチ（配列）・壊れた JSON は通さない。例外の本文（入力の一部）は捨てる。
        throw RequestRejectedException()
    }
