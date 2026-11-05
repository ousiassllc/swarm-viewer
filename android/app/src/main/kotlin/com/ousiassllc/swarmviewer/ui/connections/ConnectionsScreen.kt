package com.ousiassllc.swarmviewer.ui.connections

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.service.ConnectionItem
import com.ousiassllc.swarmviewer.ui.NEEDS_REPAIR_LABEL
import com.ousiassllc.swarmviewer.ui.main.Badge
import com.ousiassllc.swarmviewer.ui.main.DisplayTone
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppDangerButton
import com.ousiassllc.swarmviewer.ui.theme.AppDialog
import com.ousiassllc.swarmviewer.ui.theme.AppEmptyState
import com.ousiassllc.swarmviewer.ui.theme.AppNavTextButton
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily

/**
 * 接続管理（S1）。保存済み接続の一覧（表示名と URL の host のみ。token は出さない）、切替、削除（確認つき）、
 * 再ペアリングが必要な接続の導線。接続の追加はペアリング画面へ。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConnectionsScreen(
    viewModel: ConnectionsViewModel,
    onAdd: () -> Unit,
    onRepair: () -> Unit,
    onBack: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.refresh() } // ペアリングから戻ったときに読み直す

    Scaffold(
        topBar = { TopAppBar(title = { Text("接続") }, navigationIcon = { AppNavTextButton("戻る", onBack) }) },
    ) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).padding(LocalGutter.current),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            state.message?.let { AppAlert(it) }
            val connections = state.connections
            when {
                connections == null -> Text("読み込み中…")
                connections.isEmpty() -> AppEmptyState("接続がありません。接続文字列を貼り付けるか、QR を読み取って追加してください。")
                else ->
                    LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.weight(1f)) {
                        items(connections, key = { it.id }) { item ->
                            ConnectionCard(
                                item = item,
                                onSelect = { viewModel.select(item.id) },
                                onRepair = onRepair,
                                onDelete = { viewModel.requestDelete(item) },
                            )
                        }
                    }
            }
            AppButton(onClick = onAdd, modifier = Modifier.fillMaxWidth()) { Text("接続を追加") }
        }
    }

    state.pendingDelete?.let { target ->
        AppDialog(
            onDismissRequest = viewModel::dismissDelete,
            title = { Text("接続を削除") },
            text = { Text("「${target.name}」を削除します。保存した token・前回取得した表示・通知済みの記録もすべて消えます。") },
            confirmButton = { AppDangerButton(onClick = viewModel::confirmDelete) { Text("削除") } },
            dismissButton = { AppOutlinedButton(onClick = viewModel::dismissDelete) { Text("キャンセル") } },
        )
    }
}

@Composable
private fun ConnectionCard(
    item: ConnectionItem,
    onSelect: () -> Unit,
    onRepair: () -> Unit,
    onDelete: () -> Unit,
) {
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(item.name, style = MaterialTheme.typography.titleMedium)
            Text(item.host, style = MaterialTheme.typography.bodyMedium, fontFamily = MonoFontFamily)
            if (item.selected) Badge("表示中", DisplayTone.Accent, marker = true)
            if (item.needsRepair) AppAlert(NEEDS_REPAIR_LABEL)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (!item.selected) AppOutlinedButton(onClick = onSelect) { Text("切り替える") }
                if (item.needsRepair) AppButton(onClick = onRepair) { Text("再ペアリング") }
                AppDangerButton(onClick = onDelete) { Text("削除") }
            }
        }
    }
}
