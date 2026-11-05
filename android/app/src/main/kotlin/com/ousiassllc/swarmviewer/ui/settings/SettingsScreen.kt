package com.ousiassllc.swarmviewer.ui.settings

import android.content.Context
import android.content.pm.PackageManager
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.R
import com.ousiassllc.swarmviewer.domain.ALLOWED_SCREEN_POLL_SECONDS

/**
 * 設定画面（A3。機能要件 §12.4）。画面更新間隔（5 / 10 / 30 / 60 秒）の選択、表示名・バージョン、接続管理（A1）への導線。
 * 通知の項目は後続の変更が [SettingsSection] として同じ画面に足す。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    viewModel: SettingsViewModel,
    onOpenConnections: () -> Unit,
    onBack: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.refresh() }

    Scaffold(
        topBar = {
            TopAppBar(title = { Text("設定") }, navigationIcon = { TextButton(onClick = onBack) { Text("戻る") } })
        },
    ) { padding ->
        Column(
            modifier =
                Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .verticalScroll(rememberScrollState())
                    .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            state.message?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            state.settings?.let { settings ->
                SettingsSection("画面更新間隔（選択中セッション）") {
                    ALLOWED_SCREEN_POLL_SECONDS.forEach { seconds ->
                        RadioRow("$seconds 秒", settings.screenPollSeconds == seconds) { viewModel.setScreenPollSeconds(seconds) }
                    }
                    Text(
                        "画面が見えている間だけ更新します。バックグラウンドでは止まります。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            SettingsSection("接続") {
                OutlinedButton(onClick = onOpenConnections, modifier = Modifier.fillMaxWidth()) { Text("接続を管理") }
            }
            SettingsSection("このアプリ") {
                Text(stringResource(R.string.app_name))
                Text(
                    "バージョン " + versionName(LocalContext.current),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

/** 設定の 1 区画（見出しと内容）。 */
@Composable
fun SettingsSection(
    title: String,
    content: @Composable () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(title, style = MaterialTheme.typography.titleSmall)
        content()
    }
}

@Composable
private fun RadioRow(
    label: String,
    selected: Boolean,
    onSelect: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().selectable(selected = selected, onClick = onSelect, role = Role.RadioButton),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(selected = selected, onClick = null)
        Text(label, modifier = Modifier.padding(start = 12.dp))
    }
}

/** アプリのバージョン名。読めなければ「不明」。 */
@Composable
private fun versionName(context: Context): String =
    remember(context) {
        try {
            context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: "不明"
        } catch (e: PackageManager.NameNotFoundException) {
            "不明"
        }
    }
