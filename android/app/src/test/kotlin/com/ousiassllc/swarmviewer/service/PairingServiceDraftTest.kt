package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.domain.Session
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

internal class PairingServiceDraftTest : PairingServiceFixture() {
    @Test
    fun aDraftExpiresAfterTheTtlAndIsNotProbed() =
        runTest {
            val draft = service.parse(link())
            clock.now = clock.now.plus(DRAFT_TTL).minusSeconds(1)
            assertEquals(URL_1, service.confirm(draft.id).url) // 期限の直前はまだ有効

            val second = service.parse(link(token = "another"))
            probed.clear()
            clock.now = clock.now.plus(DRAFT_TTL)
            try {
                service.confirm(second.id)
                throw AssertionError("期限切れの下書きが使えた")
            } catch (e: DraftExpiredException) {
                assertTrue(probed.isEmpty())
            }
        }

    @Test
    fun aNewParseReplacesTheOldDraft() =
        runTest {
            val old = service.parse(link(token = "first-token"))
            val current = service.parse(link(token = "second-token"))
            assertNotEquals(old.id, current.id)

            try {
                service.confirm(old.id)
                throw AssertionError("古い下書きが使えた")
            } catch (e: DraftExpiredException) {
                assertTrue(probed.isEmpty())
            }
            service.confirm(current.id)
            assertEquals("second-token", probed.single().second)
        }

    @Test
    fun aDraftReplacedDuringTheProbeIsNotSaved() =
        runTest {
            val draft = service.parse(link(token = "first-token"))
            probeResult = {
                kotlinx.coroutines.runBlocking { service.parse(link(token = "second-token")) } // 疎通確認の間に新しい入力が来た
                listOf(Session("0", true))
            }

            try {
                service.confirm(draft.id)
                throw AssertionError("置き換えられた下書きが保存された")
            } catch (e: DraftExpiredException) {
                assertTrue(store.connectionRows.isEmpty())
                assertTrue(tokens.tokens.isEmpty())
            }
        }

    @Test
    fun discardDropsTheDraftAndItsToken() =
        runTest {
            val draft = service.parse(link())
            service.discard(draft.id)

            try {
                service.confirm(draft.id)
                throw AssertionError("破棄した下書きが使えた")
            } catch (e: DraftExpiredException) {
                assertTrue(probed.isEmpty())
            }
        }

    @Test
    fun discardingAnotherDraftsIdKeepsTheCurrentOne() =
        runTest {
            val old = service.parse(link(token = "first-token"))
            val current = service.parse(link(token = "second-token"))

            service.discard(old.id)

            assertEquals(URL_1, service.confirm(current.id).url)
        }
}
