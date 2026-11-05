package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test

private const val MIN_REPORT = 20
private const val MIN_REPORT_INVALID = 10
private const val MIN_SESSIONS = 3
private const val MIN_SESSIONS_INVALID = 8
private const val MIN_LAYOUT = 10
private const val MIN_STATE_LABELS = 9
private const val SHORT_INPUT = 8

@Serializable
private data class ReportCase(
    val name: String,
    val input: JsonElement,
    val expect: JsonElement,
)

@Serializable
private data class RawCase(
    val name: String,
    @SerialName("input_raw") val inputRaw: String,
)

@Serializable
private data class SessionsCase(
    val name: String,
    val input: JsonElement,
    val want: List<String>,
)

@Serializable
private data class LayoutCase(
    val name: String,
    val rows: Int,
    val cols: List<Int>,
    @SerialName("pane_count") val paneCount: Int,
    val want: Boolean,
)

@Serializable
private data class StateLabelCase(
    val state: String,
    val label: String,
)

@Serializable
private data class ReportFile(
    val description: String,
    @SerialName("parse_report") val parseReport: List<ReportCase>,
    @SerialName("parse_report_invalid") val parseReportInvalid: List<RawCase>,
    @SerialName("parse_sessions") val parseSessions: List<SessionsCase>,
    @SerialName("parse_sessions_invalid") val parseSessionsInvalid: List<RawCase>,
    @SerialName("layout_valid") val layoutValid: List<LayoutCase>,
    @SerialName("state_labels") val stateLabels: List<StateLabelCase>,
)

class VectorsReportTest {
    private val file = Vectors.load<ReportFile>("report.json")

    @Test
    fun parseReportMatchesVectors() {
        Vectors.requireMin("report.json の parse_report", file.parseReport.size, MIN_REPORT)
        for (case in file.parseReport) {
            val report = parseReport(case.input.toString())
            val mismatch = subsetMismatch("", case.expect, normalizedReport(report))
            assertEquals("${case.name}: $mismatch", "", mismatch)
        }
    }

    @Test
    fun invalidReportsAreRejectedWithoutLeakingTheBody() {
        Vectors.requireMin("report.json の parse_report_invalid", file.parseReportInvalid.size, MIN_REPORT_INVALID)
        for (case in file.parseReportInvalid) {
            val e = assertThrows(case.name, InvalidResponseException::class.java) { parseReport(case.inputRaw) }
            assertFalse(case.name, case.inputRaw.length > SHORT_INPUT && e.message.orEmpty().contains(case.inputRaw))
        }
    }

    @Test
    fun parseSessionsMatchesVectors() {
        Vectors.requireMin("report.json の parse_sessions", file.parseSessions.size, MIN_SESSIONS)
        for (case in file.parseSessions) {
            val sessions = parseSessions(case.input.toString())
            assertEquals(case.name, case.want, sessions.map { it.id })
            assertFalse(case.name, sessions.any { !it.allowed })
        }
        Vectors.requireMin("report.json の parse_sessions_invalid", file.parseSessionsInvalid.size, MIN_SESSIONS_INVALID)
        for (case in file.parseSessionsInvalid) {
            assertThrows(case.name, InvalidResponseException::class.java) { parseSessions(case.inputRaw) }
        }
    }

    @Test
    fun layoutValidMatchesVectors() {
        Vectors.requireMin("report.json の layout_valid", file.layoutValid.size, MIN_LAYOUT)
        for (case in file.layoutValid) {
            assertEquals(case.name, case.want, Layout(case.rows, case.cols).valid(case.paneCount))
        }
    }

    @Test
    fun stateLabelsMatchVectors() {
        Vectors.requireMin("report.json の state_labels", file.stateLabels.size, MIN_STATE_LABELS)
        for (case in file.stateLabels) {
            assertEquals(case.state, case.label, PaneState.parse(case.state).label)
        }
    }
}

/** expect が actual の部分木か（testdata/vectors/README.md）。一致なら空文字、違えば場所と理由。 */
private fun subsetMismatch(
    path: String,
    expect: JsonElement,
    actual: JsonElement?,
): String =
    when {
        expect is JsonObject -> {
            if (actual !is JsonObject) {
                "$path: オブジェクトでない"
            } else {
                expect.entries
                    .firstNotNullOfOrNull { (key, value) ->
                        actual[key]?.let { subsetMismatch("$path/$key", value, it).ifEmpty { null } }
                            ?: "$path/$key: キーが無い".takeIf { key !in actual }
                    }.orEmpty()
            }
        }
        expect is JsonArray -> {
            if (actual !is JsonArray || actual.size != expect.size) {
                "$path: 配列の長さが違う"
            } else {
                expect.indices.firstNotNullOfOrNull { subsetMismatch("$path/$it", expect[it], actual[it]).ifEmpty { null } }.orEmpty()
            }
        }
        scalarEquals(expect, actual) -> ""
        else -> "$path: $actual != $expect"
    }

private fun scalarEquals(
    expect: JsonElement,
    actual: JsonElement?,
): Boolean {
    if (expect !is JsonPrimitive || actual !is JsonPrimitive) return false
    if (expect is JsonNull || actual is JsonNull) return expect is JsonNull && actual is JsonNull
    if (expect.isString || actual.isString) return expect.isString && actual.isString && expect.content == actual.content
    val expectNumber = expect.content.toDoubleOrNull()
    return if (expectNumber != null) expectNumber == actual.content.toDoubleOrNull() else expect.content == actual.content
}
