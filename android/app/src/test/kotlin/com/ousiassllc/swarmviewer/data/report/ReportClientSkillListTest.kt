package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** `skill_list`（仮称。swarm-console 側は未実装）: 通信前の検査・古いホスト（`unknown_tool`）・スキル名などをログに出さないこと。 */
internal class ReportClientSkillListTest : ReportClientTestBase() {
    override val consolePath = "/ops"

    @Test
    fun anInvalidAgentIsRejectedBeforeCommunicating() =
        runTest {
            console.tools = opsTools()
            for (agent in listOf("", "vim", "OMP", "omp ")) {
                val e = failure { client().skillList(connection(), agent) }
                assertEquals(agent, ErrKind.Operation, e.kind)
                assertEquals(agent, "invalid_args", e.code)
            }

            assertEquals(0, console.seen.size)
        }

    @Test
    fun unknownToolKeepsItsCodeAsAnOperationErrorAndNothingLeaks() =
        runTest {
            console.tools = { toolError("unknown_tool") }
            val e = failure { client().skillList(connection(), "claude") }

            assertEquals(ErrKind.Operation, e.kind)
            assertEquals("unknown_tool", e.code)
            assertFalse(e.message.orEmpty().contains(SECRET_MESSAGE))
            assertFalse(logged.joinToString().contains(SECRET_MESSAGE))
        }

    @Test
    fun skillNamesDescriptionsAndInvokeAreNotLoggedAndTheResultIsParsed() =
        runTest {
            console.tools = opsTools()
            val list = client().skillList(connection(), "omp")

            assertEquals("omp", list.agent)
            assertEquals("/secret-invoke", list.skills.single().invoke)
            val everything = logged.joinToString("\n")
            for (secret in listOf(
                "secret-skill-name",
                "secret-skill-desc",
                "/secret-invoke",
            )) {
                assertFalse(secret, everything.contains(secret))
            }
            assertTrue(logged.isEmpty()) // 成功は記録しない
        }
}
