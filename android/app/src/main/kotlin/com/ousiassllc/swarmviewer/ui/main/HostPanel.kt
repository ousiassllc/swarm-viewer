package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.Host
import com.ousiassllc.swarmviewer.domain.percentTone

private val HOST_LABEL_WIDTH = 72.dp
private val HOST_VALUE_WIDTH = 132.dp

/**
 * ホスト負荷（CPU・メモリ・スワップ・ディスク。機能要件 §6）。各行は「ラベル・バー・値」の 3 列で、バーの両端が全行でそろう。
 * しきい値の色は利用上限と同じ規則（domain の [percentTone]）。マウント名はホスト由来の文字列としてテキストで描画する。
 */
@Composable
fun HostPanel(
    host: Host,
    stale: Boolean,
) {
    OutlinedCard(modifier = Modifier.fillMaxWidth().grayedOut(stale)) {
        Column(modifier = Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text("ホスト", style = MaterialTheme.typography.titleSmall)
            hostRows(host).forEach { HostRow(it) }
        }
    }
}

@Composable
private fun HostRow(row: HostRowModel) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        MutedText(row.label, modifier = Modifier.width(HOST_LABEL_WIDTH))
        when {
            !row.available -> {
                Spacer(Modifier.weight(1f))
                ValueText(DASH)
            }
            row.noBar -> {
                Spacer(Modifier.weight(1f))
                ValueText(row.detail)
            }
            else -> {
                val tone = displayTone(percentTone(row.percent))
                ToneBar(row.percent, tone, Modifier.weight(1f))
                ValueText(joinNonEmpty(" ", formatPercent(row.percent), row.detail), tone.valueColor())
            }
        }
    }
}

@Composable
private fun ValueText(
    text: String,
    color: Color = MaterialTheme.colorScheme.onSurfaceVariant,
) {
    Text(
        text,
        color = color,
        fontFamily = FontFamily.Monospace,
        style = MaterialTheme.typography.bodySmall,
        textAlign = TextAlign.End,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.width(HOST_VALUE_WIDTH),
    )
}
