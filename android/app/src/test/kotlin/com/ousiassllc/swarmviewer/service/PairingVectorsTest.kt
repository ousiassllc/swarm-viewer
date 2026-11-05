package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.PairingException
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.domain.Vectors
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

private const val MIN_CASES = 55

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
private data class Want(
    val version: Int,
    val url: String,
    val token: String,
    val name: String,
    @SerialName("sessions_hint") val sessionsHint: List<String>,
)

@Serializable
private data class Case(
    val name: String,
    val input: String,
    val pad: Pad? = null,
    val want: Want? = null,
    @SerialName("want_token_length") val wantTokenLength: Int? = null,
    val error: String? = null,
)

@Serializable
private data class CaseFile(
    val description: String,
    val cases: List<Case>,
)

/**
 * 貼り付け（ディープリンク・QR も同じ経路）で取り込む接続文字列が、デスクトップ版と共有するテストベクタ（API 仕様 §2.1・§2.2）と
 * 同じ結果になることを、サービス層まで通して検査する。取り込みに成功したものは疎通確認を経て、その token・URL で保存される。
 */
class PairingVectorsTest {
    @Test
    fun everyPairingVectorBehavesAsSpecified() =
        runTest {
            val file = Vectors.load<CaseFile>("pairing.json")
            Vectors.requireMin("pairing.json", file.cases.size, MIN_CASES)
            var accepted = 0
            var rejected = 0
            for (case in file.cases) {
                val store = FakeViewerStore()
                val tokens = FakeTokenStore()
                val probed = mutableListOf<Pair<String, String>>()
                val service =
                    PairingService(
                        probe = { url, token ->
                            probed += url to token
                            listOf(Session("0", true))
                        },
                        reports = { _, _ -> Report() },
                        store = store,
                        tokens = tokens,
                        connections = ConnectionService(store, tokens),
                    )
                val raw = case.pad?.apply(case.input) ?: case.input

                if (case.error != null) {
                    rejected++
                    try {
                        service.parse(raw)
                        throw AssertionError("${case.name}: 検証に失敗するはずの入力が通った")
                    } catch (e: PairingException) {
                        assertEquals(case.name, case.error, e.error.id)
                    }
                    assertTrue(case.name, store.connectionRows.isEmpty() && tokens.tokens.isEmpty() && probed.isEmpty())
                } else {
                    accepted++
                    val draft = service.parse(raw)
                    assertFalse(case.name, draft.toString().contains(case.want?.token ?: "\u0000"))
                    val saved = service.confirm(draft.id)

                    val want = case.want
                    if (want != null) {
                        assertEquals(case.name, want.url, saved.url)
                        assertEquals(case.name, want.name, saved.name)
                        assertEquals(case.name, want.sessionsHint, draft.sessionsHint)
                        assertEquals(case.name, want.token, tokens.tokens[saved.id])
                        assertEquals(case.name, listOf(want.url to want.token), probed)
                    }
                    case.wantTokenLength?.let { assertEquals(case.name, it, tokens.tokens[saved.id]?.length) }
                }
            }
            Vectors.requireMin("pairing.json の受理ケース", accepted, 10)
            Vectors.requireMin("pairing.json の拒否ケース", rejected, 30)
        }
}
