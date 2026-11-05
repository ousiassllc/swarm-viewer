package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.domain.Settings
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

private val NOW: Instant = Instant.parse("2026-11-05T02:00:00Z")

class AttentionCheckTest {
    private val store = FakeViewerStore()
    private val console = FakeConsole()
    private val notifier = RecordingNotifier()
    private val flags = FakeRepairFlags()
    private val check =
        AttentionCheck(
            store = store,
            lister = console.lister,
            reports = console.fetcher,
            observer = NotificationService(store, notifier),
            notifier = notifier,
            flags = flags,
            clock = Clock.fixed(NOW, ZoneOffset.UTC),
        )

    private fun addConnection(
        id: String,
        baselined: Boolean = true,
        vararg sessions: Session,
    ) {
        store.connectionRows[id] =
            Connection(
                id,
                "dev-$id",
                "http://100.64.0.${store.connectionRows.size + 1}:48120/report",
                baselinedAt = if (baselined) NOW else null,
            )
        console.sessions[id] = sessions.toList()
    }

    private fun asking(
        connection: String,
        session: String,
        paneId: String = "%1",
        state: PaneState = PaneState.Asking,
    ) {
        console.reports[connection to session] = reportOf(pane(paneId, state))
    }

    @Test
    fun everyConnectionAndAllowedSessionIsCheckedAndNewAttentionIsNotified() =
        runBlocking {
            addConnection("a", true, Session("s0", true), Session("hidden", false), Session("s1", true))
            addConnection("b", true, Session("t0", true))
            asking("a", "s0")
            asking("a", "s1", "%2", PaneState.Stalled)
            asking("b", "t0", "%3")

            check.run()

            assertEquals(
                listOf("a/s0" to PaneState.Asking, "a/s1" to PaneState.Stalled, "b/t0" to PaneState.Asking),
                notifier.notices.map { "${it.connectionId}/${it.session}" to it.state },
            )
            assertFalse(console.events.any { it.contains("hidden") }) // 許可されていないセッションは取得しない
        }

    @Test
    fun theChecksAreSequentialNeverParallel() =
        runBlocking {
            addConnection("a", true, Session("s0", true), Session("s1", true))
            addConnection("b", true, Session("t0", true), Session("t1", true))

            check.run()

            assertEquals(1, console.maxInFlight) // NFR-PERF-6
            assertEquals(
                listOf("sessions:a", "report:a/s0", "report:a/s1", "sessions:b", "report:b/t0", "report:b/t1"),
                console.events,
            )
        }

    @Test
    fun theSameStateIsNotNotifiedAgainOnTheNextRun() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            asking("a", "s0")

            check.run()
            check.run()

            assertEquals(1, notifier.notices.size)
        }

    @Test
    fun theFirstPassOfANewConnectionOnlyRecordsAndThenMarksItBaselined() =
        runBlocking {
            addConnection("a", false, Session("s0", true), Session("s1", true))
            asking("a", "s0")

            check.run()

            assertEquals(0, notifier.notices.size)
            assertEquals(1, store.notifiedRows.size) // 記録だけ作る
            assertEquals(NOW, store.connectionRows["a"]?.baselinedAt) // 全セッションが取れたので一巡が完了

            asking("a", "s1", "%7")
            store.connectionRows["a"] = store.connectionRows.getValue("a") // 一巡のあとに新しく現れたものは通常どおり通知する
            check.run()
            assertEquals(listOf("%7"), notifier.notices.map { it.paneId })
        }

    @Test
    fun aPartialFirstPassStaysUnbaselinedAndKeepsNotNotifying() =
        runBlocking {
            addConnection("a", false, Session("s0", true), Session("s1", true))
            asking("a", "s0")
            console.failures["a" to "s1"] = FetchException(ErrKind.Unreachable)

            check.run()

            assertNull(store.connectionRows["a"]?.baselinedAt) // 一部が取れないので未完了のまま
            assertEquals(0, notifier.notices.size)

            console.failures.clear()
            check.run() // 次の周期で続きを行う
            assertNotNull(store.connectionRows["a"]?.baselinedAt)
            assertEquals(0, notifier.notices.size)
        }

    @Test
    fun whileNotificationsAreOffNothingIsFetchedAndNothingChanges() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            asking("a", "s0")
            store.settingsRow = Settings(notificationsEnabled = false)

            check.run()

            assertEquals(emptyList<String>(), console.events) // オフの間は取得しない
            assertEquals(0, notifier.notices.size)
            assertEquals(0, store.notifiedRows.size)
        }

    @Test
    fun aFailedFetchIsSkippedWithoutChangingTheRecordsAndTheOthersContinue() =
        runBlocking {
            addConnection("a", true, Session("s0", true), Session("s1", true))
            addConnection("b", true, Session("t0", true))
            asking("a", "s0")
            asking("a", "s1", "%2")
            asking("b", "t0", "%3")
            store.notifiedRows += listOf("a", "s0", "%1", "asking") // 通知済み
            console.failures["a" to "s0"] = FetchException(ErrKind.Unreachable) // 取れなければ、ペインが消えたと誤認して記録を消さない

            check.run()

            assertTrue(store.notifiedRows.contains(listOf("a", "s0", "%1", "asking"))) // 失敗したセッションの記録は変えない
            assertEquals(listOf("a/s1", "b/t0"), notifier.notices.map { "${it.connectionId}/${it.session}" })
            assertEquals(0, notifier.repairs.size)
        }

    @Test
    fun aFailureOfTheSessionListSkipsThatConnectionOnly() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            addConnection("b", true, Session("t0", true))
            asking("a", "s0")
            asking("b", "t0", "%3")
            console.sessionFailures["a"] = FetchException(ErrKind.Unreachable)

            check.run()

            assertEquals(listOf("b/t0"), notifier.notices.map { "${it.connectionId}/${it.session}" })
        }

    @Test
    fun unauthorizedOrForbiddenFlagsTheConnectionNotifiesOnceAndStopsCheckingIt() =
        runBlocking {
            addConnection("a", true, Session("s0", true), Session("s1", true))
            addConnection("b", true, Session("t0", true))
            asking("b", "t0", "%3")
            console.failures["a" to "s0"] = FetchException(ErrKind.Unauthorized, "unauthorized")

            check.run()

            assertEquals(listOf("a"), notifier.repairs) // 再ペアリングを促す通知は 1 回
            assertTrue(flags.isFlagged("a"))
            assertFalse(console.events.contains("report:a/s1")) // その接続の残りは取得しない
            assertEquals(listOf("b/t0"), notifier.notices.map { "${it.connectionId}/${it.session}" }) // 他の接続は続ける

            console.events.clear()
            check.run()
            check.run()
            assertEquals(listOf("a"), notifier.repairs) // 何度確認しても、通知は増えない
            assertFalse(console.events.any { it.contains(":a") }) // 再ペアリングまで確認の対象から外れる

            flags.clear("a") // 再ペアリングの成功
            console.failures.clear()
            check.run()
            assertTrue(console.events.contains("report:a/s0"))
        }

    @Test
    fun forbiddenScopeOnTheSessionListIsHandledTheSameWay() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            console.sessionFailures["a"] = FetchException(ErrKind.ForbiddenScope, "forbidden_scope")

            check.run()
            check.run()

            assertEquals(listOf("a"), notifier.repairs)
            assertTrue(flags.isFlagged("a"))
        }

    @Test
    fun noNotificationOrRecordCarriesTokensUrlsOrPaneText() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            asking("a", "s0")
            console.failures.clear()

            check.run()

            val shown = notifier.notices.toString() + notifier.repairs + store.notifiedRows
            assertFalse(shown.contains("secret"))
            assertFalse(shown.contains("48120"))
        }

    @Test
    fun cancellationIsPropagatedAndNotSwallowed() =
        runBlocking {
            addConnection("a", true, Session("s0", true))
            val gate = CompletableDeferred<Unit>()
            val slow =
                AttentionCheck(
                    store,
                    console.lister,
                    { _, _ ->
                        gate.await()
                        Report()
                    },
                    NotificationService(store, notifier),
                    notifier,
                    flags,
                )

            val job = async(start = CoroutineStart.UNDISPATCHED) { slow.run() }
            delay(10)
            job.cancel()

            assertTrue(runCatching { job.await() }.exceptionOrNull() is kotlinx.coroutines.CancellationException)
        }
}
