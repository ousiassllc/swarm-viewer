package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.Attention
import com.ousiassllc.swarmviewer.domain.Auth
import com.ousiassllc.swarmviewer.domain.Warning
import com.ousiassllc.swarmviewer.ui.theme.AlertKind
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily
import java.time.Instant

/**
 * 要対応（`attention`）と警告（`warnings`）。ペイン一覧の上に出す（機能要件 §7）。どちらも空なら何も描画しない。
 * 警告は `message` を出す。`attention` はペイン関連とログイン関連の 2 グループに分け、間とエージェントごとに区切り線を入れる。
 * `message`・`detail` は信頼できない入力として、常にテキストで描画する。
 */
@Composable
fun AttentionBar(
    attention: List<Attention>,
    warnings: List<Warning>,
    auth: Auth,
    stale: Boolean,
) {
    if (attention.isEmpty() && warnings.isEmpty()) return
    Column(modifier = Modifier.grayedOut(stale), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        warnings.forEach { WarningBanner(it.message) }
        if (attention.isNotEmpty()) AttentionList(groupAttention(attention), auth)
    }
}

@Composable
private fun WarningBanner(message: String) {
    AppAlert(message, kind = AlertKind.Warning)
}

@Composable
private fun AttentionList(
    groups: AttentionGroups,
    auth: Auth,
) {
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column {
            if (groups.panes.isNotEmpty()) {
                Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    groups.panes.forEach { AttentionRow(it, auth) }
                }
            }
            if (groups.panes.isNotEmpty() && groups.agents.isNotEmpty()) HorizontalDivider()
            groups.agents.forEachIndexed { index, agent ->
                if (index > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    agent.items.forEach { AttentionRow(it, auth) }
                }
            }
        }
    }
}

@Composable
private fun AttentionRow(
    item: Attention,
    auth: Auth,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
        Badge(attentionLabel(item.kind), attentionTone(item.kind))
        if (item.pane.isNotEmpty()) Text(item.pane, style = MaterialTheme.typography.bodyMedium, fontFamily = MonoFontFamily)
        val detail = attentionDetail(item.kind, item.detail)
        if (detail.isNotEmpty()) {
            Text(
                detail,
                style = MaterialTheme.typography.bodyMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f, fill = false),
            )
        }
        authExpiresAt(auth, item)?.let { MutedText("期限 " + formatDateTimeWithYear(it)) }
    }
}

/** `auth_expired` / `auth_expiring` の `detail`（エージェント名）に対応する `auth.agents[].expires_at`。無ければ null。 */
private fun authExpiresAt(
    auth: Auth,
    item: Attention,
): Instant? = if (isAuthKind(item.kind)) auth.agents.firstOrNull { it.agent == item.detail }?.expiresAt else null
