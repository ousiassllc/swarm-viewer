package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test

private const val MIN_URL_CASES = 75
private const val MIN_PAIRING_CASES = 55
private const val FAKE_TOKEN = "test-token-not-secret"
private const val SHORT_INPUT = 8

@Serializable
private data class UrlCase(
    val name: String,
    val input: String,
    val want: String?,
)

@Serializable
private data class UrlFile(
    val description: String,
    val cases: List<UrlCase>,
)

@Serializable
private data class Pad(
    val fill: String,
    @SerialName("to_bytes") val toBytes: Int,
    val suffix: String = "",
) {
    fun apply(prefix: String): String {
        val out = StringBuilder(prefix)
        var bytes = prefix.toByteArray(Charsets.UTF_8).size
        while (bytes < toBytes) {
            out.append(fill)
            bytes += fill.toByteArray(Charsets.UTF_8).size
        }
        return out.append(suffix).toString()
    }
}

@Serializable
private data class PairingWant(
    val version: Int,
    val url: String,
    val token: String,
    val name: String,
    @SerialName("sessions_hint") val sessionsHint: List<String>,
)

@Serializable
private data class PairingCase(
    val name: String,
    val input: String,
    val pad: Pad? = null,
    val want: PairingWant? = null,
    @SerialName("want_token_length") val wantTokenLength: Int? = null,
    val error: String? = null,
)

@Serializable
private data class PairingFile(
    val description: String,
    val cases: List<PairingCase>,
)

class VectorsUrlPairingTest {
    @Test
    fun validateUrlMatchesVectors() {
        val file = Vectors.load<UrlFile>("url.json")
        Vectors.requireMin("url.json", file.cases.size, MIN_URL_CASES)
        for (case in file.cases) {
            if (case.want == null) {
                val e = assertThrows(case.name, InvalidUrlException::class.java) { validateUrl(case.input) }
                assertFalse(case.name, case.input.length > SHORT_INPUT && e.message.orEmpty().contains(case.input))
            } else {
                assertEquals(case.name, case.want, validateUrl(case.input))
                assertEquals("${case.name}: 正規化結果が冪等でない", case.want, validateUrl(case.want))
            }
        }
    }

    @Test
    fun parsePairingMatchesVectors() {
        val file = Vectors.load<PairingFile>("pairing.json")
        Vectors.requireMin("pairing.json", file.cases.size, MIN_PAIRING_CASES)
        val errors =
            mapOf(
                "format" to PairingError.Format,
                "version" to PairingError.Version,
                "token" to PairingError.Token,
                "invalid_url" to PairingError.InvalidUrl,
            )
        for (case in file.cases) {
            val input = case.pad?.apply(case.input) ?: case.input
            if (case.error != null) {
                val e = assertThrows(case.name, PairingException::class.java) { parsePairing(input) }
                assertEquals(case.name, errors.getValue(case.error), e.error)
                val message = e.message.orEmpty()
                assertFalse("${case.name}: 例外に token が含まれる", message.contains(FAKE_TOKEN))
                assertFalse("${case.name}: 例外に入力が含まれる", input.length > SHORT_INPUT && message.contains(input))
            } else {
                assertEquals(case.name, expected(case), parsePairing(input).toExpected())
            }
        }
    }

    private fun expected(case: PairingCase): PairingWant {
        case.want?.let { return it }
        val length = checkNotNull(case.wantTokenLength) { "${case.name}: want・want_token_length・error のどれも無い" }
        return PairingWant(1, "http://100.64.0.5:48120/report", "a".repeat(length), "100.64.0.5", emptyList())
    }

    private fun PairingPayload.toExpected() = PairingWant(version, url, token, name, sessionsHint)
}
