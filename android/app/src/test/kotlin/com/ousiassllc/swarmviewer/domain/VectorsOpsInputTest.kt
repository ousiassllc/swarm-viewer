package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

private const val MIN_TEXT_CASES = 12
private const val MIN_TITLE_CASES = 10
private const val MIN_TAIL_CASES = 6

@Serializable
private data class Repeat(
    val char: String,
    val count: Int,
)

@Serializable
private data class TextCase(
    val name: String,
    val text: String = "",
    val repeat: Repeat? = null,
    val want: String,
)

@Serializable
private data class TitleCase(
    val name: String,
    val input: String,
    val want: String? = null,
)

@Serializable
private data class TailCase(
    val name: String,
    val lines: Int,
    val want: Int,
)

@Serializable
private data class OpsInputFile(
    val description: String,
    @SerialName("text_cases") val textCases: List<TextCase>,
    @SerialName("title_cases") val titleCases: List<TitleCase>,
    @SerialName("tail_lines_cases") val tailLinesCases: List<TailCase>,
)

/** 操作モードの入力の検査が、デスクトップ版と共有するテストベクタ（testdata/vectors/opsinput.json）と同じ結果になること。 */
class VectorsOpsInputTest {
    private val file = Vectors.load<OpsInputFile>("opsinput.json")

    @Test
    fun sendTextProblemMatchesVectors() {
        Vectors.requireMin("opsinput.json の text_cases", file.textCases.size, MIN_TEXT_CASES)
        for (case in file.textCases) {
            val text = case.repeat?.let { it.char.repeat(it.count) } ?: case.text
            assertEquals(case.name, case.want, sendTextProblem(text).orEmpty())
        }
    }

    @Test
    fun normalizeTitleMatchesVectors() {
        Vectors.requireMin("opsinput.json の title_cases", file.titleCases.size, MIN_TITLE_CASES)
        for (case in file.titleCases) {
            if (case.want ==
                null
            ) {
                assertNull(case.name, normalizeTitle(case.input))
            } else {
                assertEquals(case.name, case.want, normalizeTitle(case.input))
            }
        }
    }

    @Test
    fun normalizeTailLinesMatchesVectors() {
        Vectors.requireMin("opsinput.json の tail_lines_cases", file.tailLinesCases.size, MIN_TAIL_CASES)
        for (case in file.tailLinesCases) assertEquals(case.name, case.want, normalizeTailLines(case.lines))
    }
}
