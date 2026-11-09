package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test

private const val MIN_VALID_CASES = 35
private const val MIN_INVALID_CASES = 37
private const val SHORT_INPUT = 8

@Serializable
private data class ToolCase(
    val name: String,
    val input: String,
    val want: JsonObject,
)

@Serializable
private data class InvalidCase(
    val name: String,
    val tool: String,
    @SerialName("input_raw") val inputRaw: String,
)

@Serializable
private data class ToolResultFile(
    val description: String,
    @SerialName("pane_tail") val paneTail: List<ToolCase>,
    @SerialName("ask_card") val askCard: List<ToolCase>,
    @SerialName("ask_pick") val askPick: List<ToolCase>,
    @SerialName("send_text") val sendText: List<ToolCase>,
    @SerialName("send_escape") val sendEscape: List<ToolCase>,
    @SerialName("pane_start") val paneStart: List<ToolCase>,
    @SerialName("pane_close") val paneClose: List<ToolCase>,
    @SerialName("dir_list") val dirList: List<ToolCase>,
    @SerialName("skill_list") val skillList: List<ToolCase>,
    val invalid: List<InvalidCase>,
)

/** 操作ツールの結果の解析が、デスクトップ版と共有するテストベクタ（testdata/vectors/toolresult.json）と同じ結果になること。 */
class VectorsToolResultTest {
    private val file = Vectors.load<ToolResultFile>("toolresult.json")

    private val parsers: Map<String, (String) -> JsonElement> =
        mapOf(
            "pane_tail" to { parsePaneTail(it).toJson() },
            "ask_card" to { parseAskCard(it).toJson() },
            "ask_pick" to { parsePick(it).toJson() },
            "send_text" to { parseSend(it).toJson() },
            "send_escape" to { parseEscape(it).toJson() },
            "pane_start" to { parseStart(it).toJson() },
            "pane_close" to { parseClose(it).toJson() },
            "dir_list" to { parseDirList(it).toJson() },
            "skill_list" to { parseSkillList(it).toJson() },
        )

    @Test
    fun everyToolResultIsParsedAsInTheVectors() {
        val groups =
            mapOf(
                "pane_tail" to file.paneTail,
                "ask_card" to file.askCard,
                "ask_pick" to file.askPick,
                "send_text" to file.sendText,
                "send_escape" to file.sendEscape,
                "pane_start" to file.paneStart,
                "pane_close" to file.paneClose,
                "dir_list" to file.dirList,
                "skill_list" to file.skillList,
            )
        Vectors.requireMin("toolresult.json の正常系", groups.values.sumOf { it.size }, MIN_VALID_CASES)
        for ((tool, cases) in groups) {
            for (case in cases) {
                assertEquals("$tool: ${case.name}", case.want, parsers.getValue(tool)(case.input))
            }
        }
    }

    @Test
    fun malformedToolResultsAreInvalidResponsesThatDoNotEchoTheBody() {
        Vectors.requireMin("toolresult.json の invalid", file.invalid.size, MIN_INVALID_CASES)
        for (case in file.invalid) {
            val e = assertThrows(case.name, InvalidResponseException::class.java) { parsers.getValue(case.tool)(case.inputRaw) }
            assertFalse(case.name, case.inputRaw.length > SHORT_INPUT && e.message.orEmpty().contains(case.inputRaw))
        }
    }
}

private fun PaneTail.toJson() =
    buildJsonObject {
        put("text", text)
        put("truncated", truncated)
    }

private fun AskCard.toJson() =
    buildJsonObject {
        put("asking", asking)
        put("question", question)
        put(
            "options",
            buildJsonArray {
                options.forEach {
                    add(
                        buildJsonObject {
                            put("n", it.n)
                            put("label", it.label)
                            put("recommended", it.recommended)
                        },
                    )
                }
            },
        )
        put("cursor", cursor)
        put("allows_custom", allowsCustom)
    }

private fun PickResult.toJson() =
    buildJsonObject {
        put("ok", ok)
        put("state", state)
        put("picked_n", pickedN)
        put("picked_label", pickedLabel)
        put("card_closed", cardClosed)
    }

private fun SendResult.toJson() =
    buildJsonObject {
        put("ok", ok)
        put("state", state)
        put("accepted", accepted)
        put("chars", chars)
    }

private fun EscapeResult.toJson() =
    buildJsonObject {
        put("ok", ok)
        put("state", state)
        put("interrupted", interrupted)
    }

private fun StartResult.toJson() =
    buildJsonObject {
        put("ok", ok)
        put("state", state)
        put("agent", agent)
        put("cwd", cwd)
        put("title", title)
    }

private fun CloseResult.toJson() =
    buildJsonObject {
        put("ok", ok)
        put("state", state)
        put("note_archived", noteArchived)
    }

private fun DirList.toJson(): JsonElement =
    buildJsonObject {
        put("path", path)
        put(
            "entries",
            JsonArray(
                entries.map {
                    buildJsonObject {
                        put("name", it.name)
                        put("path", it.path)
                        put("git", it.git)
                    }
                },
            ),
        )
        put("truncated", truncated)
    }

private fun SkillList.toJson(): JsonElement =
    buildJsonObject {
        put("agent", agent)
        put(
            "skills",
            JsonArray(
                skills.map {
                    buildJsonObject {
                        put("name", it.name)
                        put("description", it.description)
                        put("invoke", it.invoke)
                    }
                },
            ),
        )
        put("truncated", truncated)
    }
