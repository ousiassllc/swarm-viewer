package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** 見るだけの接続（URL の path が `/report`）: `pane_tail`・`ask_card` は呼べるが、操作ツールは通信せずに拒否する。`/ops` には 1 件も行かない。 */
internal class ReportClientViewOnlyTest : ReportClientTestBase() {
    @Test
    fun paneTailAndAskCardAreCalledOnTheReportPathAndNothingGoesToOps() =
        runTest {
            console.tools = opsTools()
            val tail = client().paneTail(connection(), "s0", "T1", lines = 30)
            val card = client().askCard(connection(), "s0", "T1")

            assertEquals(PANE_BODY, tail.text)
            assertTrue(card.asking)
            assertEquals(listOf("pane_tail", "ask_card"), console.seen.filter { it.rpcMethod == "tools/call" }.map { it.tool })
            assertEquals(0, console.countAt("/ops"))
            assertTrue(console.seen.all { it.path == "/report" })
        }

    @Test
    fun everyOperationToolIsRefusedWithoutAnyCommunication() =
        runTest {
            console.tools = opsTools()
            val c = client()
            val conn = connection()
            val calls: List<suspend () -> Unit> =
                listOf(
                    { c.askPick(conn, "s0", "T1", 1) },
                    { c.askPick(conn, "s0", "T1", "text") },
                    { c.sendText(conn, "s0", "T1", "text") },
                    { c.sendEscape(conn, "s0", "T1", confirm = true) },
                    { c.paneStart(conn, "s0", "T1", "omp", "/work", "t") },
                    { c.paneClose(conn, "s0", "T1") },
                    { c.dirList(conn) },
                    { c.skillList(conn, "omp") },
                )
            for ((index, call) in calls.withIndex()) {
                val e = failure(call)
                assertEquals("call $index", ErrKind.Protocol, e.kind)
                assertEquals("call $index", "tool_not_allowed", e.code)
            }

            assertEquals(0, console.seen.size)
            assertEquals(calls.size, logged.size) // 拒否も記録する（ツール名・種別・コードだけ）
            assertFalse(logged.joinToString().contains(TEST_TOKEN))
        }

    @Test
    fun theViewOnlyToolsAreExactlyTheFourAndTheOpsToolsAreElevenWithNothingOutOfScope() {
        assertEquals(setOf("sessions_list", "report", "pane_tail", "ask_card"), ReportTools.view)
        assertEquals(
            setOf("ask_pick", "send_text", "send_escape", "pane_start", "pane_close", "dir_list", "skill_list"),
            ReportTools.opsOnly,
        )
        assertEquals(11, ReportTools.ops.size)
        assertEquals(ReportTools.view + ReportTools.opsOnly, ReportTools.ops)
        val outOfScope = listOf("dispatch", "note_set", "note_log", "file_get", "panes_status", "host_status", "report_read", "tools/list")
        assertTrue(outOfScope.none { it in ReportTools.ops })
    }
}
