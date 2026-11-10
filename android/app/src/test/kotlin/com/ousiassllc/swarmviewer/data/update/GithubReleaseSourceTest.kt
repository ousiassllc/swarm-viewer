package com.ousiassllc.swarmviewer.data.update

import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import kotlinx.coroutines.runBlocking
import okhttp3.Request
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.net.UnknownHostException
import java.util.concurrent.TimeUnit

/** GitHub への通信（本物の HTTP を、手元の MockWebServer で受ける。実際の GitHub・tailnet には届かない）。 */
class GithubReleaseSourceTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val server = MockWebServer()
    private lateinit var endpoints: UpdateEndpoints
    private lateinit var source: GithubReleaseSource
    private val dest by lazy { File(tmp.root, "updates/app.apk") }
    private val payload = ByteArray(200_000) { (it % 251).toByte() }

    @Before
    fun setUp() {
        server.start()
        endpoints = UpdateEndpoints.forTesting(server.url("/repos/ousiassllc/swarm-viewer/releases/latest").toString())
        source = GithubReleaseSource(endpoints)
    }

    @After
    fun tearDown() {
        runCatching { server.shutdown() }
    }

    private fun failure(block: suspend () -> Unit): UpdateFailure =
        assertThrows(UpdateException::class.java) {
            runBlocking { block() }
        }.failure

    private val releaseJson =
        """{"tag_name":"v0.2.0","assets":[{"name":"SHA256SUMS","browser_download_url":"https://github.com/x/sums"}]}"""

    @Test
    fun theLatestReleaseIsReadAndNoCredentialsAreSent() =
        runBlocking {
            server.enqueue(MockResponse().setBody(releaseJson))

            val release = source.latest()

            assertEquals("v0.2.0", release?.tag)
            val request = server.takeRequest(1, TimeUnit.SECONDS)!!
            assertEquals("GET", request.method)
            assertEquals("/repos/ousiassllc/swarm-viewer/releases/latest", request.path)
            // (h) token・Authorization・cookie は GitHub へ送らない
            assertNull(request.getHeader("Authorization"))
            assertNull(request.getHeader("Cookie"))
            assertNull(request.getHeader("Proxy-Authorization"))
            assertNotNull(request.getHeader("User-Agent"))
        }

    // (h)
    @Test
    fun theGuardStripsAuthorizationAndCookieEvenIfACallerSetsThem() {
        server.enqueue(MockResponse().setBody("ok"))
        val request =
            Request
                .Builder()
                .url(endpoints.latestUrl)
                .header("Authorization", "Bearer secret-console-token")
                .header("Proxy-Authorization", "Basic secret")
                .header("Cookie", "session=secret")
                .build()

        newUpdateHttpClient(endpoints).newCall(request).execute().close()

        val seen = server.takeRequest(1, TimeUnit.SECONDS)!!
        assertNull(seen.getHeader("Authorization"))
        assertNull(seen.getHeader("Proxy-Authorization"))
        assertNull(seen.getHeader("Cookie"))
        assertFalse(seen.headers.toString().contains("secret"))
    }

    @Test
    fun theGuardRefusesNonGetAndForeignHostsBeforeConnecting() {
        val client = newUpdateHttpClient(UpdateEndpoints.PRODUCTION)
        for (url in listOf("http://api.github.com/", "https://100.64.0.1/", "https://example.com/", "https://127.0.0.1:443/")) {
            assertThrows(url, UpdateRejectedException::class.java) { client.newCall(Request.Builder().url(url).build()).execute() }
        }
        val post =
            Request
                .Builder()
                .url(UpdateEndpoints.PRODUCTION.latestUrl)
                .post(okhttp3.RequestBody.create(null, "x"))
                .build()
        assertThrows(UpdateRejectedException::class.java) { client.newCall(post).execute() }
    }

    @Test
    fun productionSourceRejectsForeignUrlsWithoutAnyNetworkAccess() {
        val production = GithubReleaseSource(UpdateEndpoints.PRODUCTION)
        for (url in listOf(
            "http://api.github.com/x",
            "https://127.0.0.1/x",
            "https://100.64.0.1/x",
            "https://evil.example/x",
            "not a url",
        )) {
            assertEquals(url, UpdateFailure.Rejected, failure { production.text(url) })
            assertEquals(url, UpdateFailure.Rejected, failure { production.download(url, dest) })
        }
        assertFalse(dest.exists())
    }

    @Test
    fun theProductionNameResolutionNeverYieldsLoopbackOrPrivateAddresses() {
        assertThrows(UnknownHostException::class.java) { PublicOnlyDns.lookup("localhost") }
    }

    @Test
    fun anHttpErrorIsANetworkFailure() {
        server.enqueue(MockResponse().setResponseCode(500).setBody("secret internal detail"))
        assertEquals(UpdateFailure.Network, failure { source.latest() })
        server.enqueue(MockResponse().setResponseCode(403))
        assertEquals(UpdateFailure.Network, failure { source.text(server.url("/sums").toString()) })
    }

    // (d)
    @Test
    fun offlineIsANetworkFailure() {
        server.shutdown()
        assertEquals(UpdateFailure.Network, failure { source.latest() })
        assertEquals(UpdateFailure.Network, failure { source.download(endpoints.latestUrl.toString(), dest) })
        assertFalse(dest.exists())
    }

    @Test
    fun aDownloadIsWrittenCompletely() =
        runBlocking {
            server.enqueue(MockResponse().setBody(okio.Buffer().write(payload)))
            source.download(server.url("/apk").toString(), dest)
            assertArrayEquals(payload, dest.readBytes())
        }

    // (d)
    @Test
    fun aDownloadCutOffMidwayIsTruncatedAndTheFileIsDeleted() {
        server.enqueue(MockResponse().setBody(okio.Buffer().write(payload)).setSocketPolicy(SocketPolicy.DISCONNECT_DURING_RESPONSE_BODY))
        assertEquals(UpdateFailure.Truncated, failure { source.download(server.url("/apk").toString(), dest) })
        assertFalse(dest.exists())
    }

    @Test
    fun aDownloadShorterThanItsContentLengthIsTruncated() {
        // 宣言より短く閉じる。OkHttp が「想定外の終端」として読み取りエラーにする。
        server.enqueue(
            MockResponse()
                .setBody(okio.Buffer().write(payload.copyOf(1000)))
                .setHeader("Content-Length", payload.size.toString()) // setBody は長さを上書きするので、あとで宣言する
                .setSocketPolicy(SocketPolicy.DISCONNECT_AT_END),
        )
        assertEquals(UpdateFailure.Truncated, failure { source.download(server.url("/apk").toString(), dest) })
        assertFalse(dest.exists())
    }

    @Test
    fun aRedirectToAnAllowedHostIsFollowedWithoutCredentials() =
        runBlocking {
            server.enqueue(MockResponse().setResponseCode(302).setHeader("Location", server.url("/real").toString()))
            server.enqueue(MockResponse().setBody("sums text"))

            assertEquals("sums text", source.text(server.url("/start").toString()))

            assertEquals("/start", server.takeRequest(1, TimeUnit.SECONDS)!!.path)
            val second = server.takeRequest(1, TimeUnit.SECONDS)!!
            assertEquals("/real", second.path)
            assertNull(second.getHeader("Authorization"))
        }

    @Test
    fun aRedirectToAForeignHostIsRejectedAndNeverContacted() {
        for (target in listOf("https://evil.example/steal", "http://100.64.0.1:48120/report", "ftp://example.com/x")) {
            server.enqueue(MockResponse().setResponseCode(302).setHeader("Location", target))
            assertEquals(target, UpdateFailure.Rejected, failure { source.text(server.url("/start").toString()) })
        }
        assertEquals(3, server.requestCount) // 最初の宛先だけに届き、リダイレクト先には届いていない
    }

    @Test
    fun anEndlessRedirectLoopGivesUp() {
        repeat(10) { server.enqueue(MockResponse().setResponseCode(302).setHeader("Location", server.url("/loop").toString())) }
        assertEquals(UpdateFailure.Network, failure { source.text(server.url("/loop").toString()) })
        assertTrue(server.requestCount <= 7)
    }
}
