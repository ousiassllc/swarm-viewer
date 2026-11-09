package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.FirstTask
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

private const val VIEW = "view-conn"
private const val OPS = "ops-conn"

/** 新しいセッションの最初のタスク（FR-OPS-11）: `pane_start` → `send_text` を同じ排他の中で、`idle` のときだけ行う。 */
@OptIn(ExperimentalCoroutinesApi::class)
class PaneServiceFirstTaskTest {
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

    private fun ops(pos: String = "B1") = PaneTarget(OPS, "s0", pos)

    private suspend fun start(
        first: FirstTask,
        target: PaneTarget = ops(),
    ) = service.start(target, "omp", "/work/app", "t", first)

    @Test
    fun theFirstTaskIsSentAfterPaneStartOnlyWhenTheStateIsIdle() =
        runTest {
            val outcome = start(FirstTask("/impl", "#12 を直す"))

            assertEquals(listOf("pane_start s0 B1 omp /work/app t", "send_text s0 B1 /impl #12 を直す"), api.calls)
            assertEquals(FirstTaskStatus.Sent, outcome.task)
            assertTrue(outcome.start.ok)
        }

    @Test
    fun withoutAFirstTaskOnlyPaneStartIsCalled() =
        runTest {
            assertEquals(FirstTaskStatus.None, start(FirstTask("  ", "\n")).task)
            assertEquals(FirstTaskStatus.None, service.start(ops(), "omp", "/work/app", "t").task)

            assertEquals(2, api.calls.size)
            assertTrue(api.calls.all { it.startsWith("pane_start") })
        }

    @Test
    fun nothingIsSentWhenPaneStartIsNotOkOrTheStateIsNotIdle() =
        runTest {
            api.start = api.start.copy(ok = false, state = "idle")
            assertEquals(FirstTaskStatus.None, start(FirstTask(task = "x")).task)
            for (state in listOf("shell", "working", "asking", "stalled", "unknown", "weird")) {
                api.start = api.start.copy(ok = true, state = state)
                val outcome = start(FirstTask(task = "x"))
                assertEquals(state, FirstTaskStatus.NotIdle, outcome.task)
            }

            assertTrue(api.calls.none { it.startsWith("send_text") })
        }

    @Test
    fun aFailedPaneStartNeverSendsTheFirstTask() =
        runTest {
            api.failure = FetchException(ErrKind.Operation, "forbidden_path")
            try {
                start(FirstTask(task = "x"))
                throw AssertionError("FetchException が投げられなかった")
            } catch (e: FetchException) {
                assertEquals("forbidden_path", e.code)
            }

            assertEquals(listOf("pane_start s0 B1 omp /work/app t"), api.calls)
        }

    @Test
    fun aFailedSendKeepsTheStartAndNeverClosesThePane() =
        runTest {
            api.sendFailure = FetchException(ErrKind.Operation, "busy_timeout")
            val failed = start(FirstTask(task = "x"))

            assertEquals(FirstTaskStatus.Failed, failed.task)
            assertEquals("busy_timeout", failed.error?.code)
            assertTrue(failed.start.ok)

            api.sendFailure = null
            api.send = api.send.copy(accepted = false)
            val rejected = start(FirstTask(task = "x"))
            assertEquals(FirstTaskStatus.Failed, rejected.task)
            assertNull(rejected.error)
            assertEquals(false, rejected.send?.accepted)

            assertTrue(api.calls.none { it.startsWith("pane_close") })
        }

    @Test
    fun aFirstTaskProblemIsRefusedBeforePaneStart() =
        runTest {
            for ((first, code) in listOf(
                FirstTask(task = "a\u001bb") to "forbidden_control_char",
                FirstTask(task = "a\r\nb") to "forbidden_control_char",
                FirstTask(skill = "/impl", task = "あ".repeat(7995)) to "text_too_long",
            )) {
                try {
                    start(first)
                    throw AssertionError("FetchException が投げられなかった")
                } catch (e: FetchException) {
                    assertEquals(code, e.code)
                    assertEquals(ErrKind.Operation, e.kind)
                }
            }

            assertEquals(emptyList<String>(), api.calls)
        }

    @Test
    fun aViewOnlyConnectionNeverCommunicatesForStartResendOrSkills() =
        runTest {
            val view = PaneTarget(VIEW, "s0", "B1")
            val operations: List<suspend () -> Unit> =
                listOf(
                    { service.start(view, "omp", "/work", "t", FirstTask(task = "x")) },
                    { service.resend(view, FirstTask(task = "x")) },
                    { service.skills(VIEW, "omp") },
                )
            for (operation in operations) {
                try {
                    operation()
                    throw AssertionError("OpsRequiredException が投げられなかった")
                } catch (e: OpsRequiredException) {
                    // 期待どおり
                }
            }

            assertEquals(emptyList<String>(), api.calls)
        }

    @Test
    fun resendSendsOnlySendTextAndNeverStartsAgain() =
        runTest {
            val sent = service.resend(ops(), FirstTask("/impl", "本文"))

            assertTrue(sent.accepted)
            assertEquals(listOf("send_text s0 B1 /impl 本文"), api.calls)
        }

    @Test
    fun resendRefusesAnEmptyOrInvalidFirstTaskWithoutCommunicating() =
        runTest {
            for (first in listOf(FirstTask(), FirstTask(task = "a\u0000b"))) {
                try {
                    service.resend(ops(), first)
                    throw AssertionError("FetchException が投げられなかった")
                } catch (e: FetchException) {
                    assertEquals(ErrKind.Operation, e.kind)
                }
            }

            assertEquals(emptyList<String>(), api.calls)
        }

    @Test
    fun thePaneStaysExclusiveFromPaneStartThroughSendText() =
        runTest {
            api.gate = CompletableDeferred()
            val running = async { start(FirstTask(task = "x")) }
            runCurrent()

            try {
                service.send(ops(), "割り込み")
                throw AssertionError("PaneBusyException が投げられなかった")
            } catch (e: PaneBusyException) {
                // pane_start 中は同じペインの操作を通信せずに断る
            }
            try {
                service.resend(ops(), FirstTask(task = "x"))
                throw AssertionError("PaneBusyException が投げられなかった")
            } catch (e: PaneBusyException) {
                // 再送も同じ排他
            }
            assertEquals(listOf("pane_start s0 B1 omp /work/app t"), api.calls)

            api.gate?.complete(Unit)
            runCurrent()
            assertEquals(FirstTaskStatus.Sent, running.await().task)
            assertEquals(listOf("pane_start s0 B1 omp /work/app t", "send_text s0 B1 x"), api.calls)

            service.send(ops(), "解放後は送れる")
        }

    @Test
    fun skillsAreFetchedWithOnlyTheAgentAndDoNotTakeThePaneLock() =
        runTest {
            api.gate = CompletableDeferred()
            val running = async { start(FirstTask(task = "x")) }
            runCurrent()

            val skills = service.skills(OPS, "claude") // ペインの排他の対象外
            api.gate?.complete(Unit)
            runCurrent()
            running.await()

            assertEquals("claude", skills.agent)
            assertTrue("skill_list claude" in api.calls)
        }
}
