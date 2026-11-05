package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

internal const val TOKEN = "test-token-not-secret"
internal const val URL_1 = "http://100.64.0.5:48120/report"
internal const val URL_ENCODED = "http%3A%2F%2F100.64.0.5%3A48120%2Freport"

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

/** [PairingService] のテストの共通の組み立て（フェイクの store・token・時計、疎通確認と取得の差し替え）。 */
internal abstract class PairingServiceFixture {
    protected val store = FakeViewerStore()
    protected val tokens = FakeTokenStore()
    protected val clock = MutableClock()
    protected val connections = ConnectionService(store, tokens)
    protected val probed = mutableListOf<Pair<String, String>>()
    protected var probeResult: () -> List<Session> = { listOf(Session("0", true)) }
    protected val fetched = mutableListOf<String>()
    protected var reportResult: (String) -> Report = { Report() }
    protected var ids = 0
    protected val service =
        PairingService(
            probe = { url, token ->
                probed += url to token
                probeResult()
            },
            reports = { _, session ->
                fetched += session
                reportResult(session)
            },
            store = store,
            tokens = tokens,
            connections = connections,
            clock = clock,
            newId = { "id-${++ids}" },
        )
}
