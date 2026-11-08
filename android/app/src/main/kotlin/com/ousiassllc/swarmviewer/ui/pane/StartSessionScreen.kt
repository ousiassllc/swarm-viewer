package com.ousiassllc.swarmviewer.ui.pane

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.domain.MAX_TITLE_CHARS
import com.ousiassllc.swarmviewer.domain.START_AGENTS
import com.ousiassllc.swarmviewer.domain.normalizeTitle
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.ui.CONFIRM_CANCEL_ACTION
import com.ousiassllc.swarmviewer.ui.NEEDS_REPAIR_LABEL
import com.ousiassllc.swarmviewer.ui.VIEW_ONLY_MESSAGE
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.settings.RadioRow
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppDialog
import com.ousiassllc.swarmviewer.ui.theme.AppNavTextButton
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.AppTextField
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter

/**
 * 新しいセッション（機能要件 §4.7）。エージェント（omp / claude / codex）→ フォルダ（[FolderPicker]）→ タイトル → 確認 → `pane_start`。
 * 見るだけの接続では何も出さず固定文言だけ（導線も出さない）。起動できたら [onStarted]（そのペインの詳細へ）。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StartSessionScreen(
    viewModel: StartSessionViewModel,
    target: PaneTarget,
    onStarted: () -> Unit,
    onBack: () -> Unit,
    onRepair: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(target) { viewModel.open(target) }
    LaunchedEffect(Unit) { viewModel.started.collect { onStarted() } }

    Scaffold(
        topBar = { TopAppBar(title = { Text("新しいセッション") }, navigationIcon = { AppNavTextButton("戻る", onBack) }) },
    ) { padding ->
        Box(modifier = Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.TopCenter) {
            Column(
                modifier =
                    Modifier
                        .widthIn(max = ContentMaxWidth)
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(LocalGutter.current),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                if (state.needsRepair) {
                    AppAlert(NEEDS_REPAIR_LABEL) { AppButton(onClick = onRepair, modifier = Modifier.bigButton()) { Text("再ペアリング") } }
                }
                state.message?.let { AppAlert(it) }
                if (state.viewOnly) {
                    AppAlert(VIEW_ONLY_MESSAGE)
                } else {
                    StartForm(state, viewModel)
                }
            }
        }
    }

    if (state.confirming) StartConfirmDialog(state, onConfirm = viewModel::start, onDismiss = viewModel::dismissConfirm)
}

@Composable
private fun StartForm(
    state: StartSessionUiState,
    viewModel: StartSessionViewModel,
) {
    val enabled = !state.busy
    Text("エージェント", style = MaterialTheme.typography.titleSmall)
    Column {
        START_AGENTS.forEach { agent ->
            RadioRow(label = agent, selected = state.agent == agent, onSelect = { if (enabled) viewModel.selectAgent(agent) })
        }
    }
    Text("フォルダ", style = MaterialTheme.typography.titleSmall)
    FolderPicker(state, viewModel)
    Text("タイトル", style = MaterialTheme.typography.titleSmall)
    AppTextField(
        value = state.title,
        onValueChange = viewModel::onTitleChange,
        label = "タイトル（1〜$MAX_TITLE_CHARS 字）",
        singleLine = true,
        enabled = enabled,
        isError = state.title.isNotEmpty() && normalizeTitle(state.title) == null,
        modifier = Modifier.fillMaxWidth(),
    )
    if (state.busy) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            CircularProgressIndicator(modifier = Modifier.padding(4.dp))
            MutedText("起動しています…")
        }
    }
    AppButton(onClick = viewModel::requestConfirm, enabled = state.canConfirm && enabled, modifier = Modifier.bigButton()) { Text("確認へ") }
}

/** 確認: エージェント・フォルダ・タイトルの要約。確定で `pane_start`。 */
@Composable
private fun StartConfirmDialog(
    state: StartSessionUiState,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AppDialog(
        onDismissRequest = onDismiss,
        title = { Text("この内容で始めます") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("エージェント: ${state.agent.orEmpty()}")
                Text("フォルダ: ${state.folder.orEmpty()}")
                Text("タイトル: ${normalizeTitle(state.title).orEmpty()}")
            }
        },
        confirmButton = { AppButton(onClick = onConfirm, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text("始める") } },
        dismissButton = {
            AppOutlinedButton(onClick = onDismiss, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text(CONFIRM_CANCEL_ACTION) }
        },
    )
}
