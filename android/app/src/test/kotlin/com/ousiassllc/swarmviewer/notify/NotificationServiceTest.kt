package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Settings
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

private val AT: Instant = Instant.parse("2026-11-05T01:00:00Z")

class NotificationServiceTest {
    private val store = FakeViewerStore()
    private val notifier = RecordingNotifier()
    private val service = NotificationService(store, notifier)
    private val connection = Connection("c1", "dev", "http://100.64.0.1:48120/report", baselinedAt = AT)

    private fun recorded() = store.notifiedRows.map { it.drop(2) }.toSet() // ペイン・状態

    @Test
    fun aTransitionToAskingOrStalledIsNotifiedWithOnlyNamesAndTheStateLabel() =
        runBlocking {
            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Asking, "T1"), pane("%2", PaneState.Stalled, "B1")), AT)

            assertEquals(2, notifier.notices.size)
            assertEquals(
                AttentionNotice("c1", "dev", "s0", "%1", "T1", PaneState.Asking),
                notifier.notices[0],
            )
            assertEquals(PaneState.Stalled, notifier.notices[1].state)
            assertEquals(setOf(listOf("%1", "asking"), listOf("%2", "stalled")), recorded())
            val shown = notifier.notices.toString()
            assertTrue(!shown.contains("secret") && !shown.contains("48120")) // ペイン本文・URL は通知に載らない
        }

    @Test
    fun otherStatesAreNotNotified() =
        runBlocking {
            service.observe(
                connection,
                "s0",
                reportOf(
                    pane("%1", PaneState.Working),
                    pane("%2", PaneState.Idle),
                    pane("%3", PaneState.Shell),
                    pane("%4", PaneState.Unknown),
                ),
                AT,
            )

            assertEquals(0, notifier.notices.size)
            assertEquals(0, store.notifiedRows.size)
        }

    @Test
    fun theSameStateIsNotNotifiedAgainUntilThePaneLeavesIt() =
        runBlocking {
            val asking = reportOf(pane("%1", PaneState.Asking))
            service.observe(connection, "s0", asking, AT)
            service.observe(connection, "s0", asking, AT)
            assertEquals(1, notifier.notices.size)

            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Working)), AT) // 状態を抜けた
            assertEquals(0, store.notifiedRows.size)
            service.observe(connection, "s0", asking, AT) // 再び入った = 新しい遷移
            assertEquals(2, notifier.notices.size)
        }

    @Test
    fun theRecordSurvivesARestartBecauseItLivesInTheStore() =
        runBlocking {
            val asking = reportOf(pane("%1", PaneState.Asking))
            service.observe(connection, "s0", asking, AT)

            val restarted = NotificationService(store, notifier) // プロセスの再起動を模す（記録はストアにある）
            restarted.observe(connection, "s0", asking, AT)

            assertEquals(1, notifier.notices.size)
        }

    @Test
    fun theFirstPassAfterPairingRecordsWithoutNotifying() =
        runBlocking {
            val fresh = connection.copy(baselinedAt = null)

            service.observe(fresh, "s0", reportOf(pane("%1", PaneState.Asking)), AT)

            assertEquals(0, notifier.notices.size)
            assertEquals(setOf(listOf("%1", "asking")), recorded()) // 記録だけ作る（あとで通知されない）

            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Asking)), AT) // 一巡のあとも再通知しない
            assertEquals(0, notifier.notices.size)
            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Asking), pane("%9", PaneState.Stalled)), AT)
            assertEquals(listOf("%9"), notifier.notices.map { it.paneId }) // 一巡後に新しく現れたものは通常どおり通知する
        }

    @Test
    fun whileNotificationsAreOffOnlyTheRecordIsUpdated() =
        runBlocking {
            store.settingsRow = Settings(notificationsEnabled = false)

            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Asking)), AT)

            assertEquals(0, notifier.notices.size)
            assertEquals(setOf(listOf("%1", "asking")), recorded())

            // オンに戻しても、オフの間に入った状態を一斉に通知しない
            store.settingsRow = Settings(notificationsEnabled = true)
            service.observe(connection, "s0", reportOf(pane("%1", PaneState.Asking)), AT)
            assertEquals(0, notifier.notices.size)
        }

    @Test
    fun theFirstKeyInsertionWinsWhenTwoPathsSeeTheSameTransition() =
        runBlocking {
            // 画面更新とバックグラウンドが、同じ遷移を同時に見る。後の経路は通知済みの記録を読めていない（古い読み）が、
            // notified_states の主キー（接続・セッション・ペイン・状態）の挿入が 1 回しか成功しないので、通知は 1 回になる。
            val stale =
                object : ViewerStore by store {
                    override suspend fun notifiedKeys(
                        connectionId: String,
                        session: String,
                    ): List<NotifiedKey> = emptyList()
                }
            val asking = reportOf(pane("%1", PaneState.Asking))

            NotificationService(stale, notifier).observe(connection, "s0", asking, AT)
            NotificationService(stale, notifier).observe(connection, "s0", asking, AT)

            assertEquals(1, notifier.notices.size)
            assertEquals(1, store.notifiedRows.size)
        }
}
