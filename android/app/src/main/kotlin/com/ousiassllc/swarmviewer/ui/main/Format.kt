package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Note
import com.ousiassllc.swarmviewer.domain.UsageStatus
import java.math.BigDecimal
import java.time.Duration
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

// 画面表示のための書式（デスクトップ版の internal/web/format/text.go と同じ文言・規則。機能要件 §3・§5〜§7）。
// 時刻を扱う関数は zone・now を引数に取る（テストで固定できる）。

/** 利用できない値（`available: false`）の表示。 */
const val DASH = "—"

private const val TENTH = 10.0
private const val MAX_NOTE_ITEMS = 3
private const val HOURS_PER_DAY = 24L

/** 使用率を小数第 1 位まで、末尾の 0 は落として「80%」「79.9%」の形にする。NaN・無限大は [DASH]。 */
fun formatPercent(value: Double): String = if (value.isFinite()) "${trimmed(value)}%" else DASH

/** 金額を「$12.50」の形にする。NaN・無限大は [DASH]。 */
fun formatUsd(value: Double): String = if (value.isFinite()) "$" + String.format(Locale.ROOT, "%.2f", value) else DASH

/** 容量（GB）を小数第 1 位で「12.3 GB」の形にする。NaN・無限大は [DASH]。 */
fun formatGb(value: Double): String = if (value.isFinite()) "${trimmed(value)} GB" else DASH

/** 小数第 1 位に丸め（0.5 は 0 から遠い側）、末尾の 0 を落とす。 */
private fun trimmed(value: Double): String {
    val rounded = if (value < 0) -Math.round(-value * TENTH) / TENTH else Math.round(value * TENTH) / TENTH
    return BigDecimal.valueOf(rounded).stripTrailingZeros().toPlainString()
}

/** プログレスバーの割合（0〜1）。使用率（%）を 0〜100 に丸める。NaN は 0。 */
fun barFraction(percent: Double): Float = if (percent.isNaN()) 0f else (percent.coerceIn(0.0, 100.0) / 100.0).toFloat()

private val clockFormat = DateTimeFormatter.ofPattern("HH:mm", Locale.ROOT)
private val dateTimeFormat = DateTimeFormatter.ofPattern("MM/dd HH:mm", Locale.ROOT)
private val dateTimeWithYearFormat = DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm", Locale.ROOT)

/** ローカル時刻の「HH:MM」。 */
fun formatClock(
    at: Instant,
    zone: ZoneId = ZoneId.systemDefault(),
): String = clockFormat.format(at.atZone(zone))

/** ローカル時刻の「MM/DD HH:MM」。 */
fun formatDateTime(
    at: Instant,
    zone: ZoneId = ZoneId.systemDefault(),
): String = dateTimeFormat.format(at.atZone(zone))

/** ローカル時刻の「YYYY/MM/DD HH:MM」。年をまたぐ期限を誤読させたくない箇所（ログイン期限）で使う。 */
fun formatDateTimeWithYear(
    at: Instant,
    zone: ZoneId = ZoneId.systemDefault(),
): String = dateTimeWithYearFormat.format(at.atZone(zone))

/** [at] までの残り時間を「あと N 分／時間／日」にする。1 分未満・過去は「まもなく」。端数は切り捨てる（機能要件 §5.2）。 */
fun formatUntil(
    at: Instant,
    now: Instant,
): String {
    val remaining = Duration.between(now, at)
    return when {
        remaining < Duration.ofMinutes(1) -> "まもなく"
        remaining < Duration.ofHours(1) -> "あと ${remaining.toMinutes()} 分"
        remaining < Duration.ofHours(HOURS_PER_DAY) -> "あと ${remaining.toHours()} 時間"
        else -> "あと ${remaining.toDays()} 日"
    }
}

/** 空でない要素を [separator] で連結する。 */
fun joinNonEmpty(
    separator: String,
    vararg parts: String,
): String = parts.filter { it.isNotEmpty() }.joinToString(separator)

/** `attention.kind` の表示ラベル（機能要件 §7）。未知の kind は文字列のまま返す。 */
fun attentionLabel(kind: String): String =
    when (kind) {
        "asking" -> "答え待ち"
        "stalled" -> "止まってるかも"
        "report_waiting" -> "報告待ち"
        "auth_expired" -> "ログイン期限切れ"
        "auth_expiring" -> "ログイン期限間近"
        else -> kind
    }

/** 要対応 1 件に添える detail。`report_waiting` の detail はサーバー上のファイルパスなので出さない（機能要件 §7）。 */
fun attentionDetail(
    kind: String,
    detail: String,
): String = if (kind == "report_waiting") "" else detail

/** `usage.status` ごとの本文（`ok` 以外）。`ok` は空文字。未知の status は `unavailable` として扱う（機能要件 §5.1）。 */
fun usageStatusLabel(status: String): String =
    when (UsageStatus.normalize(status)) {
        UsageStatus.OK -> ""
        UsageStatus.STALE -> "古いデータ"
        UsageStatus.PENDING -> "取得中"
        UsageStatus.DISABLED -> "利用上限の取得は無効です"
        else -> "取得できません"
    }

/** 数字を出す `usage.status`（`ok`・`stale`）か。それ以外は数字を出さない。 */
fun usageShowsNumbers(status: String): Boolean = UsageStatus.normalize(status).let { it == UsageStatus.OK || it == UsageStatus.STALE }

/**
 * 利用上限のアカウントの `reason`（枠を取れなかった理由）の表示文言。原因は swarm-console からは確かには分からないため
 * 「可能性」と書く。null・知らない値は理由なし（空文字。機能要件 §5.2）。
 */
fun usageReasonLabel(reason: String?): String =
    when (reason) {
        "without_usage" -> "ログイン切れの可能性"
        "no_email" -> "アカウントを特定できません（ログイン切れの可能性）"
        else -> ""
    }

/** Cursor の `window` の表示名。null・空は空文字、未知の値はそのまま。 */
fun cursorWindowLabel(window: String?): String =
    when (window) {
        null -> ""
        "monthly" -> "月間"
        "weekly" -> "週間"
        else -> window
    }

/** メモ要約（`repo` と `items` 先頭 3 件、残りの件数。機能要件 §4.4）。 */
data class NoteSummary(
    val repo: String,
    val items: List<String>,
    val more: Int,
) {
    val isEmpty: Boolean get() = repo.isEmpty() && items.isEmpty()
}

fun noteSummary(note: Note?): NoteSummary =
    NoteSummary(
        repo = note?.repo.orEmpty(),
        items = note?.items.orEmpty().take(MAX_NOTE_ITEMS),
        more = ((note?.items?.size ?: 0) - MAX_NOTE_ITEMS).coerceAtLeast(0),
    )
