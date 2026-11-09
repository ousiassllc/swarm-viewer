package com.ousiassllc.swarmviewer.domain

// 操作モードの入力の検査（機能要件 §4.7）。swarm-console に送る前に弾くので、通信しない。結果のコードは `ErrKind.Operation` と同じ語彙
// （画面は固定文言を引く）。デスクトップ版（Go の domain）と同じ規則。

/** `send_text`・`ask_pick`（自由入力）の本文の上限（文字数）。 */
const val MAX_SEND_CHARS = 8000

/** 新しいセッションのタイトルの上限（文字数）。 */
const val MAX_TITLE_CHARS = 80

/** `pane_start` で選べるエージェント（表示順）。 */
val START_AGENTS: List<String> = listOf("omp", "claude", "codex")

/** `pane_tail` の既定の行数と上限。 */
const val DEFAULT_TAIL_LINES = 30
const val MAX_TAIL_LINES = 200

/** `pane_tail` の `lines` を整える（0 以下は既定の 30、上限 200 に収める）。 */
fun normalizeTailLines(lines: Int): Int = if (lines <= 0) DEFAULT_TAIL_LINES else minOf(lines, MAX_TAIL_LINES)

/** 「最近使ったフォルダ」の最大件数。 */
const val MAX_RECENT_FOLDERS = 5

/**
 * 送る本文の問題。無ければ null、あれば [ErrKind.Operation] のコード: 空は `invalid_args`、[MAX_SEND_CHARS] 超過は `text_too_long`、
 * 改行（`\n`）・タブ以外の制御文字（`\r` を含む）は `forbidden_control_char`。
 */
fun sendTextProblem(text: String): String? =
    when {
        text.isEmpty() -> "invalid_args"
        text.codePointCount(0, text.length) > MAX_SEND_CHARS -> "text_too_long"
        text.any { it != '\n' && it != '\t' && isGoControl(it) } -> "forbidden_control_char"
        else -> null
    }

/**
 * タイトルを検査して前後の空白（Go の `strings.TrimSpace` と同じ範囲）を除いた値を返す。1〜[MAX_TITLE_CHARS] 文字で、制御文字
 * （改行・タブを含む）を含まないものだけ。違えば null。
 */
fun normalizeTitle(raw: String): String? {
    val title = raw.trimGoSpace()
    if (title.codePointCount(0, title.length) !in 1..MAX_TITLE_CHARS || title.hasControl()) return null
    return title
}

/** フォルダの選択が空・制御文字を含まないか（`pane_start` の `cwd`）。 */
fun validCwd(cwd: String): Boolean = cwd.isNotEmpty() && !cwd.hasControl()

/**
 * 「最近使ったフォルダ」: `panes[]` のうち `agent` があるペインの `cwd` を、重複なしで最大 [MAX_RECENT_FOLDERS] 件（report の順）。
 * 保存はしない。許可外のフォルダは `pane_start` が `forbidden_path` を返す。
 */
fun recentFolders(panes: List<Pane>): List<String> =
    panes
        .filter { !it.agent.isNullOrEmpty() && it.cwd.isNotEmpty() }
        .map { it.cwd }
        .distinct()
        .take(MAX_RECENT_FOLDERS)
