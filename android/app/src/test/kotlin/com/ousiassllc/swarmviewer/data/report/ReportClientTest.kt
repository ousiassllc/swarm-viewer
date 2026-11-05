package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.util.concurrent.TimeUnit

private const val CONN_ID = "conn-1"
private const val SHORT_TIMEOUT_MILLIS = 300L
private const val TIMEOUT_BOUND_MILLIS = 5_000L
private const val STEP_DELAY_MILLIS = 150L
private const val AWAIT_SECONDS = 10L

class ReportClientTest {
    private lateinit var console: FakeConsole
    private val logged = mutableListOf<String>()
    private val logger = FetchLogger { id, tool, kind, code -> logged += "$id $tool $kind $code" }

    @Before
    fun setUp() {
        console = FakeConsole().start()
        console.tools = okTools()
    }

    @After
    fun tearDown() = console.stop()

    private fun client(
        token: String? = TEST_TOKEN,
        timeoutMillis: Long = REQUEST_TIMEOUT_MILLIS,
    ) = ReportClient({ token }, timeoutMillis = timeoutMillis, logger = logger)

    private fun connection() = Connection(id = CONN_ID, name = "stub", url = console.url)

    private suspend fun failure(block: suspend () -> Unit): FetchException {
        try {
            block()
        } catch (e: FetchException) {
            return e
        }
        throw AssertionError("FetchException が投げられなかった")
    }

    @Test
    fun sessionsReturnsAllowedOnesAndSendsTheHandshakeWithBearer() =
        runTest {
            val sessions = client().sessions(connection())

            assertEquals(listOf("0", "2"), sessions.map { it.id })
            assertEquals(listOf("initialize", "notifications/initialized", "tools/call"), console.rpcMethods())
            assertTrue(console.seen.all { it.auth == "Bearer $TEST_TOKEN" })
            val call = console.seen.last()
            assertEquals("sessions_list", call.tool)
            assertEquals(0, call.arguments?.size)
            assertEquals("2025-06-18", console.seen[1].protocolHeader)
        }

    @Test
    fun reportOverSseSendsSessionAndFormatExplicitly() =
        runTest {
            console.sse = true
            val report = client().report(connection(), "1")

            assertEquals(listOf("%1", "%2"), report.panes.map { it.id })
            assertEquals(PaneState.Asking, report.panes[0].state)
            val call = console.seen.last()
            assertEquals("report", call.tool)
            assertEquals(JsonPrimitive("json"), call.arguments?.get("format"))
            assertEquals(JsonPrimitive("1"), call.arguments?.get("session"))
        }

    @Test
    fun reportWithEmptySessionFailsWithoutAnyRequest() =
        runTest {
            val e = failure { client().report(connection(), "") }

            assertEquals(ErrKind.SessionRequired, e.kind)
            assertEquals(0, console.seen.size)
        }

    @Test
    fun probeUsesTheGivenTokenBeforeAnythingIsSaved() =
        runTest {
            val sessions = ReportClient({ error("probe は token の保管先を引かない") }).probe(console.url, TEST_TOKEN)

            assertEquals(listOf("0", "2"), sessions.map { it.id })
            assertTrue(console.seen.all { it.auth == "Bearer $TEST_TOKEN" })
        }

    @Test
    fun onlyPostIsSentWhenTheServerKeepsNoSession() =
        runTest {
            client().sessions(connection())

            assertTrue(console.seen.all { it.httpMethod == "POST" })
        }

    @Test
    fun sessionIdIsEchoedAndClosedWithDeleteWhoseRefusalIsNotAFailure() =
        runTest {
            console.sessionId = "sid-1"

            val sessions = client().sessions(connection())

            assertEquals(2, sessions.size)
            assertEquals(listOf(null, "sid-1", "sid-1", "sid-1"), console.seen.map { it.sessionHeader })
            val last = console.seen.last()
            assertEquals("DELETE", last.httpMethod) // 405 が返っても失敗にしない
            assertEquals("Bearer $TEST_TOKEN", last.auth)
            assertTrue(logged.isEmpty())
        }

    @Test
    fun unauthorizedAndForbiddenAreClassifiedFromTheHttpStatus() =
        runTest {
            console.rawPostResponse = console.httpError(401, "unauthorized")
            assertEquals(ErrKind.Unauthorized, failure { client().sessions(connection()) }.kind)

            console.rawPostResponse = console.httpError(403, "forbidden_scope")
            assertEquals(ErrKind.ForbiddenScope, failure { client().sessions(connection()) }.kind)
        }

    @Test
    fun aWrongTokenIsRejectedByTheServerAs401() =
        runTest {
            val e = failure { client(token = "other-token").sessions(connection()) }

            assertEquals(ErrKind.Unauthorized, e.kind)
            assertEquals(1, console.seen.size) // 失敗した initialize 以降は送らない
        }

    @Test
    fun missingTokenFailsAsUnauthorizedWithoutAnyRequest() =
        runTest {
            val e = failure { client(token = null).sessions(connection()) }

            assertEquals(ErrKind.Unauthorized, e.kind)
            assertEquals("token_missing", e.code)
            assertEquals(0, console.seen.size)
        }

    @Test
    fun notFoundMethodNotAllowedAndTooLargeAreProtocol() =
        runTest {
            for (status in listOf(404, 405, 413, 500)) {
                console.rawPostResponse = MockResponse().setResponseCode(status).setBody("not json")
                val e = failure { client().sessions(connection()) }
                assertEquals("$status", ErrKind.Protocol, e.kind)
                assertEquals("$status", "http_$status", e.code)
            }
        }

    @Test
    fun redirectsAreNotFollowed() =
        runTest {
            console.rawPostResponse = MockResponse().setResponseCode(302).addHeader("Location", "http://127.0.0.1:1/elsewhere")

            val e = failure { client().sessions(connection()) }

            assertEquals(ErrKind.Protocol, e.kind)
            assertEquals("http_302", e.code)
            assertEquals(1, console.seen.size)
        }

    @Test
    fun toolErrorsAreClassifiedFromTheErrorCode() =
        runTest {
            val cases =
                mapOf(
                    "session_forbidden" to ErrKind.SessionForbidden,
                    "invalid_args" to ErrKind.SessionMissing,
                    "tmux_error" to ErrKind.HostError,
                    "internal" to ErrKind.HostError,
                    "something_new" to ErrKind.Protocol,
                )
            for ((code, kind) in cases) {
                console.tools = { toolError(code) }
                val e = failure { client().report(connection(), "0") }
                assertEquals(code, kind, e.kind)
                assertEquals(code, e.code)
            }
        }

    @Test
    fun malformedToolOutputAndJsonRpcErrorsAreProtocol() =
        runTest {
            console.tools = { ToolReply("not json") }
            assertEquals(ErrKind.Protocol, failure { client().report(connection(), "0") }.kind)

            console.tools = { ToolReply("") }
            assertEquals(ErrKind.Protocol, failure { client().sessions(connection()) }.kind)

            console.rpcOverride = """{"jsonrpc":"2.0","id":2,"error":{"code":-32602,"message":"$SECRET_MESSAGE"}}"""
            assertEquals(ErrKind.Protocol, failure { client().sessions(connection()) }.kind)
        }

    @Test
    fun noResponseTimesOutAsUnreachableWithinTheBound() =
        runTest {
            console.rawPostResponse = MockResponse().setSocketPolicy(SocketPolicy.NO_RESPONSE)
            val start = System.nanoTime()

            val e = failure { client(timeoutMillis = SHORT_TIMEOUT_MILLIS).sessions(connection()) }

            assertEquals(ErrKind.Unreachable, e.kind)
            assertTrue((System.nanoTime() - start) / 1_000_000 < TIMEOUT_BOUND_MILLIS)
        }

    @Test
    fun theTimeoutCoversTheWholeExchangeNotEachRequest() =
        runTest {
            console.delayMillis = STEP_DELAY_MILLIS // 1 回ごとは上限内でも、3 回の合計は 300 ミリ秒を超える

            val e = failure { client(timeoutMillis = SHORT_TIMEOUT_MILLIS).sessions(connection()) }

            assertEquals(ErrKind.Unreachable, e.kind)
        }

    @Test
    fun connectionRefusedIsUnreachable() =
        runTest {
            val closed = console.url
            console.stop()

            val e = failure { client().sessions(Connection(CONN_ID, "stub", closed)) }

            assertEquals(ErrKind.Unreachable, e.kind)
        }

    @Test
    fun urlsOutsideTheAllowedRangeAreNeverContacted() =
        runTest {
            for (url in listOf("http://192.168.0.1:48120/report", "http://devbox.ts.net:48120/report", "https://100.64.0.5:48120/report")) {
                val e = failure { client().sessions(Connection(CONN_ID, "stub", url)) }
                assertEquals(url, ErrKind.Protocol, e.kind)
                assertEquals(url, "invalid_url", e.code)
            }
            assertEquals(0, console.seen.size)
        }

    @Test
    fun failuresNeverLeakTokenUrlOrServerMessages() =
        runTest {
            val errors = mutableListOf<FetchException>()
            console.rawPostResponse = console.httpError(401, "unauthorized")
            errors += failure { client().sessions(connection()) }
            console.rawPostResponse = null
            console.tools = { toolError("tmux_error") }
            errors += failure { client().report(connection(), "0") }
            console.tools = { ToolReply("""{"layout":"$SECRET_MESSAGE"}""") } // 型が想定外の応答
            errors += failure { client().report(connection(), "0") }

            val everything = (errors.map { it.message.orEmpty() } + logged).joinToString("\n")
            for (forbidden in listOf(TEST_TOKEN, console.url, "127.0.0.1", SECRET_MESSAGE)) {
                assertFalse(forbidden, everything.contains(forbidden))
            }
            assertEquals(3, logged.size)
        }

    @Test
    fun cancellationIsNotSwallowedAsAFetchError() =
        runTest {
            console.rawPostResponse = MockResponse().setSocketPolicy(SocketPolicy.NO_RESPONSE)
            var unexpected: Throwable? = null
            val handler = CoroutineExceptionHandler { _, e -> unexpected = e }
            val job = launch(Dispatchers.Default + handler) { client().sessions(connection()) }
            assertTrue(withContext(Dispatchers.IO) { console.firstRequest.await(AWAIT_SECONDS, TimeUnit.SECONDS) })

            job.cancelAndJoin()

            assertTrue(job.isCancelled)
            assertNull(unexpected) // 取得失敗（FetchException）として伝わらない
        }
}
