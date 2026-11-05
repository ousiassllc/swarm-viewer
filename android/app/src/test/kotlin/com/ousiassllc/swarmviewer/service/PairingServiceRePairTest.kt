package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Report
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

internal class PairingServiceRePairTest : PairingServiceFixture() {
    @Test
    fun rePairingTheSameUrlReplacesTheTokenAndKeepsIdAndBaseline() =
        runTest {
            service.confirm(service.parse(link(token = "old-token", name = "old")).id)
            val first = store.connections().single()
            val baselinedAt = first.baselinedAt
            assertNotNull(baselinedAt) // 最初のペアリングで最初の一巡が済んでいる
            fetched.clear()
            store.setLastSession(first.id, "1")
            clock.now = clock.now.plusSeconds(3600)

            val draft = service.parse(link(token = TOKEN, name = "renamed"))
            assertTrue(draft.replacesExisting)
            val again = service.confirm(draft.id)

            assertEquals(first.id, again.id)
            assertEquals(1, store.connectionRows.size)
            assertEquals("renamed", again.name)
            assertEquals(TOKEN, tokens.tokens[first.id])
            assertEquals(baselinedAt, again.baselinedAt) // 初回の一巡は済んでいるので、やり直さない
            assertTrue(fetched.isEmpty())
            assertEquals("1", again.lastSession)
            assertEquals(clock.now, again.updatedAt)
        }

    @Test
    fun rePairingClearsTheNeedsRepairMark() =
        runTest {
            service.confirm(service.parse(link(token = "old-token")).id)
            val id = store.connections().single().id
            connections.markNeedsRepair(id)

            service.confirm(service.parse(link()).id)

            assertFalse(id in connections.needsRepair.value)
            assertFalse(connections.list().single().needsRepair)
        }

    @Test
    fun aTokenSaveFailureRollsBackANewConnection() =
        runTest {
            val draft = service.parse(link())
            tokens.failPut = true

            try {
                service.confirm(draft.id)
                throw AssertionError("保存の失敗が伝わらなかった")
            } catch (e: PairingSaveException) {
                assertFalse(e.message.orEmpty().contains(TOKEN))
                assertNull(e.cause) // 原因（パス・鍵の情報）を持ち込まない
            }
            assertTrue(store.connectionRows.isEmpty()) // 行だけ残る不整合を作らない
            assertTrue(tokens.tokens.isEmpty())

            tokens.failPut = false // 下書きは残っており、再試行できる
            assertEquals(URL_1, service.confirm(draft.id).url)
        }

    @Test
    fun aTokenSaveFailureKeepsAnExistingConnection() =
        runTest {
            service.confirm(service.parse(link(token = "old-token")).id)
            val id = store.connections().single().id
            tokens.failPut = true

            try {
                service.confirm(service.parse(link(token = TOKEN)).id)
                throw AssertionError("保存の失敗が伝わらなかった")
            } catch (e: PairingSaveException) {
                // 既存の接続は残り、token は古いまま
            }
            assertEquals(id, store.connections().single().id)
            assertEquals("old-token", tokens.tokens[id])
        }

    @Test
    fun anUnfinishedBaselineIsRetriedWhenRePairing() =
        runTest {
            reportResult = { throw FetchException(ErrKind.Unreachable) }
            val first = service.confirm(service.parse(link(token = "old-token")).id)
            assertNull(first.baselinedAt)
            fetched.clear()
            reportResult = { Report() }

            val again = service.confirm(service.parse(link()).id)

            assertEquals(first.id, again.id)
            assertEquals(listOf("0"), fetched)
            assertEquals(clock.now, again.baselinedAt)
        }
}
