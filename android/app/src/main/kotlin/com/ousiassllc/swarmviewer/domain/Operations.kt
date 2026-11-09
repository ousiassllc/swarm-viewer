package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.JsonObject

// 操作モード（機能要件 §4.7、API 仕様 §1.2）のツール結果。`content[0].text` の JSON から解析する。
// 規則は [parseReport] と同じ: 未知のキーは無視、欠落・null は既定値、型が想定外・JSON が壊れているときは [InvalidResponseException]
// （`classify` が protocol に分類する）。応答の本文はエラーに含めない。デスクトップ版（Go の domain）と同じ形・同じ名前。

/** `pane_tail`。[text] は本文（等幅で、テキストとして描画する）、[truncated] は行数の上限で切ったか。 */
data class PaneTail(
    val text: String = "",
    val truncated: Boolean = false,
)

/** 質問カードの選択肢 1 つ。[n] は 1 始まりの番号。 */
data class AskOption(
    val n: Int = 0,
    val label: String = "",
    val recommended: Boolean = false,
)

/** `ask_card`。[asking] が false のとき、ほかの項目は既定値。 */
data class AskCard(
    val asking: Boolean = false,
    val question: String = "",
    val options: List<AskOption> = emptyList(),
    val cursor: Int = 0,
    val allowsCustom: Boolean = false,
)

/** `ask_pick`。[pickedN]・[pickedLabel] は `picked`（`text` を送ったときは無い）。 */
data class PickResult(
    val ok: Boolean = false,
    val state: String = "",
    val pickedN: Int = 0,
    val pickedLabel: String = "",
    val cardClosed: Boolean = false,
)

/** `send_text`。[accepted] はペインが受理（入力欄が空になって `working`）したか。 */
data class SendResult(
    val ok: Boolean = false,
    val state: String = "",
    val accepted: Boolean = false,
    val chars: Int = 0,
)

/** `send_escape`。 */
data class EscapeResult(
    val ok: Boolean = false,
    val state: String = "",
    val interrupted: Boolean = false,
)

/** `pane_start`。 */
data class StartResult(
    val ok: Boolean = false,
    val state: String = "",
    val agent: String = "",
    val cwd: String = "",
    val title: String = "",
)

/** `pane_close`。 */
data class CloseResult(
    val ok: Boolean = false,
    val state: String = "",
    val noteArchived: Boolean = false,
)

/** `dir_list` の 1 行。[git] は直下に `.git` があるフォルダ。[path] はそのまま `pane_start` の `cwd`・次の `dir_list` の `path` に渡せる。 */
data class DirEntry(
    val name: String = "",
    val path: String = "",
    val git: Boolean = false,
)

/** `dir_list`。[path] は実パス（`path` を省略した一覧では空）。[truncated] は件数の上限で一部しか返していない。 */
data class DirList(
    val path: String = "",
    val entries: List<DirEntry> = emptyList(),
    val truncated: Boolean = false,
)

fun parsePaneTail(data: String): PaneTail {
    val root = parseJsonObject(data)
    return PaneTail(text = root["text"].string(), truncated = root["truncated"].bool())
}

/** `asking` が false のときは、ほかの項目が付いていても捨てる（質問は無い）。 */
fun parseAskCard(data: String): AskCard {
    val root = parseJsonObject(data)
    if (!root["asking"].bool()) return AskCard()
    return AskCard(
        asking = true,
        question = root["question"].string(),
        options = root["options"].listOrEmpty().map { it.objectOrNull().toAskOption() },
        cursor = root["cursor"].int(),
        allowsCustom = root["allows_custom"].bool(),
    )
}

fun parsePick(data: String): PickResult {
    val root = parseJsonObject(data)
    val picked = root["picked"].objectOrNull()
    return PickResult(
        ok = root["ok"].bool(),
        state = root["state"].string(),
        pickedN = picked?.get("n").int(),
        pickedLabel = picked?.get("label").string(),
        cardClosed = root["card_closed"].bool(),
    )
}

fun parseSend(data: String): SendResult {
    val root = parseJsonObject(data)
    return SendResult(
        ok = root["ok"].bool(),
        state = root["state"].string(),
        accepted = root["accepted"].bool(),
        chars = root["chars"].int(),
    )
}

fun parseEscape(data: String): EscapeResult {
    val root = parseJsonObject(data)
    return EscapeResult(ok = root["ok"].bool(), state = root["state"].string(), interrupted = root["interrupted"].bool())
}

fun parseStart(data: String): StartResult {
    val root = parseJsonObject(data)
    return StartResult(
        ok = root["ok"].bool(),
        state = root["state"].string(),
        agent = root["agent"].string(),
        cwd = root["cwd"].string(),
        title = root["title"].string(),
    )
}

fun parseClose(data: String): CloseResult {
    val root = parseJsonObject(data)
    return CloseResult(ok = root["ok"].bool(), state = root["state"].string(), noteArchived = root["note_archived"].bool())
}

fun parseDirList(data: String): DirList {
    val root = parseJsonObject(data)
    return DirList(
        path = root["path"].string(),
        // name か path が空の項目は捨てる（選べない・たどれない行を出さない）。
        entries =
            root["entries"]
                .listOrEmpty()
                .map {
                    it.objectOrNull().toDirEntry()
                }.filter { it.name.isNotEmpty() && it.path.isNotEmpty() },
        truncated = root["truncated"].bool(),
    )
}

private fun JsonObject?.toAskOption(): AskOption =
    if (this == null) {
        AskOption()
    } else {
        AskOption(n = get("n").int(), label = get("label").string(), recommended = get("recommended").bool())
    }

private fun JsonObject?.toDirEntry(): DirEntry =
    if (this == null) DirEntry() else DirEntry(name = get("name").string(), path = get("path").string(), git = get("git").bool())
