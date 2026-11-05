package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.domain.Vectors
import com.ousiassllc.swarmviewer.domain.parseReport
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

private const val MIN_CASES = 20

class SnapshotCodecTest {
    /** 保存した JSON を読み戻すと、元の Report と等しい（共有ベクタの全ケースで）。 */
    @Test
    fun everyVectorReportSurvivesTheRoundTrip() {
        val cases =
            Json
                .parseToJsonElement(Vectors.read("report.json"))
                .jsonObject
                .getValue("parse_report")
                .jsonArray
        Vectors.requireMin("report.json の parse_report", cases.size, MIN_CASES)
        for (case in cases) {
            val name = case.jsonObject.getValue("name").toString()
            val report = parseReport(case.jsonObject.getValue("input").toString())

            assertEquals(name, report, decodeReport(encodeReport(report)))
        }
    }

    @Test
    fun unknownKeysAreNotStored() {
        val report =
            parseReport("""{"layout":{"rows":1,"cols":[1]},"panes":[{"id":"%1","state":"idle","future":"x"}],"future_key":{"x":1}}""")

        val json = encodeReport(report)

        assertFalse(json.contains("future"))
        assertEquals(report, decodeReport(json))
    }
}
