package com.ousiassllc.swarmviewer.ui.pairing

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.service.PairingDraft
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton

/**
 * ペアリング画面。貼り付け欄（伏せ字。token を含むため画面に出さない）と「QR を読み取る」から入り、
 * ディープリンク・QR の結果も同じ確認画面（[PairingDraft]）に入る。確認画面は表示名・接続先の host・許可セッションのヒントだけで、
 * token は出さない（機能要件 §12.2）。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PairingScreen(
    viewModel: PairingViewModel,
    onScan: () -> Unit,
    onBack: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val leave = {
        viewModel.cancel()
        onBack()
    }
    BackHandler(onBack = leave)

    Scaffold(topBar = { TopAppBar(title = { Text("接続を追加") }) }) { padding ->
        Column(
            modifier =
                Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .padding(16.dp)
                    .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            val draft = state.draft
            if (draft == null) {
                PasteSection(
                    input = state.input,
                    busy = state.busy,
                    onInputChange = viewModel::onInputChange,
                    onSubmit = viewModel::submitInput,
                    onScan = onScan,
                )
            } else {
                ConfirmSection(draft = draft, busy = state.busy, onConfirm = viewModel::confirm, onCancel = viewModel::cancel)
            }
            state.message?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            TextButton(onClick = leave) { Text("戻る") }
        }
    }
}

@Composable
private fun PasteSection(
    input: String,
    busy: Boolean,
    onInputChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onScan: () -> Unit,
) {
    Text("swarm-console の接続文字列（swarm-viewer://pair?…）を貼り付けるか、QR を読み取ってください。")
    OutlinedTextField(
        value = input,
        onValueChange = onInputChange,
        label = { Text("接続文字列") },
        // token を含むため伏せ字にし、キーボードの学習・候補に残さない。
        visualTransformation = PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, autoCorrectEnabled = false),
        singleLine = true,
        modifier = Modifier.fillMaxWidth(),
    )
    AppButton(onClick = onSubmit, enabled = input.isNotBlank() && !busy, modifier = Modifier.fillMaxWidth()) { Text("接続") }
    AppOutlinedButton(onClick = onScan, enabled = !busy, modifier = Modifier.fillMaxWidth()) { Text("QR を読み取る") }
}

@Composable
private fun ConfirmSection(
    draft: PairingDraft,
    busy: Boolean,
    onConfirm: () -> Unit,
    onCancel: () -> Unit,
) {
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("この接続先に接続します", style = MaterialTheme.typography.titleMedium)
            Text("表示名: ${draft.name}")
            Text("接続先: ${draft.host}")
            if (draft.sessionsHint.isNotEmpty()) Text("ホストが申告した許可セッション: ${draft.sessionsHint.joinToString(", ")}")
            if (draft.replacesExisting) Text("同じ接続先が保存済みです。token を差し替えます。")
        }
    }
    AppButton(onClick = onConfirm, enabled = !busy, modifier = Modifier.fillMaxWidth()) { Text(if (busy) "確認中…" else "接続") }
    AppOutlinedButton(onClick = onCancel, enabled = !busy, modifier = Modifier.fillMaxWidth()) { Text("キャンセル") }
}
