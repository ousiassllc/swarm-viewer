package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.FetchException
import org.junit.After
import org.junit.Before

internal const val CONN_ID = "conn-1"
internal const val AWAIT_SECONDS = 10L

internal abstract class ReportClientTestBase {
    protected lateinit var console: FakeConsole
    protected val logged = mutableListOf<String>()
    protected val logger = FetchLogger { id, tool, kind, code -> logged += "$id $tool $kind $code" }

    @Before
    fun setUp() {
        console = FakeConsole().start()
        console.tools = okTools()
    }

    @After
    fun tearDown() = console.stop()

    protected fun client(
        token: String? = TEST_TOKEN,
        timeoutMillis: Long = REQUEST_TIMEOUT_MILLIS,
    ) = ReportClient({ token }, timeoutMillis = timeoutMillis, logger = logger)

    protected fun connection() = Connection(id = CONN_ID, name = "stub", url = console.url)

    protected suspend fun failure(block: suspend () -> Unit): FetchException {
        try {
            block()
        } catch (e: FetchException) {
            return e
        }
        throw AssertionError("FetchException が投げられなかった")
    }
}
