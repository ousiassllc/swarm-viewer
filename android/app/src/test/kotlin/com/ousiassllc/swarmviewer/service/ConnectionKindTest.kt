package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.connectionKindOf
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.Instant

private const val URL_ENCODED_OPS = "http%3A%2F%2F100.64.0.5%3A48120%2Fops"

/** 接続の種類は URL の path から導く（DB に列を持たない）。接続一覧・ペアリングの確認にも同じ値が届く。 */
class ConnectionKindTest {
    @Test
    fun theKindComesFromTheExactUrlPathAndUndeterminableUrlsAreViewOnly() {
        assertEquals(ConnectionKind.Ops, connectionKindOf("http://100.64.0.5:48120/ops"))
        assertEquals(ConnectionKind.Ops, connectionKindOf("http://[fd7a:115c:a1e0::1]:48120/ops"))
        assertEquals(ConnectionKind.View, connectionKindOf("http://100.64.0.5:48120/report"))
        for (url in listOf("", "garbage", "http://100.64.0.5:48120", "http://100.64.0.5:48120/ops/", "http://100.64.0.5:48120/OPS")) {
            assertEquals(url, ConnectionKind.View, connectionKindOf(url))
        }
        assertEquals(ConnectionKind.Ops, Connection("c", "n", "http://100.64.0.5:48120/ops").kind)
    }

    @Test
    fun theConnectionListCarriesTheKindOfEachConnection() =
        runTest {
            val store = FakeViewerStore()
            val tokens = FakeTokenStore()
            store.connectionRows["v"] = Connection("v", "view", "http://100.64.0.1:48120/report", createdAt = Instant.EPOCH)
            store.connectionRows["o"] = Connection("o", "ops", "http://100.64.0.2:48120/ops", createdAt = Instant.EPOCH.plusSeconds(1))
            tokens.put("v", "test-token-not-secret")
            tokens.put("o", "test-token-not-secret")

            val items = ConnectionService(store, tokens, io = Dispatchers.Unconfined).list()

            assertEquals(listOf(ConnectionKind.View, ConnectionKind.Ops), items.map { it.kind })
        }
}

internal class PairingServiceKindTest : PairingServiceFixture() {
    @Test
    fun theConfirmationDraftCarriesTheKindSoAnOpsConnectionCanBeWarnedAbout() =
        runTest {
            assertEquals(ConnectionKind.View, service.parse(link()).kind)
            assertEquals(ConnectionKind.Ops, service.parse(link(url = URL_ENCODED_OPS)).kind)
        }

    @Test
    fun anOpsConnectionIsSavedWithItsOpsUrlAndKeepsTheOpsKind() =
        runTest {
            val draft = service.parse(link(url = URL_ENCODED_OPS))

            val saved = service.confirm(draft.id)

            assertEquals("http://100.64.0.5:48120/ops", saved.url)
            assertEquals(ConnectionKind.Ops, saved.kind)
            assertEquals("http://100.64.0.5:48120/ops", probed.single().first) // 疎通確認も /ops に対して行う
        }
}
