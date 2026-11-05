package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.withContext
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.SocketPolicy
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.TimeUnit

/** 許可範囲外の URL・秘匿・キャンセルの扱い（接続先に何も送らない・何も漏らさない）。 */
internal class ReportClientSafetyTest : ReportClientTestBase() {
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
