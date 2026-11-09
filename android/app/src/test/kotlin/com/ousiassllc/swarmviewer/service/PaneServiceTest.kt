package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

private const val VIEW = "view-conn"
private const val OPS = "ops-conn"

/** [block] が [E] を投げることを確かめて、その例外を返す（`runTest` の中で中断関数を呼ぶ）。 */
private suspend inline fun <reified E : Throwable> thrown(block: suspend () -> Unit): E {
    try {
        block()
    } catch (e: Throwable) {
        if (e is E) return e
        throw e
    }
    throw AssertionError("${E::class.simpleName} が投げられなかった")
}

@OptIn(ExperimentalCoroutinesApi::class)
class PaneServiceTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val connections = ConnectionService(store, tokens, io = Dispatchers.Unconfined)
    private val api = FakePaneApi()
    private val service = PaneService(api, store, connections)

    init {
        store.connectionRows[VIEW] = Connection(VIEW, "view", "http://100.64.0.1:48120/report", createdAt = Instant.EPOCH)
        store.connectionRows[OPS] = Connection(OPS, "ops", "http://100.64.0.2:48120/ops", createdAt = Instant.EPOCH)
        tokens.put(VIEW, "test-token-not-secret")
        tokens.put(OPS, "test-token-not-secret")
    }

    private fun view(pos: String = "T1") = PaneTarget(VIEW, "s0", pos)

    private fun ops(pos: String = "T1") = PaneTarget(OPS, "s0", pos)

    @Test
    fun theConnectionKindComesFromTheUrlPath() =
        runTest {
            assertEquals(ConnectionKind.View, service.kind(VIEW))
            assertEquals(ConnectionKind.Ops, service.kind(OPS))
        }

    @Test
    fun aViewOnlyConnectionCanReadButEveryOperationIsRefusedWithoutCommunication() =
        runTest {
            service.tail(view())
            service.askCard(view())
            assertEquals(listOf("pane_tail s0 T1 30", "ask_card s0 T1"), api.calls)
            api.calls.clear()

            val operations: List<suspend () -> Unit> =
                listOf(
                    { service.pickOption(view(), 1) },
                    { service.pickText(view(), "x") },
                    { service.send(view(), "x") },
                    { service.escape(view(), confirm = true) },
                    { service.start(view("B1"), "omp", "/work", "t") },
                    { service.close(view()) },
                    { service.dirList(VIEW, null) },
                )
            for ((index, operation) in operations.withIndex()) {
                thrown<OpsRequiredException> { operation() }
            }

            assertEquals(emptyList<String>(), api.calls) // 通信は 0 件
        }

    @Test
    fun anOpsConnectionPassesEveryOperationToTheApiWithTheSessionAndPane() =
        runTest {
            service.pickOption(ops(), 2)
            service.pickText(ops(), "custom")
            service.send(ops(), "go")
            service.escape(ops(), confirm = true)
            service.start(ops("B1"), "claude", "/work/app", "  task  ")
            service.close(ops("B1"))
            service.dirList(OPS, "/work")

            assertEquals(
                listOf(
                    "ask_pick s0 T1 n=2",
                    "ask_pick s0 T1 text=custom",
                    "send_text s0 T1 go",
                    "send_escape s0 T1 confirm=true",
                    "pane_start s0 B1 claude /work/app task", // タイトルは前後の空白を除いて送る
                    "pane_close s0 B1",
                    "dir_list /work",
                ),
                api.calls,
            )
        }

    @Test
    fun escapeWithoutConfirmationIsRefusedBeforeTheApi() =
        runTest {
            val e = thrown<FetchException> { service.escape(ops(), confirm = false) }

            assertEquals(ErrKind.Operation, e.kind)
            assertEquals("confirm_required", e.code)
            assertEquals(emptyList<String>(), api.calls)
        }

    @Test
    fun aSecondOperationOnTheSamePaneIsRefusedWithoutCommunicationWhileTheFirstRuns() =
        runTest {
            api.gate = CompletableDeferred()
            val first = async { service.send(ops("T1"), "first") }
            runCurrent()
            assertEquals(listOf("send_text s0 T1 first"), api.calls)

            thrown<PaneBusyException> { service.pickOption(ops("T1"), 1) }
            thrown<PaneBusyException> { service.escape(ops("T1"), confirm = true) }
            assertEquals(listOf("send_text s0 T1 first"), api.calls) // 2 つ目・3 つ目は通信していない

            val otherPane = async { service.send(ops("T2"), "other") } // 別のペインは同時に動ける
            runCurrent()
            assertEquals(2, api.calls.size)

            api.gate?.complete(Unit)
            first.await()
            otherPane.await()
            service.send(ops("T1"), "again") // 終わったあとは、また送れる
            assertEquals("send_text s0 T1 again", api.calls.last())
        }

    @Test
    fun theLockIsReleasedWhenAnOperationFails() =
        runTest {
            api.failure = FetchException(ErrKind.Operation, "invalid_state")
            val e = thrown<FetchException> { service.send(ops(), "x") }
            assertEquals("invalid_state", e.code)

            api.failure = null
            service.send(ops(), "y")
            assertEquals(listOf("send_text s0 T1 x", "send_text s0 T1 y"), api.calls)
        }

    @Test
    fun textAndTitleAreCheckedBeforeAnythingIsSent() =
        runTest {
            suspend fun code(block: suspend () -> Unit): String = thrown<FetchException> { block() }.code

            assertEquals("invalid_args", code { service.send(ops(), "") })
            assertEquals("text_too_long", code { service.send(ops(), "あ".repeat(8001)) })
            assertEquals("forbidden_control_char", code { service.send(ops(), "a\u001bb") })
            assertEquals("forbidden_control_char", code { service.pickText(ops(), "a\rb") })
            assertEquals("unsupported_agent", code { service.start(ops(), "vim", "/work", "t") })
            assertEquals("invalid_args", code { service.start(ops(), "omp", "/work", "   ") })
            assertEquals("invalid_args", code { service.start(ops(), "omp", "/work", "a".repeat(81)) })
            assertEquals("invalid_args", code { service.start(ops(), "omp", "/work", "a\nb") })
            assertEquals("invalid_args", code { service.start(ops(), "omp", "", "t") })
            assertEquals(emptyList<String>(), api.calls)

            service.send(ops(), "改行\nタブ\tは通る" + "あ".repeat(7980)) // 8000 字ちょうどまでは送れる
            assertEquals(1, api.calls.size)
        }

    @Test
    fun anUnauthorizedResponseMarksTheConnectionForRepairAndStopsFurtherCommunication() =
        runTest {
            api.failure = FetchException(ErrKind.Unauthorized, "unauthorized")
            thrown<FetchException> { service.send(ops(), "x") }
            assertTrue(OPS in connections.needsRepair.value)

            api.calls.clear()
            api.failure = null
            val e = thrown<FetchException> { service.tail(ops()) }
            assertEquals(ErrKind.Unauthorized, e.kind)
            assertEquals(emptyList<String>(), api.calls) // 再ペアリングまで通信しない
        }

    @Test
    fun aMissingConnectionIsReported() =
        runTest {
            thrown<ConnectionGoneException> { service.tail(PaneTarget("gone", "s0", "T1")) }
        }
}
