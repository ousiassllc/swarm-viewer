package com.ousiassllc.swarmviewer.ui.pane

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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.DirEntry
import com.ousiassllc.swarmviewer.ui.main.Badge
import com.ousiassllc.swarmviewer.ui.main.DisplayTone
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily

/**
 * フォルダピッカー。`dir_list`（`path` 省略 = `allowed_roots`）から 1 階層ずつたどる。パンくず（現在の階層。表示のみ）・戻る（1 つ上へ）・
 * 「このフォルダを選ぶ」・各行をタップで下へ。`git: true` のフォルダに印。`truncated: true` なら「一部のみ表示」。
 * 最近使ったフォルダ（report の `agent` ありのペインの `cwd`）は先に出す。名前・パスはテキストとして描画する。
 */
@Composable
fun FolderPicker(
    state: StartSessionUiState,
    viewModel: StartSessionViewModel,
) {
    val enabled = !state.busy
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        state.folder?.let { Text("選択中: $it", fontFamily = MonoFontFamily, style = MaterialTheme.typography.bodyMedium) }
        if (state.recent.isNotEmpty()) {
            MutedText("最近使ったフォルダ")
            state.recent.forEach { path ->
                AppOutlinedButton(onClick = { viewModel.chooseRecent(path) }, enabled = enabled, modifier = Modifier.bigButton()) {
                    Text(path, fontFamily = MonoFontFamily, maxLines = 1)
                }
            }
        }
        val listing = state.listing
        if (listing == null) {
            MutedText(if (state.listingLoading) "読み込み中…" else "フォルダを読み込めませんでした")
            if (!state.listingLoading) {
                AppOutlinedButton(onClick = viewModel::retryListing, enabled = enabled, modifier = Modifier.bigButton()) {
                    Text("再読み込み")
                }
            }
            return@Column
        }
        MutedText("現在の場所: ${listing.path ?: "（許可されたフォルダ）"}")
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            AppOutlinedButton(
                onClick = viewModel::up,
                enabled = enabled && state.canGoUp && !state.listingLoading,
                modifier = Modifier.heightIn(min = TouchTargetHeight),
            ) { Text("1 つ上へ") }
            if (state.canChooseListed) {
                AppButton(onClick = viewModel::chooseListed, enabled = enabled, modifier = Modifier.heightIn(min = TouchTargetHeight)) {
                    Text("このフォルダを選ぶ")
                }
            }
        }
        if (listing.truncated) MutedText("一部のみ表示しています")
        DirEntries(listing.entries, enabled && !state.listingLoading, viewModel::browse)
    }
}

@Composable
private fun DirEntries(
    entries: List<DirEntry>,
    enabled: Boolean,
    onOpen: (String) -> Unit,
) {
    if (entries.isEmpty()) {
        MutedText("下の階層はありません")
        return
    }
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column {
            entries.forEachIndexed { index, entry ->
                if (index > 0) HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                Row(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .heightIn(min = TouchTargetHeight)
                            .clickable(enabled = enabled, role = Role.Button) { onOpen(entry.path) }
                            .padding(horizontal = 16.dp, vertical = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(entry.name, modifier = Modifier.weight(1f), maxLines = 1, fontFamily = MonoFontFamily)
                    if (entry.git) Badge("git", DisplayTone.Muted)
                }
            }
        }
    }
}
