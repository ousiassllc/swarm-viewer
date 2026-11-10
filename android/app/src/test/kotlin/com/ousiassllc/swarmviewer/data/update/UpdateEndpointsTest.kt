package com.ousiassllc.swarmviewer.data.update

import com.ousiassllc.swarmviewer.domain.InvalidUrlException
import com.ousiassllc.swarmviewer.domain.validateUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress

/** 本番（リリース）の設定は、GitHub のホストへの https だけを許す（#109 §5・§7）。 */
class UpdateEndpointsTest {
    private val production = UpdateEndpoints.PRODUCTION

    @Test
    fun productionUsesTheGithubLatestReleaseApi() {
        assertEquals("https://api.github.com/repos/ousiassllc/swarm-viewer/releases/latest", production.latestUrl.toString())
        assertFalse(production.relaxed)
        assertEquals(
            setOf("api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"),
            UpdateEndpoints.GITHUB_HOSTS,
        )
    }

    @Test
    fun productionPermitsOnlyHttpsToTheGithubHosts() {
        for (host in UpdateEndpoints.GITHUB_HOSTS) {
            assertTrue(host, production.permits("https://$host/some/path?x=1".toHttpUrl()))
            assertFalse(host, production.permits("http://$host/some/path".toHttpUrl()))
            assertFalse(host, production.permits("https://$host:8443/".toHttpUrl()))
        }
        assertTrue(production.permits(production.latestUrl))
    }

    @Test
    fun productionRejectsEveryOtherHost() {
        val rejected =
            listOf(
                "https://example.com/",
                "https://evil.github.com/", // サブドメインは許可しない
                "https://github.com.evil.example/",
                "https://notgithub.com/",
                "https://githubusercontent.com/",
                "https://raw.githubusercontent.com/",
                "https://127.0.0.1/",
                "https://10.0.2.2/",
                "https://100.64.0.1/", // tailnet
                "https://192.168.0.1/",
                "https://[::1]/",
                "https://localhost/",
                "https://devbox.ts.net/",
            )
        for (url in rejected) assertFalse(url, production.permits(url.toHttpUrl()))
        assertFalse(production.permits("http://100.64.0.1:48120/report".toHttpUrl()))
    }

    @Test
    fun theTailnetGuardDoesNotAllowGithubHosts() {
        // GitHub のホストは tailnet 用（report/ops）の許可リストには入れていない（別のクライアント・別の許可リスト）
        for (host in UpdateEndpoints.GITHUB_HOSTS) {
            val url = "https://$host/report"
            assertThrows(url, InvalidUrlException::class.java) { validateUrl(url) }
        }
    }

    @Test
    fun theRelaxedTestSettingStillPermitsOnlyItsOwnHost() {
        val testing = UpdateEndpoints.forTesting("http://10.0.2.2:8765/repos/x/y/releases/latest")
        assertTrue(testing.permits("http://10.0.2.2:8765/a".toHttpUrl()))
        assertFalse(testing.permits("https://api.github.com/".toHttpUrl()))
        assertFalse(testing.permits("http://100.64.0.1:8765/".toHttpUrl()))
    }

    @Test
    fun privateLoopbackAndTailnetAddressesAreNotPublic() {
        for (ip in listOf(
            "127.0.0.1",
            "0.0.0.0",
            "10.1.2.3",
            "172.16.0.1",
            "192.168.1.1",
            "169.254.1.1",
            "100.64.0.1",
            "100.127.255.254",
            "::1",
            "fe80::1",
            "fc00::1",
            "fd12::1",
            "224.0.0.1",
        )) {
            assertFalse(ip, isPublicAddress(InetAddress.getByName(ip)))
        }
        for (ip in listOf("140.82.112.5", "185.199.108.133", "100.63.255.255", "100.128.0.1", "2606:50c0:8000::154")) {
            assertTrue(ip, isPublicAddress(InetAddress.getByName(ip)))
        }
    }
}
