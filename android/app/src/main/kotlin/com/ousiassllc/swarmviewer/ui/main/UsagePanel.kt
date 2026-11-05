package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.AgentAccount
import com.ousiassllc.swarmviewer.domain.CursorAccount
import com.ousiassllc.swarmviewer.domain.Slot
import com.ousiassllc.swarmviewer.domain.SlotValues
import com.ousiassllc.swarmviewer.domain.Usage
import com.ousiassllc.swarmviewer.domain.UsageStatus
import com.ousiassllc.swarmviewer.domain.usageTone
import com.ousiassllc.swarmviewer.ui.theme.AlertKind
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily
import java.time.Instant

private val LABEL_WIDTH = 72.dp
private val VALUE_WIDTH = 64.dp

/**
 * 利用上限パネル（機能要件 §5）。`usage.status` で表示を分ける: `ok` は通常、`stale` はグレーアウトして「古いデータ」と取得時刻、
 * `unavailable`・`pending`・`disabled` は数字を出さない。しきい値の色は domain の [usageTone]（80% 黄・95% 赤・`exhausted` 赤）。
 * ホスト全体の値で、タブを切り替えても同じ値になる。
 */
@Composable
fun UsagePanel(
    usage: Usage,
    stale: Boolean,
    now: Instant,
) {
    val status = UsageStatus.normalize(usage.status)
    val gray = stale || status == UsageStatus.STALE
    AppCard(modifier = Modifier.fillMaxWidth().grayedOut(gray)) {
        Column(modifier = Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
            Text("利用上限", style = MaterialTheme.typography.titleMedium)
            usageStatusLabel(status).takeIf { it.isNotEmpty() }?.let { AppAlert(it, kind = usageStatusAlertKind(status)) }
            if (usageShowsNumbers(status)) {
                usage.claude.forEach { AgentAccountBlock("Claude", it, now) }
                usage.codex.forEach { AgentAccountBlock("Codex", it, now) }
                usage.cursor.forEach { CursorAccountBlock(it, now) }
                usage.fetchedAt?.let { MutedText("最終取得 " + formatDateTime(it)) }
            }
            if (usage.reason.isNotEmpty() && (status == UsageStatus.STALE || status == UsageStatus.UNAVAILABLE)) {
                NoticeText(usage.reason) // `reason` はホスト由来の文字列。コードとしてそのまま出す
            }
        }
    }
}

/** `ok` 以外の `usage.status` をアラートの種類にする（`stale` は注意、`pending`・`disabled` はお知らせ、`unavailable`・未知はエラー）。 */
private fun usageStatusAlertKind(status: String): AlertKind =
    when (status) {
        UsageStatus.STALE -> AlertKind.Warning
        UsageStatus.PENDING, UsageStatus.DISABLED -> AlertKind.Info
        else -> AlertKind.Danger
    }

/** Claude / Codex のアカウント。2 枠とも取れないときは「取得できません」を 1 行で出し、`reason` があれば理由を添える。 */
@Composable
private fun AgentAccountBlock(
    agent: String,
    account: AgentAccount,
    now: Instant,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(joinNonEmpty(" · ", agent, account.account), style = MaterialTheme.typography.bodyMedium)
        if (!account.fiveHour.available && !account.sevenDay.available) {
            AppAlert(joinNonEmpty(" · ", "取得できません", usageReasonLabel(account.reason)), kind = AlertKind.Warning)
        } else {
            SlotRow("5 時間枠", account.fiveHour, now)
            SlotRow("7 日枠", account.sevenDay, now)
        }
    }
}

@Composable
private fun CursorAccountBlock(
    account: CursorAccount,
    now: Instant,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(joinNonEmpty(" · ", "Cursor", account.account), style = MaterialTheme.typography.bodyMedium)
        SlotRow(cursorWindowLabel(account.window).ifEmpty { "モデル" }, account.models, now)
    }
}

/**
 * 枠 1 つ。`unit` が `percent` はバー＋数値、`usd` は金額のみ（バー・しきい値色なし）。`available: false` は「—」。
 * リセット時刻は `resets_at` をローカル時刻と相対表示で併記する。
 */
@Composable
private fun SlotRow(
    label: String,
    slot: Slot,
    now: Instant,
) {
    val used = slot.used
    val tone = displayTone(usageTone(slot))
    Column {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            MutedText(label, modifier = Modifier.width(LABEL_WIDTH))
            when {
                !slot.available || used == null -> MutedText(DASH)
                slot.unit == SlotValues.UNIT_PERCENT -> {
                    ToneBar(used, tone, Modifier.weight(1f))
                    Text(
                        formatPercent(used),
                        color = tone.valueColor(),
                        fontFamily = MonoFontFamily,
                        textAlign = TextAlign.End,
                        style = MaterialTheme.typography.bodyMedium,
                        modifier = Modifier.width(VALUE_WIDTH),
                    )
                }
                slot.unit == SlotValues.UNIT_USD ->
                    Text(
                        formatUsd(used),
                        fontFamily = MonoFontFamily,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                else -> MutedText(DASH)
            }
            if (slot.available && slot.status == SlotValues.STATUS_EXHAUSTED) Badge("上限到達", DisplayTone.Error)
        }
        val resetsAt = slot.resetsAt
        if (slot.available && resetsAt != null) {
            MutedText(
                "リセット ${formatDateTime(resetsAt)}（${formatUntil(resetsAt, now)}）",
                modifier = Modifier.padding(start = LABEL_WIDTH + 8.dp),
            )
        }
    }
}
