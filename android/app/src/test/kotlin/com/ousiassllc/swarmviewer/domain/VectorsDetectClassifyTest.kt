package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

private const val MIN_DETECT = 20
private const val MIN_CYCLES = 2
private const val MIN_CLASSIFY = 30
private const val MIN_TONE = 15
private const val MIN_PERCENT = 7
private const val SHORT_INPUT = 8

@Serializable
private data class VKey(
    @SerialName("pane_id") val paneId: String,
    val state: String,
) {
    fun toKey() = NotifiedKey(paneId, PaneState.parse(state))
}

@Serializable
private data class VPane(
    val id: String,
    val pos: String,
    val state: String,
)

@Serializable
private data class DetectCase(
    val name: String,
    val known: List<VKey>,
    val panes: List<VPane>,
    val baselining: Boolean,
    val notify: List<VKey>,
    val insert: List<VKey>,
    val delete: List<VKey>,
)

@Serializable
private data class CycleStep(
    val state: String,
    val notify: Int,
)

@Serializable
private data class CycleCase(
    val name: String,
    val steps: List<CycleStep>,
)

@Serializable
private data class DetectFile(
    val description: String,
    val cases: List<DetectCase>,
    @SerialName("detect_cycles") val cycles: List<CycleCase>,
)

@Serializable
private data class VWant(
    val kind: String,
    val code: String,
)

@Serializable
private data class ClassifyCase(
    val name: String,
    val error: String,
    @SerialName("fetch_kind") val fetchKind: String? = null,
    @SerialName("fetch_code") val fetchCode: String? = null,
    @SerialName("http_status") val httpStatus: Int,
    @SerialName("tool_code") val toolCode: String,
    val want: VWant?,
)

@Serializable
private data class ClassifyFile(
    val description: String,
    val cases: List<ClassifyCase>,
)

@Serializable
private data class VSlot(
    val available: Boolean,
    val used: Double?,
    val unit: String,
    val status: String,
)

@Serializable
private data class ToneCase(
    val name: String,
    val slot: VSlot,
    val want: String,
)

@Serializable
private data class PercentCase(
    val percent: Double,
    val want: String,
)

@Serializable
private data class ToneFile(
    val description: String,
    val cases: List<ToneCase>,
    @SerialName("percent_cases") val percentCases: List<PercentCase>,
)

class VectorsDetectClassifyTest {
    @Test
    fun detectMatchesVectors() {
        val file = Vectors.load<DetectFile>("detect.json")
        Vectors.requireMin("detect.json の cases", file.cases.size, MIN_DETECT)
        Vectors.requireMin("detect.json の detect_cycles", file.cycles.size, MIN_CYCLES)
        for (case in file.cases) {
            val known = case.known.map { it.toKey() }
            val panes = case.panes.map { Pane(id = it.id, pos = it.pos, state = PaneState.parse(it.state)) }
            val knownBefore = known.toList()
            val panesBefore = panes.toList()

            val diff = detect(known, panes, case.baselining)

            assertEquals(case.name, case.notify.map { it.toKey() }, diff.notify.map { NotifiedKey(it.pane.id, it.state) })
            assertEquals(case.name, case.insert.map { it.toKey() }, diff.insert)
            assertEquals(case.name, case.delete.map { it.toKey() }, diff.delete)
            assertEquals("${case.name}: 入力を変更してはならない", knownBefore, known)
            assertEquals("${case.name}: 入力を変更してはならない", panesBefore, panes)
        }
        for (case in file.cycles) {
            var known = emptyList<NotifiedKey>()
            for ((index, step) in case.steps.withIndex()) {
                val pane = Pane(id = "%1", pos = "T1", state = PaneState.parse(step.state))
                val diff = detect(known, listOf(pane), false)
                assertEquals("${case.name}: step $index (${step.state})", step.notify, diff.notify.size)
                known = known.filter { it !in diff.delete } + diff.insert
            }
        }
    }

    @Test
    fun classifyMatchesVectors() {
        val file = Vectors.load<ClassifyFile>("classify.json")
        Vectors.requireMin("classify.json", file.cases.size, MIN_CLASSIFY)
        val kinds = ErrKind.entries.associateBy { it.id }
        for (case in file.cases) {
            val error: Throwable? =
                when (case.error) {
                    "none" -> null
                    "network" -> java.io.IOException("network")
                    "invalid_response" -> InvalidResponseException()
                    "fetch_error" -> FetchException(kinds.getValue(checkNotNull(case.fetchKind)), case.fetchCode.orEmpty())
                    else -> error("${case.name}: 未知の error ${case.error}")
                }
            val got = classify(error, case.httpStatus, case.toolCode)
            if (case.want == null) {
                assertNull(case.name, got)
                continue
            }
            assertNotNull(case.name, got)
            assertEquals(case.name, kinds.getValue(case.want.kind), got!!.kind)
            assertEquals(case.name, case.want.code, got.code)
            val message = got.message.orEmpty()
            val untrusted = case.toolCode.length > SHORT_INPUT && case.want.code != case.toolCode
            assertFalse("${case.name}: メッセージに信頼できないコードが含まれる", untrusted && message.contains(case.toolCode))
        }
    }

    @Test
    fun usageToneMatchesVectors() {
        val file = Vectors.load<ToneFile>("usage_tone.json")
        Vectors.requireMin("usage_tone.json の cases", file.cases.size, MIN_TONE)
        Vectors.requireMin("usage_tone.json の percent_cases", file.percentCases.size, MIN_PERCENT)
        for (case in file.cases) {
            val slot = Slot(case.slot.available, case.slot.used, case.slot.unit, case.slot.status)
            assertEquals(case.name, case.want, usageTone(slot).id)
        }
        for (case in file.percentCases) {
            assertEquals("percent ${case.percent}", case.want, percentTone(case.percent).id)
        }
    }
}
