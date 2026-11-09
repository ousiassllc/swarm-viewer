package com.ousiassllc.swarmviewer.ui.pane

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.MAX_SEND_CHARS
import com.ousiassllc.swarmviewer.ui.CLOSE_CONFIRM_ACTION
import com.ousiassllc.swarmviewer.ui.CLOSE_CONFIRM_BODY
import com.ousiassllc.swarmviewer.ui.CLOSE_CONFIRM_TITLE
import com.ousiassllc.swarmviewer.ui.CONFIRM_CANCEL_ACTION
import com.ousiassllc.swarmviewer.ui.ESCAPE_CONFIRM_ACTION
import com.ousiassllc.swarmviewer.ui.ESCAPE_CONFIRM_BODY
import com.ousiassllc.swarmviewer.ui.ESCAPE_CONFIRM_TITLE
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppDangerButton
import com.ousiassllc.swarmviewer.ui.theme.AppDialog
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.AppTextField

/**
 * 指示を送る欄（操作の接続だけ。[PaneControls.showSendField]）。状態が `idle`・`stalled` 以外のときは入力欄も送信も無効にし、
 * 理由（[PaneControls.sendDisabledReason]）を 1 行出す。送信中（[busy]）も無効。
 */
@Composable
fun SendSection(
    controls: PaneControls,
    busy: Boolean,
    text: String,
    onTextChange: (String) -> Unit,
    onSend: () -> Unit,
) {
    if (!controls.showSendField) return
    val enabled = controls.sendEnabled && !busy
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        AppTextField(value = text, onValueChange = onTextChange, label = "指示を送る", enabled = enabled, modifier = Modifier.fillMaxWidth())
        controls.sendDisabledReason?.let { MutedText(it) }
        Row(horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
            MutedText("${text.length} / $MAX_SEND_CHARS")
            AppButton(onClick = onSend, enabled = enabled && text.isNotEmpty(), modifier = Modifier.heightIn(min = TouchTargetHeight)) {
                Text("送信")
            }
        }
    }
}

/**
 * 詳細の最下部の「中断」（目立たないテキストボタン）と「終了」（枠だけの危険ボタン）。どちらも押すと確認ダイアログを出すだけで、
 * 送らない（[PaneDetailViewModel.requestEscape]・[PaneDetailViewModel.requestClose]）。見るだけの接続・対象外の状態では出さない。
 */
@Composable
fun EscapeCloseSection(
    controls: PaneControls,
    busy: Boolean,
    onEscape: () -> Unit,
    onClose: () -> Unit,
) {
    if (!controls.showEscape && !controls.showClose) return
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
        if (controls.showEscape) {
            TextButton(onClick = onEscape, enabled = !busy, modifier = Modifier.heightIn(min = TouchTargetHeight)) {
                Text("中断", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        if (controls.showClose) {
            AppDangerButton(onClick = onClose, enabled = !busy, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text("終了") }
        }
    }
}

/** 中断・終了の確認ダイアログ。確定（[onConfirm]）を経た場合だけ、`send_escape`（`confirm: true`）・`pane_close` が送られる。 */
@Composable
fun PaneConfirmDialog(
    confirm: PaneConfirm,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    val (title, body, action) =
        when (confirm) {
            PaneConfirm.Escape -> Triple(ESCAPE_CONFIRM_TITLE, ESCAPE_CONFIRM_BODY, ESCAPE_CONFIRM_ACTION)
            PaneConfirm.Close -> Triple(CLOSE_CONFIRM_TITLE, CLOSE_CONFIRM_BODY, CLOSE_CONFIRM_ACTION)
        }
    AppDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { Text(body) },
        confirmButton = { AppDangerButton(onClick = onConfirm, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text(action) } },
        dismissButton = {
            AppOutlinedButton(onClick = onDismiss, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text(CONFIRM_CANCEL_ACTION) }
        },
    )
}
