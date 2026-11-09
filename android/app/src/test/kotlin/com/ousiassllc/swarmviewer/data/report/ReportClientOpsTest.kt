package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.OPERATION_CODES
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** 操作の接続（URL の path が `/ops`）: 操作ツールが、期待どおりのツール名・引数・path・Bearer で偽の swarm-console に届く。 */
internal class ReportClientOpsTest : ReportClientTestBase() {
    override val consolePath = "/ops"

    private fun calls() = console.seen.filter { it.rpcMethod == "tools/call" }

    private fun args(tool: String): JsonObject = checkNotNull(calls().single { it.tool == tool }.arguments)

    @Test
    fun askPickIsSentWithTheOptionNumberOrTheCustomTextNeverBoth() =
        runTest {
            console.tools = opsTools()
            val byNumber = client().askPick(connection(), "s0", "T1", 2)
            val byText = client().askPick(connection(), "s0", "T1", "自分の答え")

            assertTrue(byNumber.ok && byNumber.cardClosed)
            assertEquals(2, byNumber.pickedN)
            assertTrue(byText.ok)
            assertEquals(
                listOf(
                    buildJsonObject {
                        put("session", "s0")
                        put("pane", "T1")
                        put("n", 2)
                    },
                    buildJsonObject {
                        put("session", "s0")
                        put("pane", "T1")
                        put("text", "自分の答え")
                    },
                ),
                calls().map { it.arguments },
            )
        }

    @Test
    fun everyOperationArrivesWithToolNameArgumentsOpsPathAndBearer() =
        runTest {
            console.tools = opsTools()
            val c = client()
            val conn = connection()

            c.sendText(conn, "s0", "T2", "go\ttab\nline")
            c.sendEscape(conn, "s0", "T2", confirm = true)
            val roots = c.dirList(conn)
            val children = c.dirList(conn, "/work")
            c.paneStart(conn, "s0", "B1", "claude", "/work/app", "task")
            c.paneClose(conn, "s0", "B1")
            val skills = c.skillList(conn, "claude")
            c.paneTail(conn, "s0", "T1", lines = 500)
            c.askCard(conn, "s0", "T1")

            assertEquals(
                listOf(
                    "send_text",
                    "send_escape",
                    "dir_list",
                    "dir_list",
                    "pane_start",
                    "pane_close",
                    "skill_list",
                    "pane_tail",
                    "ask_card",
                ),
                calls().map { it.tool },
            )
            assertTrue(calls().all { it.path == "/ops" && it.auth == "Bearer $TEST_TOKEN" })
            assertEquals(0, console.countAt("/report"))
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "T2")
                    put("text", "go\ttab\nline")
                },
                args("send_text"),
            )
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "T2")
                    put("confirm", true)
                },
                args("send_escape"),
            )
            assertEquals(
                listOf(buildJsonObject {}, buildJsonObject { put("path", "/work") }),
                calls().filter { it.tool == "dir_list" }.map { it.arguments },
            ) // session・limit は付けない
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "B1")
                    put("agent", "claude")
                    put("cwd", "/work/app")
                    put("title", "task")
                },
                args("pane_start"),
            )
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "B1")
                },
                args("pane_close"),
            )
            assertEquals(buildJsonObject { put("agent", "claude") }, args("skill_list")) // session も cwd も渡さない
            assertEquals(listOf("secret-skill-name"), skills.skills.map { it.name })
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "T1")
                    put("lines", 200) // 上限 200 に丸める。raw は付けない
                },
                args("pane_tail"),
            )
            assertEquals(
                buildJsonObject {
                    put("session", "s0")
                    put("pane", "T1")
                },
                args("ask_card"),
            )
            assertEquals("/work", children.path)
            assertTrue(roots.entries.first().git)
        }

    @Test
    fun theFolderPickerFlowGoesFromDirListToPaneStart() =
        runTest {
            console.tools = opsTools()
            val c = client()
            val listing = c.dirList(connection(), "/work")
            val chosen = listing.entries.first { it.git }
            val started = c.paneStart(connection(), "s0", "B1", "omp", chosen.path, "title")

            assertTrue(started.ok)
            assertEquals(listOf("dir_list", "pane_start"), calls().map { it.tool })
            assertEquals("/work/app", args("pane_start")["cwd"].toString().trim('"'))
        }

    @Test
    fun escapeWithoutConfirmationIsNeverSent() =
        runTest {
            console.tools = opsTools()
            val e = failure { client().sendEscape(connection(), "s0", "T1", confirm = false) }

            assertEquals(ErrKind.Operation, e.kind)
            assertEquals("confirm_required", e.code)
            assertEquals(0, console.seen.size)
        }

    @Test
    fun operationFailuresKeepTheirCodeAsAnOperationError() =
        runTest {
            for (code in OPERATION_CODES) {
                console.tools = { toolError(code) }
                val e = failure { client().askPick(connection(), "s0", "T1", 1) }
                assertEquals(code, ErrKind.Operation, e.kind)
                assertEquals(code, e.code)
            }
        }

    @Test
    fun invalidArgsIsSessionMissingOnlyWhenTheDetailsFieldIsSessionOtherwiseAnOperationError() =
        runTest {
            val send: suspend () -> Unit = { client().sendText(connection(), "s0", "T1", "x") }

            console.tools = { toolErrorField("invalid_args", "session") }
            assertEquals(ErrKind.SessionMissing, failure(send).kind)
            console.tools = { toolErrorField("invalid_args", "pane") }
            val pane = failure(send)
            assertEquals(ErrKind.Operation, pane.kind)
            assertEquals("invalid_args", pane.code)
            console.tools = { toolError("invalid_args") } // details.field なし
            assertEquals(ErrKind.Operation, failure(send).kind)
            console.tools = { toolErrorField("invalid_args", "Bad Field!") } // 形の合わない field は session とみなさない
            assertEquals(ErrKind.Operation, failure(send).kind)
        }

    @Test
    fun theSamePaneRuleAppliesToTheViewingToolsAndDirList() =
        runTest {
            val c = client()
            val reads: List<suspend () -> Unit> =
                listOf({ c.paneTail(connection(), "s0", "T1") }, { c.askCard(connection(), "s0", "T1") }, { c.dirList(connection(), "/x") })
            for ((index, read) in reads.withIndex()) {
                console.tools = { toolErrorField("invalid_args", "session") }
                assertEquals("read $index", ErrKind.SessionMissing, failure(read).kind)
                console.tools = { toolErrorField("invalid_args", "pane") }
                assertEquals("read $index", ErrKind.Operation, failure(read).kind)
                console.tools = { toolError("invalid_args") }
                assertEquals("read $index", ErrKind.Operation, failure(read).kind)
            }
        }

    @Test
    fun sessionsListAndReportKeepInvalidArgsAsSessionMissingWhateverTheField() =
        runTest {
            for (reply in listOf(
                toolError("invalid_args"),
                toolErrorField("invalid_args", "pane"),
                toolErrorField("invalid_args", "session"),
            )) {
                console.tools = { reply }
                assertEquals(ErrKind.SessionMissing, failure { client().sessions(connection()) }.kind)
                assertEquals(ErrKind.SessionMissing, failure { client().report(connection(), "s0") }.kind)
            }
        }

    @Test
    fun unknownCodesStayProtocolAndTheOtherMappingsAreUnchanged() =
        runTest {
            val send: suspend () -> Unit = { client().sendText(connection(), "s0", "T1", "x") }
            console.tools = { toolError("brand_new_code") }
            val unknown = failure(send)
            assertEquals(ErrKind.Protocol, unknown.kind)
            assertEquals("brand_new_code", unknown.code)
            console.tools = { toolError("session_forbidden") }
            assertEquals(ErrKind.SessionForbidden, failure(send).kind)
            console.tools = { toolError("tmux_error") }
            assertEquals(ErrKind.HostError, failure(send).kind)
        }

    @Test
    fun aTokenThatTheHostRefusesIsUnauthorizedAndNotAnOperationError() =
        runTest {
            console.tools = opsTools()
            val e = failure { client(token = "another-token").sendText(connection(), "s0", "T1", "x") }
            assertEquals(ErrKind.Unauthorized, e.kind)
        }

    @Test
    fun operationsLogNeitherTheTokenNorTheSentTextNorThePaneBody() =
        runTest {
            val secretText = "secret-instruction-do-not-leak"
            console.tools = opsTools()
            client().paneTail(connection(), "s0", "T1")
            client().sendText(connection(), "s0", "T1", secretText)
            console.tools = { toolError("invalid_state") }
            val failed = failure { client().sendText(connection(), "s0", "T1", secretText) }
            console.tools = { ToolReply("""{"text":"$PANE_BODY","truncated":"not-a-bool"}""") } // 想定外の形
            val malformed = failure { client().paneTail(connection(), "s0", "T1") }
            console.rawPostResponse = console.httpError(401, "unauthorized")
            val rejected = failure { client().sendText(connection(), "s0", "T1", secretText) }

            val everything = (logged + listOf(failed, malformed, rejected).map { it.message.orEmpty() }).joinToString("\n")
            for (forbidden in listOf(TEST_TOKEN, secretText, PANE_BODY, SECRET_MESSAGE, console.url, "127.0.0.1")) {
                assertFalse(forbidden, everything.contains(forbidden))
            }
            assertEquals(3, logged.size) // 失敗 3 件だけ。接続 ID・ツール名・種別・コード
            assertTrue(logged.all { it.startsWith("$CONN_ID ") })
        }
}
