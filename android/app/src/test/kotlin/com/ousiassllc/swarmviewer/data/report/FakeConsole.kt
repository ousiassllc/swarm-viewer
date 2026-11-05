package com.ousiassllc.swarmviewer.data.report

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonObject
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import java.net.InetAddress
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

internal const val TEST_TOKEN = "test-token-not-secret"

/** 本文・メッセージに出てはいけない値（エラー・ログへの混入検査用）。 */
internal const val SECRET_MESSAGE = "secret-message-body-do-not-leak"

/** swarm-console が受けた 1 リクエスト。 */
internal data class Seen(
    val httpMethod: String,
    val auth: String?,
    val sessionHeader: String?,
    val protocolHeader: String?,
    val rpcMethod: String?,
    val tool: String?,
    val arguments: JsonObject?,
)

/** ツール 1 回の結果。[text] は `content[0].text`。 */
internal data class ToolReply(
    val text: String,
    val isError: Boolean = false,
)

/**
 * swarm-console を模した MockWebServer の応答。`/report` は POST のみで GET・DELETE は 405。
 * Bearer が合わなければ 401（swarm-console と同じ形の JSON）。応答は単発 JSON か SSE。
 */
internal class FakeConsole(
    private val token: String = TEST_TOKEN,
) : Dispatcher() {
    val server = MockWebServer()
    val seen = CopyOnWriteArrayList<Seen>()

    /** 最初のリクエストを受けたら 0 になる（キャンセルの検査で、通信が始まるのを待つため）。 */
    val firstRequest = CountDownLatch(1)

    /** initialize の応答に付ける `Mcp-Session-Id`。null なら付けない（ステートレス）。 */
    @Volatile var sessionId: String? = null

    /** 応答を SSE（text/event-stream）で返す。 */
    @Volatile var sse = false

    /** tools/call の結果。ツール名を受けて返す。 */
    @Volatile var tools: (String) -> ToolReply = { ToolReply("{}") }

    /** null でなければ、すべての POST にこの応答を返す（HTTP レベルの失敗の再現）。 */
    @Volatile var rawPostResponse: MockResponse? = null

    /** null でなければ、tools/call の本文をこの JSON-RPC 応答（id 2）にする。 */
    @Volatile var rpcOverride: String? = null

    /** すべての応答のヘッダーをこのミリ秒だけ遅らせる（合計時間の上限の検査用）。 */
    @Volatile var delayMillis = 0L

    val url: String get() = "http://127.0.0.1:${server.port}/report"

    fun start(): FakeConsole {
        server.dispatcher = this
        server.start(InetAddress.getByName("127.0.0.1"), 0)
        return this
    }

    fun stop() = server.shutdown()

    /** 受けたリクエストの JSON-RPC メソッド（POST のみ、順に）。 */
    fun rpcMethods(): List<String> = seen.filter { it.httpMethod == "POST" }.mapNotNull { it.rpcMethod }

    override fun dispatch(request: RecordedRequest): MockResponse =
        answer(request).also { if (delayMillis > 0) it.setHeadersDelay(delayMillis, TimeUnit.MILLISECONDS) }

    private fun answer(request: RecordedRequest): MockResponse {
        val body = request.body.readUtf8()
        val rpc = if (request.method == "POST") runCatching { Json.parseToJsonElement(body).jsonObject }.getOrNull() else null
        val params = rpc?.get("params") as? JsonObject
        firstRequest.countDown()
        seen +=
            Seen(
                httpMethod = request.method.orEmpty(),
                auth = request.getHeader("Authorization"),
                sessionHeader = request.getHeader("Mcp-Session-Id"),
                protocolHeader = request.getHeader("MCP-Protocol-Version"),
                rpcMethod = (rpc?.get("method") as? JsonPrimitive)?.content,
                tool = (params?.get("name") as? JsonPrimitive)?.content,
                arguments = params?.get("arguments") as? JsonObject,
            )
        if (request.method != "POST") return httpError(405, "method_not_allowed")
        if (request.getHeader("Authorization") != "Bearer $token") return httpError(401, "unauthorized")
        rawPostResponse?.let { return it }
        return when ((rpc?.get("method") as? JsonPrimitive)?.content) {
            "initialize" -> rpcResponse(1, initializeResult(), withSession = true)
            "notifications/initialized" -> MockResponse().setResponseCode(202)
            "tools/call" -> {
                rpcOverride?.let { return respond(it) }
                val reply = tools((params?.get("name") as? JsonPrimitive)?.content.orEmpty())
                rpcResponse(2, toolResult(reply))
            }
            else -> httpError(400, "invalid_args")
        }
    }

    private fun initializeResult(): JsonObject =
        buildJsonObject {
            put("protocolVersion", "2025-06-18")
            putJsonObject("capabilities") { putJsonObject("tools") {} }
            putJsonObject("serverInfo") {
                put("name", "stub-console")
                put("version", "0")
            }
        }

    private fun toolResult(reply: ToolReply): JsonObject =
        buildJsonObject {
            put(
                "content",
                kotlinx.serialization.json.buildJsonArray {
                    add(
                        buildJsonObject {
                            put("type", "text")
                            put("text", reply.text)
                        },
                    )
                },
            )
            put("isError", reply.isError)
        }

    private fun rpcResponse(
        id: Int,
        result: JsonObject,
        withSession: Boolean = false,
    ): MockResponse {
        val message =
            buildJsonObject {
                put("jsonrpc", "2.0")
                put("id", id)
                put("result", result)
            }
        return respond(message.toString()).also { response ->
            sessionId.takeIf { withSession }?.let { response.addHeader("Mcp-Session-Id", it) }
        }
    }

    /** JSON-RPC の応答本文 [message] を、設定に応じて単発 JSON か SSE で返す。 */
    fun respond(message: String): MockResponse =
        if (sse) {
            MockResponse()
                .addHeader("Content-Type", "text/event-stream")
                .setBody("event: message\ndata: $message\n\n")
        } else {
            MockResponse().addHeader("Content-Type", "application/json").setBody(message)
        }

    fun httpError(
        status: Int,
        code: String,
    ): MockResponse =
        MockResponse()
            .setResponseCode(status)
            .addHeader("Content-Type", "application/json")
            .setBody("""{"error":{"code":"$code","message":"$SECRET_MESSAGE"}}""")
}

internal fun toolError(code: String): ToolReply =
    ToolReply("""{"error":{"code":"$code","message":"$SECRET_MESSAGE","details":{"allowed_count":2}}}""", isError = true)

internal const val SESSIONS_JSON =
    """{"sessions":[{"id":"0","allowed":true,"operable":false},{"id":"1","allowed":false},{"id":"2","allowed":true}]}"""

internal const val REPORT_JSON =
    """{"at":"2026-11-03T10:00:00Z","layout":{"rows":1,"cols":[2]},""" +
        """"panes":[{"pos":"T1","id":"%1","agent":"omp","state":"asking","title":"t","cwd":"/w"},""" +
        """{"pos":"T2","id":"%2","agent":null,"state":"idle","title":"u","cwd":"/w"}],"attention":[],"future_key":{"x":1}}"""

internal fun okTools(): (String) -> ToolReply =
    { name ->
        when (name) {
            "sessions_list" -> ToolReply(SESSIONS_JSON)
            "report" -> ToolReply(REPORT_JSON)
            else -> toolError("invalid_args")
        }
    }
