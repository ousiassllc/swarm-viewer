package com.ousiassllc.swarmviewer.domain

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

private const val TOKEN = "test-token-not-secret"
private const val DOMAIN_SOURCES = "src/main/kotlin/com/ousiassllc/swarmviewer/domain"
private val FORBIDDEN_IMPORTS = listOf("android.", "androidx.", "okhttp3.", "okio.")

/** token が toString・ログ・例外メッセージに出ないこと、domain が Android API 等に依存しないこと。 */
class DomainSafetyTest {
    private val payload =
        parsePairing("swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=$TOKEN&name=devbox&sessions=0%2C1")

    @Test
    fun payloadKeepsTheTokenButHidesItInText() {
        assertEquals(TOKEN, payload.token)
        val outputs =
            listOf(
                payload.toString(),
                "$payload",
                String.format("%s", payload),
                listOf(payload).toString(),
                mapOf("p" to payload).toString(),
                Result.success(payload).toString(),
            )
        for (out in outputs) {
            assertFalse("token が含まれる: $out", out.contains(TOKEN))
            assertFalse("完全な URL が含まれる: $out", out.contains("/report"))
            assertTrue("host が無い: $out", out.contains("100.64.0.5"))
        }
        assertEquals(TOKEN, payload.token)
    }

    @Test
    fun payloadCopyAndEqualityStillWork() {
        assertEquals(payload, payload.copy())
        assertNotEquals(payload, payload.copy(token = "other"))
    }

    @Test
    fun pairingFailuresNeverCarryTheInput() {
        val inputs =
            listOf(
                "swarm-viewer://pair?v=2&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=$TOKEN",
                "swarm-viewer://pair?v=1&url=http%3A%2F%2Fexample.com%2Freport&token=$TOKEN",
                "swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=$TOKEN&token=again",
                "https://pair?token=$TOKEN",
            )
        for (input in inputs) {
            val e = assertThrows(PairingException::class.java) { parsePairing(input) }
            assertFalse(e.message.orEmpty().contains(TOKEN))
            assertFalse(e.toString().contains(TOKEN))
            assertEquals(null, e.cause)
        }
    }

    @Test
    fun invalidUrlExceptionDoesNotCarryTheUrl() {
        val e = assertThrows(InvalidUrlException::class.java) { validateUrl("http://secret-host.example:1/report") }
        assertFalse(e.message.orEmpty().contains("secret-host"))
    }

    @Test
    fun fetchAndResponseErrorsHaveFixedMessages() {
        val fetch = FetchException(ErrKind.HostError, "tmux_error")
        assertEquals("swarm-console fetch error: host_error (tmux_error)", fetch.message)
        assertEquals("swarm-console fetch error: unreachable", FetchException(ErrKind.Unreachable).message)
        assertEquals("swarm-console: invalid response", InvalidResponseException().message)
        val e = assertThrows(InvalidResponseException::class.java) { parseReport("{\"panes\":\"$TOKEN\"}") }
        assertFalse(e.message.orEmpty().contains(TOKEN))
    }

    @Test
    fun connectionHasNoTokenProperty() {
        val names = Connection::class.java.declaredFields.map { it.name.lowercase() }
        assertTrue("Connection に token を持たせない: $names", names.none { it.contains("token") })
        val connection = Connection(id = "c1", name = "devbox", url = "http://[fd7a:115c:a1e0::1]:48120/report")
        assertEquals("fd7a:115c:a1e0::1", connection.host)
        assertEquals("100.64.0.5", Connection("c", "n", "http://100.64.0.5:48120/report").host)
        assertEquals("", Connection("c", "n", "://").host)
        assertFalse(connection.baselined)
    }

    @Test
    fun domainDoesNotImportAndroidOkHttpOrRoom() {
        val files = File(DOMAIN_SOURCES).listFiles { f -> f.extension == "kt" }.orEmpty()
        assertTrue("domain のソースが見つからない: ${File(DOMAIN_SOURCES).absolutePath}", files.size >= 10)
        for (file in files) {
            val imports = file.readLines().filter { it.startsWith("import ") }.map { it.removePrefix("import ") }
            for (import in imports) {
                assertTrue("${file.name}: 許可しない import $import", FORBIDDEN_IMPORTS.none { import.startsWith(it) })
            }
        }
    }
}
