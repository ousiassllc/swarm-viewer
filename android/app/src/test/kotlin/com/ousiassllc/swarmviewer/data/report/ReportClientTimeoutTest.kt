package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

private const val BASE_TIMEOUT_MILLIS = 500L
private const val TOOL_DELAY_MILLIS = 900L // 1 倍（500ms）は超え、3 倍（1500ms）・5 倍（2500ms）には収まる

/**
 * ツール別の通信期限（デスクトップ版の `toolTimeoutScale` と同じ倍率）。`ask_pick`・`send_text`・`send_escape` は 3 倍、
 * `pane_start`・`pane_close` は 5 倍、読み取りのツールは 1 倍のまま打ち切る。呼び出し全体の期限と OkHttp の 1 回の読み取りの期限の両方が対象。
 */
internal class ReportClientTimeoutTest : ReportClientTestBase() {
    override val consolePath = "/ops"

    private fun slowClient() = client(timeoutMillis = BASE_TIMEOUT_MILLIS)

    @Test
    fun theScalesAreThreeForPickSendEscapeFiveForStartCloseAndOneForReads() {
        for (tool in listOf("ask_pick", "send_text", "send_escape")) assertEquals(tool, 3, ReportTools.timeoutScale(tool))
        for (tool in listOf("pane_start", "pane_close")) assertEquals(tool, 5, ReportTools.timeoutScale(tool))
        for (tool in listOf(
            "sessions_list",
            "report",
            "pane_tail",
            "ask_card",
            "dir_list",
            "skill_list",
        )) {
            assertEquals(tool, 1, ReportTools.timeoutScale(tool))
        }
    }

    @Test
    fun operationToolsWaitForTheirScaledLimitAndSucceed() =
        runTest {
            console.tools = opsTools()
            console.toolDelayMillis = TOOL_DELAY_MILLIS
            val c = slowClient()
            val conn = connection()

            assertTrue(c.askPick(conn, "s0", "T1", 1).ok) // 3 倍
            assertTrue(c.sendText(conn, "s0", "T1", "x").ok)
            assertTrue(c.sendEscape(conn, "s0", "T1", confirm = true).ok)
            assertTrue(c.paneStart(conn, "s0", "B1", "omp", "/work", "t").ok) // 5 倍
            assertTrue(c.paneClose(conn, "s0", "B1").ok)
        }

    @Test
    fun readToolsAreCutOffAtTheBaseLimitWithoutAnyScale() =
        runTest {
            console.tools = opsTools()
            console.toolDelayMillis = TOOL_DELAY_MILLIS
            val c = slowClient()
            val conn = connection()
            val reads: List<suspend () -> Unit> =
                listOf(
                    { c.paneTail(conn, "s0", "T1") },
                    { c.askCard(conn, "s0", "T1") },
                    { c.dirList(conn) },
                    { c.skillList(conn, "omp") },
                    { c.sessions(conn) },
                    { c.report(conn, "s0") },
                )
            for ((index, read) in reads.withIndex()) {
                assertEquals("read $index", ErrKind.Unreachable, failure(read).kind)
            }
        }

    @Test
    fun anOperationStillFailsOnceItsScaledLimitIsExceeded() =
        runTest {
            console.tools = opsTools()
            console.toolDelayMillis = 2 * BASE_TIMEOUT_MILLIS * 3 // 3 倍の期限を超える遅さ

            val e = failure { slowClient().sendText(connection(), "s0", "T1", "x") }

            assertEquals(ErrKind.Unreachable, e.kind)
        }
}
