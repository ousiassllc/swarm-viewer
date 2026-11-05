package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PairingError
import com.ousiassllc.swarmviewer.domain.PairingException
import com.ousiassllc.swarmviewer.domain.Session
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

internal class PairingServiceTest : PairingServiceFixture() {
    @Test
    fun parseGivesAConfirmationWithoutTheTokenAndSavesNothing() =
        runTest {
            val draft = service.parse(link(name = "devbox", sessions = "0%2C1"))

            assertEquals("devbox", draft.name)
            assertEquals("100.64.0.5", draft.host) // URL 全体ではなく host のみ
            assertEquals(listOf("0", "1"), draft.sessionsHint)
            assertFalse(draft.replacesExisting)
            assertFalse(draft.toString().contains(TOKEN))
            assertFalse(draft.toString().contains("48120"))
            assertTrue(store.connectionRows.isEmpty())
            assertTrue(tokens.tokens.isEmpty())
            assertTrue(probed.isEmpty()) // 確定するまで通信しない
        }

    @Test
    fun parseRejectsInvalidInputWithTheSpecifiedKinds() =
        runTest {
            val cases =
                mapOf(
                    "http://example.com" to PairingError.Format,
                    "swarm-viewer://pair?v=2&url=$URL_ENCODED&token=$TOKEN" to PairingError.Version,
                    link(url = "http%3A%2F%2F192.168.0.1%3A48120%2Freport") to PairingError.InvalidUrl,
                    link(token = "") to PairingError.Token,
                    "swarm-viewer://pair?v=1&v=1&url=$URL_ENCODED&token=$TOKEN" to PairingError.Format,
                )
            for ((raw, error) in cases) {
                try {
                    service.parse(raw)
                    throw AssertionError("検証に失敗するはずの入力が通った: $error")
                } catch (e: PairingException) {
                    assertEquals(error, e.error)
                    assertFalse(e.message.orEmpty().contains(TOKEN))
                }
            }
            assertTrue(store.connectionRows.isEmpty())
        }

    @Test
    fun confirmSavesTheConnectionAndTokenOnlyAfterTheProbeSucceeds() =
        runTest {
            val draft = service.parse(link(name = "devbox"))

            val saved = service.confirm(draft.id)

            assertEquals(listOf(URL_1 to TOKEN), probed)
            assertEquals("id-2", saved.id) // 下書き ID（id-1）とは別の接続 ID
            assertEquals(
                Connection("id-2", "devbox", URL_1, createdAt = clock.now, updatedAt = clock.now, baselinedAt = clock.now),
                store.connection(saved.id),
            )
            assertEquals(saved, store.connection(saved.id))
            assertEquals(TOKEN, tokens.tokens[saved.id])
            assertEquals(saved.id, store.settings().lastConnectionId) // 最初の接続は選択される
        }

    @Test
    fun theDraftIsGoneAfterConfirm() =
        runTest {
            val draft = service.parse(link())
            service.confirm(draft.id)

            try {
                service.confirm(draft.id)
                throw AssertionError("下書きが残っている")
            } catch (e: DraftExpiredException) {
                assertEquals(1, store.connectionRows.size)
            }
        }

    @Test
    fun aFailedProbeSavesNothingAndKeepsTheDraftForARetry() =
        runTest {
            val draft = service.parse(link())
            probeResult = { throw FetchException(ErrKind.Unreachable) }

            try {
                service.confirm(draft.id)
                throw AssertionError("疎通確認の失敗が伝わらなかった")
            } catch (e: FetchException) {
                assertEquals(ErrKind.Unreachable, e.kind)
            }
            assertTrue(store.connectionRows.isEmpty())
            assertTrue(tokens.tokens.isEmpty())

            probeResult = { listOf(Session("0", true)) } // 回復した
            assertEquals(URL_1, service.confirm(draft.id).url)
            assertEquals(2, probed.size)
        }

    @Test
    fun everyProbeFailureKindIsPassedThroughAndNeverSaved() =
        runTest {
            for (kind in ErrKind.entries) {
                val draft = service.parse(link())
                probeResult = { throw FetchException(kind) }
                try {
                    service.confirm(draft.id)
                    throw AssertionError("$kind")
                } catch (e: FetchException) {
                    assertEquals(kind, e.kind)
                }
            }
            assertTrue(store.connectionRows.isEmpty())
            assertTrue(tokens.tokens.isEmpty())
        }

    @Test
    fun theLastConnectionIsNotChangedWhenOneIsAlreadySelected() =
        runTest {
            val first = service.confirm(service.parse(link()).id)
            val secondLink = link(url = "http%3A%2F%2F100.64.0.6%3A48120%2Freport")

            val second = service.confirm(service.parse(secondLink).id)

            assertNotEquals(first.id, second.id)
            assertEquals(first.id, store.settings().lastConnectionId)
        }
}
