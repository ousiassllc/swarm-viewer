package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
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
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

private const val TOKEN = "test-token-not-secret"
private const val URL_1 = "http://100.64.0.5:48120/report"
private const val URL_ENCODED = "http%3A%2F%2F100.64.0.5%3A48120%2Freport"

internal fun link(
    token: String = TOKEN,
    name: String? = null,
    sessions: String? = null,
    url: String = URL_ENCODED,
): String =
    buildString {
        append("swarm-viewer://pair?v=1&url=$url&token=$token")
        name?.let { append("&name=$it") }
        sessions?.let { append("&sessions=$it") }
    }

/** 時刻を手で進められる [Clock]。 */
internal class MutableClock(
    var now: Instant = Instant.parse("2026-11-05T10:00:00Z"),
) : Clock() {
    override fun getZone() = ZoneOffset.UTC

    override fun withZone(zone: java.time.ZoneId?) = this

    override fun instant(): Instant = now
}

class PairingServiceTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val clock = MutableClock()
    private val connections = ConnectionService(store, tokens)
    private val probed = mutableListOf<Pair<String, String>>()
    private var probeResult: () -> List<Session> = { listOf(Session("0", true)) }
    private var ids = 0
    private val service =
        PairingService(
            probe = { url, token ->
                probed += url to token
                probeResult()
            },
            store = store,
            tokens = tokens,
            connections = connections,
            clock = clock,
            newId = { "id-${++ids}" },
        )

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
            assertEquals(Connection("id-2", "devbox", URL_1, createdAt = clock.now, updatedAt = clock.now), store.connection(saved.id))
            assertNull(store.connection(saved.id)?.baselinedAt) // 最初の一巡は通知せず記録だけ（baselined_at は取得パスの完了時に記録）
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
    fun rePairingTheSameUrlReplacesTheTokenAndKeepsIdAndBaseline() =
        runTest {
            service.confirm(service.parse(link(token = "old-token", name = "old")).id)
            val first = store.connections().single()
            store.markBaselined(first.id, clock.now)
            store.setLastSession(first.id, "1")
            clock.now = clock.now.plusSeconds(3600)

            val draft = service.parse(link(token = TOKEN, name = "renamed"))
            assertTrue(draft.replacesExisting)
            val again = service.confirm(draft.id)

            assertEquals(first.id, again.id)
            assertEquals(1, store.connectionRows.size)
            assertEquals("renamed", again.name)
            assertEquals(TOKEN, tokens.tokens[first.id])
            assertEquals(clock.now.minusSeconds(3600), again.baselinedAt) // 初回の一巡は済み
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
