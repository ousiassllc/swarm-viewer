package com.ousiassllc.swarmviewer.data.report

import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Before
import org.junit.Test

/** NFR-SEC-1: 呼べるのは `sessions_list` と `report` だけ。許可リスト外の送信は、通信の前に止まる。 */
class AllowlistTest {
    private lateinit var console: FakeConsole
    private lateinit var client: OkHttpClient

    @Before
    fun setUp() {
        console = FakeConsole().start()
        console.tools = okTools()
        client = newReportHttpClient().newBuilder().addInterceptor(GuardInterceptor(console.url, TEST_TOKEN)).build()
    }

    @After
    fun tearDown() = console.stop()

    private fun post(
        body: String,
        url: String = console.url,
    ) {
        client
            .newCall(
                Request
                    .Builder()
                    .url(url)
                    .post(body.toRequestBody(JSON_MEDIA_TYPE))
                    .build(),
            ).execute()
            .close()
    }

    private fun rpc(
        method: String,
        params: String = "{}",
    ) = """{"jsonrpc":"2.0","id":1,"method":"$method","params":$params}"""

    @Test
    fun theAllowlistIsExactlyTheTwoTools() {
        assertEquals(setOf("sessions_list", "report"), ReportTools.allowed)
        assertEquals(setOf("initialize", "notifications/initialized", "tools/call"), ReportTools.allowedRpcMethods)
    }

    @Test
    fun allowedToolsPassAndCarryTheBearer() {
        post(rpc("tools/call", """{"name":"sessions_list","arguments":{}}"""))
        post(rpc("tools/call", """{"name":"report","arguments":{"format":"json","session":"0"}}"""))

        assertEquals(listOf("sessions_list", "report"), console.seen.map { it.tool })
        assertEquals(listOf("Bearer $TEST_TOKEN", "Bearer $TEST_TOKEN"), console.seen.map { it.auth })
    }

    @Test
    fun otherToolsAreRejectedBeforeAnythingIsSent() {
        for (name in listOf("tools_list", "ops_send", "dispatch", "pane_tail", "report_read", "Report", "report ", "")) {
            assertThrows(name, RequestRejectedException::class.java) { post(rpc("tools/call", """{"name":"$name","arguments":{}}""")) }
        }
        assertThrows(RequestRejectedException::class.java) { post(rpc("tools/call", """{"arguments":{}}""")) }
        assertThrows(RequestRejectedException::class.java) { post(rpc("tools/call", """{"name":["report"]}""")) }
        assertEquals(0, console.seen.size)
    }

    @Test
    fun otherJsonRpcMethodsAreRejected() {
        for (method in listOf("tools/list", "resources/list", "prompts/get", "ping", "server/discover", "")) {
            assertThrows(method, RequestRejectedException::class.java) { post(rpc(method)) }
        }
        // バッチ・壊れた JSON・オブジェクトでない本文・method の無い本文
        for (body in listOf("""[${rpc("tools/call")}]""", "not json", "null", "{}", """{"method":1}""")) {
            assertThrows(body, RequestRejectedException::class.java) { post(body) }
        }
        assertEquals(0, console.seen.size)
    }

    @Test
    fun onlyPostAndDeleteAreAllowed() {
        assertThrows(RequestRejectedException::class.java) {
            client
                .newCall(
                    Request
                        .Builder()
                        .url(console.url)
                        .get()
                        .build(),
                ).execute()
        }
        assertThrows(RequestRejectedException::class.java) {
            client
                .newCall(
                    Request
                        .Builder()
                        .url(console.url)
                        .put("{}".toRequestBody())
                        .build(),
                ).execute()
        }
        assertEquals(0, console.seen.size)
        client
            .newCall(
                Request
                    .Builder()
                    .url(console.url)
                    .delete()
                    .build(),
            ).execute()
            .close() // 終了通知
        assertEquals(listOf("DELETE"), console.seen.map { it.httpMethod })
    }

    @Test
    fun anOversizedBodyIsRejected() {
        val padding = "x".repeat(70 * 1024)
        assertThrows(
            RequestRejectedException::class.java,
        ) { post(rpc("tools/call", """{"name":"report","arguments":{"pad":"$padding"}}""")) }
        assertEquals(0, console.seen.size)
    }

    @Test
    fun onlyTheValidatedEndpointIsContacted() {
        val other = "http://127.0.0.1:${console.server.port}/ops"
        assertThrows(RequestRejectedException::class.java) { post(rpc("tools/call", """{"name":"report"}"""), other) }
        assertThrows(RequestRejectedException::class.java) {
            post(rpc("tools/call", """{"name":"report"}"""), "http://127.0.0.1:${console.server.port}/report?x=1")
        }
        assertEquals(0, console.seen.size)
    }

    @Test
    fun theRejectionDoesNotCarryTokenOrUrl() {
        val e = assertThrows(RequestRejectedException::class.java) { post(rpc("tools/list")) }
        val text = e.message.orEmpty() + GuardInterceptor(console.url, TEST_TOKEN).toString()
        assertEquals(false, text.contains(TEST_TOKEN) || text.contains("127.0.0.1"))
    }
}
