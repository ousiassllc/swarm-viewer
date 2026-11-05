package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

internal class PairingServiceBaselineTest : PairingServiceFixture() {
    private fun panes(vararg states: Pair<String, PaneState>) = Report(panes = states.map { (id, state) -> Pane(id = id, state = state) })

    @Test
    fun theFirstPassRecordsCurrentAttentionWithoutNotifyingThenMarksBaselined() =
        runTest {
            probeResult = { listOf(Session("0", true), Session("2", true)) }
            reportResult = { session ->
                if (session == "0") {
                    panes("%1" to PaneState.Asking, "%2" to PaneState.Working, "%3" to PaneState.Stalled)
                } else {
                    panes("%9" to PaneState.Asking)
                }
            }

            val saved = service.confirm(service.parse(link()).id)

            assertEquals(listOf("0", "2"), fetched) // 全許可セッションを順に取得する
            assertEquals(
                setOf(NotifiedKey("%1", PaneState.Asking), NotifiedKey("%3", PaneState.Stalled)),
                store.notifiedKeys(saved.id, "0").toSet(),
            )
            assertEquals(listOf(NotifiedKey("%9", PaneState.Asking)), store.notifiedKeys(saved.id, "2"))
            assertEquals(clock.now, saved.baselinedAt)
            // 次の取得（通知側）では、すでに記録済みの状態を新たな遷移として扱わない。
            val next =
                com.ousiassllc.swarmviewer.domain.detect(
                    store.notifiedKeys(saved.id, "0"),
                    panes("%1" to PaneState.Asking).panes,
                    baselining = false,
                )
            assertTrue(next.notify.isEmpty())
        }

    @Test
    fun aSessionThatCannotBeFetchedLeavesTheBaselineOpenWithoutFailingThePairing() =
        runTest {
            probeResult = { listOf(Session("0", true), Session("2", true)) }
            reportResult = { session ->
                if (session == "0") throw FetchException(ErrKind.Unreachable)
                panes("%9" to PaneState.Asking)
            }

            val saved = service.confirm(service.parse(link()).id)

            assertEquals(listOf("0", "2"), fetched) // 1 本の失敗は飛ばして次へ進む
            assertNull(saved.baselinedAt) // 通知側の次の取得が続きを行う
            assertNull(store.connection(saved.id)?.baselinedAt)
            assertEquals(listOf(NotifiedKey("%9", PaneState.Asking)), store.notifiedKeys(saved.id, "2"))
            assertEquals(TOKEN, tokens.tokens[saved.id]) // 接続は保存済み
        }

    @Test
    fun aConnectionWithNoAllowedSessionsIsBaselinedImmediately() =
        runTest {
            probeResult = { emptyList() }

            val saved = service.confirm(service.parse(link()).id)

            assertTrue(fetched.isEmpty())
            assertEquals(clock.now, saved.baselinedAt)
        }
}
