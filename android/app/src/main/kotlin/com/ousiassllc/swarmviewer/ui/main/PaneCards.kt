package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.Attention
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.Layout
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.ui.pane.paneControls
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppEmptyState
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily

/**
 * ペインの表示。[wide] で切り替える（機能要件 §12.2）: false（Compact）は `pos` 順（report の順）の縦 1 列、
 * true（Medium / Expanded）は `layout` のグリッド（T* / B*）。`layout` が欠落・不整合なら広い画面でもリストで出す。
 * カードのタップは [onOpen]（ペイン詳細）。操作の接続（[kind] が Ops）の `shell` のカードには「＋ 始める」（[onStart]）を出す。
 */
@Composable
fun PaneSection(
    panes: List<Pane>,
    layout: Layout,
    attention: List<Attention>,
    stale: Boolean,
    wide: Boolean,
    kind: ConnectionKind,
    onOpen: (Pane) -> Unit,
    onStart: (Pane) -> Unit,
) {
    if (panes.isEmpty()) {
        AppEmptyState("ペインがありません")
        return
    }
    val rows = if (wide) gridRows(layout, panes) else null
    // ペインカード同士の間隔は 12dp（ガイドのグリッド密 `--gap-grid-dense`。Web の `PaneGrid` の `gap-3`）。
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        if (rows == null) {
            panes.forEach { PaneCard(it, attention, stale, kind, onOpen, onStart, Modifier.fillMaxWidth()) }
        } else {
            rows.forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    row.forEach { PaneCard(it, attention, stale, kind, onOpen, onStart, Modifier.weight(1f)) }
                }
            }
        }
    }
}

/**
 * ペイン 1 つのカード（機能要件 §4.4）。`title`・`activity`・`note` は信頼できない入力として、常にテキストとして描画する
 * （NFR-SEC-10）。タップでペイン詳細を開くだけで、何も送らない（操作は詳細・新規セッションの画面で、確認を経て行う）。
 * `asking` / `stalled` の要対応は枠で強調する。
 */
@Composable
private fun PaneCard(
    pane: Pane,
    attention: List<Attention>,
    stale: Boolean,
    kind: ConnectionKind,
    onOpen: (Pane) -> Unit,
    onStart: (Pane) -> Unit,
    modifier: Modifier,
) {
    val highlight = hasAttention(attention, pane.pos, "asking", "stalled")
    val reportWaiting = hasAttention(attention, pane.pos, "report_waiting")
    val maxLines = 1
    AppCard(
        modifier = modifier.grayedOut(stale).clickable { onOpen(pane) },
        borderColor = if (highlight) DisplayTone.Warning.color() else MaterialTheme.colorScheme.outline,
        borderWidth = if (highlight) 2.dp else 1.dp,
    ) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            PaneHeader(pane, reportWaiting)
            Text(pane.title, style = MaterialTheme.typography.bodyLarge, maxLines = maxLines, overflow = TextOverflow.Ellipsis)
            if (pane.activity.isNotEmpty()) {
                val activity = if (pane.activityTruncated) pane.activity + "…" else pane.activity
                Text(activity, style = MaterialTheme.typography.bodyMedium, maxLines = maxLines, overflow = TextOverflow.Ellipsis)
            }
            NoteBlock(pane)
            if (paneControls(kind, pane.state, null).showStart) {
                AppButton(onClick = { onStart(pane) }, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) { Text("＋ 始める") }
            }
        }
    }
}

@Composable
private fun PaneHeader(
    pane: Pane,
    reportWaiting: Boolean,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(pane.pos, style = MaterialTheme.typography.bodyMedium, fontFamily = MonoFontFamily)
        pane.agent?.takeIf { it.isNotEmpty() }?.let { Badge(it, DisplayTone.Muted) }
        Badge(pane.state.label, stateTone(pane.state), marker = true)
        if (reportWaiting) Badge(attentionLabel("report_waiting"), attentionTone("report_waiting"), marker = true)
        if (pane.state == PaneState.Stalled && pane.changedAgo.isNotEmpty()) MutedText(pane.changedAgo)
        if (pane.elapsed.isNotEmpty()) {
            Spacer(Modifier.weight(1f))
            MutedText(pane.elapsed)
        }
    }
}

/** メモ要約（`repo` と `items` 先頭 3 件、残りは `+N`）。全件はペイン詳細で出す。 */
@Composable
private fun NoteBlock(pane: Pane) {
    val summary = noteSummary(pane.note)
    if (summary.isEmpty) return
    // 項目間 8dp（ガイド §4 リスト `--list-gap`）。ペインカードの他の要素の間隔（4dp）より広い。
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (summary.repo.isNotEmpty()) MutedText(summary.repo)
        summary.items.forEach { NoteItem(it, maxLines = 1) }
        if (summary.more > 0) MutedText("+${summary.more}")
    }
}

/** メモの 1 項目。記号は 5dp の点（`--list-marker-size`・muted）で、記号→文字は 10dp（`--list-marker-gap`）。点は 1 行目の中央に置く。 */
@Composable
private fun NoteItem(
    text: String,
    maxLines: Int,
) {
    val style = MaterialTheme.typography.bodyMedium
    val lineHeight = with(LocalDensity.current) { style.lineHeight.toDp() }
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        Box(
            Modifier
                .padding(top = (lineHeight - 5.dp) / 2)
                .size(5.dp)
                .background(MaterialTheme.colorScheme.onSurfaceVariant, CircleShape),
        )
        Text(text, style = style, maxLines = maxLines, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
    }
}
