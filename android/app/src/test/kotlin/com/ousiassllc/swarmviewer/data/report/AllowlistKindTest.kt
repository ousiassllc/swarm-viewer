package com.ousiassllc.swarmviewer.data.report

import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Before
import org.junit.Test

/** NFR-SEC-1: 送信内容の検査は URL の path で決まる。`/report` に操作ツールは出せず、`/ops` でも範囲外のツールは出せない。 */
class AllowlistKindTest {
    private lateinit var report: FakeConsole
    private lateinit var ops: FakeConsole

    @Before
    fun setUp() {
        report = FakeConsole(path = "/report").start()
        ops = FakeConsole(path = "/ops").start()
        report.tools = opsTools()
        ops.tools = opsTools()
    }

    @After
    fun tearDown() {
        report.stop()
        ops.stop()
    }

    private fun guarded(console: FakeConsole): OkHttpClient =
        newReportHttpClient().newBuilder().addInterceptor(GuardInterceptor(console.url, TEST_TOKEN)).build()

    private fun callTool(
        console: FakeConsole,
        tool: String,
    ) {
        val body = """{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"$tool","arguments":{}}}"""
        guarded(console)
            .newCall(
                Request
                    .Builder()
                    .url(console.url)
                    .post(body.toRequestBody(JSON_MEDIA_TYPE))
                    .build(),
            ).execute()
            .close()
    }

    @Test
    fun reportEndpointAcceptsOnlyTheFourViewTools() {
        for (tool in ReportTools.view) callTool(report, tool)
        assertEquals(ReportTools.view.toList().sorted(), report.seen.mapNotNull { it.tool }.sorted())

        for (tool in ReportTools.opsOnly) {
            assertThrows(tool, RequestRejectedException::class.java) { callTool(report, tool) }
        }
        assertEquals(ReportTools.view.size, report.seen.size) // 操作ツールは 1 件も送られていない
    }

    @Test
    fun opsEndpointAcceptsAllElevenTools() {
        for (tool in ReportTools.ops) callTool(ops, tool)

        assertEquals(ReportTools.ops.toList().sorted(), ops.seen.mapNotNull { it.tool }.sorted())
        assertEquals(11, ops.seen.size)
        assertEquals(0, report.seen.size)
    }

    @Test
    fun toolsOutOfScopeAreRejectedEvenOnTheOpsEndpoint() {
        val outOfScope = listOf("dispatch", "note_set", "note_log", "file_get", "panes_status", "host_status", "report_read")
        for (tool in outOfScope) {
            assertThrows(tool, RequestRejectedException::class.java) { callTool(ops, tool) }
        }
        val listBody = """{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}"""
        assertThrows(RequestRejectedException::class.java) {
            guarded(ops)
                .newCall(
                    Request
                        .Builder()
                        .url(ops.url)
                        .post(listBody.toRequestBody(JSON_MEDIA_TYPE))
                        .build(),
                ).execute()
        }
        assertEquals(0, ops.seen.size)
    }
}
