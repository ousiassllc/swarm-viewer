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
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.service.PairingDraft
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppDialog
import com.ousiassllc.swarmviewer.ui.theme.AppNavTextButton
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.AppTextField
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily

/**
 * ペアリング画面。貼り付け欄（伏せ字。token を含むため画面に出さない）と「QR を読み取る」から入り、
 * ディープリンク・QR の結果も同じ確認モーダル（[PairingDraft]）に入る。確認モーダルは表示名・接続先の host・許可セッションのヒントだけで、
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
                    .padding(LocalGutter.current)
                    .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // 確定の失敗は、確認モーダルの中に出す（背面のアラートはスクラムに隠れる）。
            val inputError = state.message.takeIf { state.draft == null }
            PasteSection(
                input = state.input,
                busy = state.busy,
                errorMessage = inputError,
                onInputChange = viewModel::onInputChange,
                onSubmit = viewModel::submitInput,
                onScan = onScan,
            )
            AppNavTextButton("戻る", leave)
        }
    }

    // 確認は全画面の差し替えではなく、貼り付け画面の上に開くモーダル（ガイド O1・O2）。外側タップ・戻るで閉じる＝キャンセル。
    state.draft?.let { draft ->
        ConfirmDialog(
            draft = draft,
            busy = state.busy,
            message = state.message,
            onConfirm = viewModel::confirm,
            onCancel = viewModel::cancel,
        )
    }
}

@Composable
private fun PasteSection(
    input: String,
    busy: Boolean,
    errorMessage: String?,
    onInputChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onScan: () -> Unit,
) {
    Text("swarm-console の接続文字列（swarm-viewer://pair?…）を貼り付けるか、QR を読み取ってください。")
    // 検証エラーの文言は入力欄の直下（間隔 4 = `--gap-help`。ガイド §4 フォーム・O7 の `.fld.bad`）に置き、ボタンはその下に出す。
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        AppTextField(
            value = input,
            onValueChange = onInputChange,
            label = "接続文字列",
            // token を含むため伏せ字にし、キーボードの学習・候補に残さない。
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, autoCorrectEnabled = false),
            singleLine = true,
            isError = errorMessage != null,
            modifier = Modifier.fillMaxWidth(),
        )
        errorMessage?.let { AppAlert(it) }
    }
    AppButton(onClick = onSubmit, enabled = input.isNotBlank() && !busy, modifier = Modifier.fillMaxWidth()) { Text("接続") }
    AppOutlinedButton(onClick = onScan, enabled = !busy, modifier = Modifier.fillMaxWidth()) { Text("QR を読み取る") }
}

/** 追加の確認モーダル。表示名・接続先の host・許可セッションのヒントだけで、token は出さない。保存するまで何も保存しない（NFR-SEC-4）。 */
@Composable
private fun ConfirmDialog(
    draft: PairingDraft,
    busy: Boolean,
    message: String?,
    onConfirm: () -> Unit,
    onCancel: () -> Unit,
) {
    AppDialog(
        onDismissRequest = { if (!busy) onCancel() },
        title = { Text("この接続先に接続します") },
        text = {
            // 文の行の間は 8（`--list-gap`。Web の `<dl>` の `gap-y-2`）、枠付きのアラートは文の外側で 16 空ける（Web の `ModalBodyClass` の `gap-4`）。
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("表示名: ${draft.name}")
                    // host は桁と区切りを読む値なので JetBrains Mono にする（T5。Web の確認モーダルと同じ）。
                    Text(
                        buildAnnotatedString {
                            append("接続先: ")
                            withStyle(SpanStyle(fontFamily = MonoFontFamily)) { append(draft.host) }
                        },
                    )
                    if (draft.sessionsHint.isNotEmpty()) Text("ホストが申告した許可セッション: ${draft.sessionsHint.joinToString(", ")}")
                    if (draft.replacesExisting) Text("同じ接続先が保存済みです。token を差し替えます。")
                }
                message?.let { AppAlert(it) }
            }
        },
        confirmButton = { AppButton(onClick = onConfirm, enabled = !busy) { Text(if (busy) "確認中…" else "接続") } },
        dismissButton = { AppOutlinedButton(onClick = onCancel, enabled = !busy) { Text("キャンセル") } },
    )
}
