package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
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
 * ペインを指す項目（`pane` あり）のタップは [onOpenPane]（ペイン詳細。`asking` はその質問カードへ直接）。
 */
@Composable
fun AttentionBar(
    attention: List<Attention>,
    warnings: List<Warning>,
    auth: Auth,
    stale: Boolean,
    onOpenPane: (String) -> Unit,
) {
    if (attention.isEmpty() && warnings.isEmpty()) return
    Column(modifier = Modifier.grayedOut(stale), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        warnings.forEach { WarningBanner(it.message) }
        if (attention.isNotEmpty()) AttentionList(groupAttention(attention), auth, onOpenPane)
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
    onOpenPane: (String) -> Unit,
) {
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column {
            if (groups.panes.isNotEmpty()) {
                Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    groups.panes.forEach { AttentionRow(it, auth, onOpenPane) }
                }
            }
            if (groups.panes.isNotEmpty() && groups.agents.isNotEmpty()) HorizontalDivider()
            groups.agents.forEachIndexed { index, agent ->
                if (index > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    agent.items.forEach { AttentionRow(it, auth, onOpenPane) }
                }
            }
        }
    }
}

@Composable
private fun AttentionRow(
    item: Attention,
    auth: Auth,
    onOpenPane: (String) -> Unit,
) {
    val open = if (item.pane.isNotEmpty()) Modifier.heightIn(min = 48.dp).clickable { onOpenPane(item.pane) } else Modifier
    Row(
        modifier = open.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Badge(attentionLabel(item.kind), attentionTone(item.kind), marker = true)
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
