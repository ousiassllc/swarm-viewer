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
import okhttp3.mockwebserver.SocketPolicy
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
    val path: String,
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
 * swarm-console を模した MockWebServer の応答。[path]（`/report` か `/ops`）は POST のみで GET・DELETE は 405。
 * 受けたリクエストの path を [Seen.path] に記録する（`/report` と `/ops` を区別する検査用）。[path] 以外への要求は 404。
 * Bearer が合わなければ 401（swarm-console と同じ形の JSON）。応答は単発 JSON か SSE。
 */
internal class FakeConsole(
    private val token: String = TEST_TOKEN,
    private val path: String = "/report",
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

    /** 応答のあと接続を閉じる（`Connection: close` は付けない。待機中の接続をサーバーが閉じた状況の再現）。 */
    @Volatile var closeAfterResponse = false

    /** すべての応答のヘッダーをこのミリ秒だけ遅らせる（合計時間の上限の検査用）。 */
    @Volatile var delayMillis = 0L

    /** tools/call の応答のヘッダーだけをこのミリ秒だけ遅らせる（ツール別の期限の検査用。initialize などは遅らせない）。 */
    @Volatile var toolDelayMillis = 0L

    val url: String get() = "http://127.0.0.1:${server.port}$path"

    /** 受けたリクエストのうち path が [wanted] のものの数。 */
    fun countAt(wanted: String): Int = seen.count { it.path == wanted }

    fun start(): FakeConsole {
        server.dispatcher = this
        server.start(InetAddress.getByName("127.0.0.1"), 0)
        return this
    }

    fun stop() = server.shutdown()

    /** 受けたリクエストの JSON-RPC メソッド（POST のみ、順に）。 */
    fun rpcMethods(): List<String> = seen.filter { it.httpMethod == "POST" }.mapNotNull { it.rpcMethod }

    override fun dispatch(request: RecordedRequest): MockResponse =
        answer(request).also {
            if (delayMillis > 0) it.setHeadersDelay(delayMillis, TimeUnit.MILLISECONDS)
            if (toolDelayMillis > 0 && lastRpcMethod(request) == "tools/call") it.setHeadersDelay(toolDelayMillis, TimeUnit.MILLISECONDS)
            if (closeAfterResponse) it.setSocketPolicy(SocketPolicy.DISCONNECT_AT_END)
        }

    private fun lastRpcMethod(request: RecordedRequest): String? = seen.lastOrNull()?.rpcMethod

    private fun answer(request: RecordedRequest): MockResponse {
        val body = request.body.readUtf8()
        val rpc = if (request.method == "POST") runCatching { Json.parseToJsonElement(body).jsonObject }.getOrNull() else null
        val params = rpc?.get("params") as? JsonObject
        firstRequest.countDown()
        seen +=
            Seen(
                httpMethod = request.method.orEmpty(),
                path = request.path.orEmpty(),
                auth = request.getHeader("Authorization"),
                sessionHeader = request.getHeader("Mcp-Session-Id"),
                protocolHeader = request.getHeader("MCP-Protocol-Version"),
                rpcMethod = (rpc?.get("method") as? JsonPrimitive)?.content,
                tool = (params?.get("name") as? JsonPrimitive)?.content,
                arguments = params?.get("arguments") as? JsonObject,
            )
        if (request.path != path) return httpError(404, "not_found")
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

/** `details.field` つきのツール失敗（`invalid_args` の分類の検査用）。 */
internal fun toolErrorField(
    code: String,
    field: String,
): ToolReply = ToolReply("""{"error":{"code":"$code","message":"$SECRET_MESSAGE","details":{"field":"$field"}}}""", isError = true)

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

/** 操作モードのツールの既定の応答（見るだけの 4 つと、操作の 7 つ）。 */
internal fun opsTools(): (String) -> ToolReply =
    { name ->
        when (name) {
            "pane_tail" -> ToolReply("""{"pane":"T1","lines":30,"text":"$PANE_BODY","truncated":false}""")
            "ask_card" ->
                ToolReply(
                    """{"asking":true,"question":"どうしますか？","options":[{"n":1,"label":"A","recommended":true},{"n":2,"label":"B"}],""" +
                        """"cursor":1,"allows_custom":true}""",
                )
            "ask_pick" -> ToolReply("""{"ok":true,"state":"working","picked":{"n":2,"label":"B"},"card_closed":true}""")
            "send_text" -> ToolReply("""{"ok":true,"state":"working","accepted":true,"chars":5}""")
            "send_escape" -> ToolReply("""{"ok":true,"state":"idle","interrupted":true}""")
            "pane_start" -> ToolReply("""{"ok":true,"state":"idle","agent":"omp","cwd":"/work/app","title":"t"}""")
            "pane_close" -> ToolReply("""{"ok":true,"state":"shell","note_archived":true}""")
            "dir_list" ->
                ToolReply(
                    """{"path":"/work","entries":[{"name":"app","path":"/work/app","git":true},{"name":"docs","path":"/work/docs"}],"truncated":false}""",
                )
            "skill_list" -> ToolReply(SKILL_LIST_BODY)
            else -> okTools()(name)
        }
    }

/** `skill_list` の既定の応答。スキル名・説明・invoke はログ・エラーに出てはいけない値。 */
internal const val SKILL_LIST_BODY =
    """{"agent":"omp","skills":[{"name":"secret-skill-name","description":"secret-skill-desc","invoke":"/secret-invoke"}],"truncated":false}"""

/** `pane_tail` の本文（ログ・エラーに出てはいけない値）。 */
internal const val PANE_BODY = "secret-pane-body-do-not-leak"
