package com.ousiassllc.swarmviewer.ui.pane

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.AskOption
import com.ousiassllc.swarmviewer.domain.MAX_SEND_CHARS
import com.ousiassllc.swarmviewer.ui.ASK_VIEW_ONLY_HINT
import com.ousiassllc.swarmviewer.ui.main.Badge
import com.ousiassllc.swarmviewer.ui.main.DisplayTone
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.AppTextField

/** ボタンの最小の高さ（親指で押せる大きさ。機能要件 §12.2）。 */
val TouchTargetHeight: Dp = 48.dp

/** 縦に並べる大きなボタンの修飾（幅いっぱい・最小 48dp）。 */
fun Modifier.bigButton(): Modifier = fillMaxWidth().heightIn(min = TouchTargetHeight)

/**
 * 質問カード（`ask_card`、`asking: true`）。質問文と選択肢はテキストとして描画する（信頼できない入力）。
 *  - 操作の接続（[PaneControls.showCardButtons]）: 選択肢を**大きなボタン**で縦に並べ、押したらすぐ [onPick]（確認なし）。
 *    `recommended` に「おすすめ」印。自由入力（[PaneControls.showCustomEntry]）は「その他（自分で書く）」→ 入力欄 → 送信
 *  - 見るだけの接続（[PaneControls.showCardList]）: 選択肢の**一覧**（ボタンにしない）と「この質問には、操作できる接続から答えられます」の 1 行
 * [busy] の間は選択肢・その他・送信のボタンをすべて無効にする。
 */
@Composable
fun AskCardSection(
    card: AskCard,
    controls: PaneControls,
    busy: Boolean,
    customOpen: Boolean,
    customText: String,
    onPick: (Int) -> Unit,
    onOpenCustom: (Boolean) -> Unit,
    onCustomTextChange: (String) -> Unit,
    onSendCustom: () -> Unit,
) {
    if (!controls.showCardList) return
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(card.question, style = MaterialTheme.typography.titleSmall)
            if (controls.showCardButtons) {
                card.options.forEach { OptionButton(it, enabled = !busy, onClick = { onPick(it.n) }) }
                if (controls.showCustomEntry) {
                    CustomEntry(busy, customOpen, customText, onOpenCustom, onCustomTextChange, onSendCustom)
                }
            } else {
                card.options.forEach { OptionRow(it) }
            }
            if (controls.showViewOnlyHint) MutedText(ASK_VIEW_ONLY_HINT)
        }
    }
}

@Composable
private fun OptionButton(
    option: AskOption,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    AppOutlinedButton(onClick = onClick, enabled = enabled, modifier = Modifier.bigButton()) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("${option.n}. ${option.label}")
            if (option.recommended) Badge(RECOMMENDED_LABEL, DisplayTone.Accent)
        }
    }
}

// 見るだけの一覧。ボタンにしない（押せるように見せない）。
@Composable
private fun OptionRow(option: AskOption) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("${option.n}. ${option.label}", style = MaterialTheme.typography.bodyMedium)
        if (option.recommended) Badge(RECOMMENDED_LABEL, DisplayTone.Accent)
    }
}

@Composable
private fun CustomEntry(
    busy: Boolean,
    open: Boolean,
    text: String,
    onOpen: (Boolean) -> Unit,
    onTextChange: (String) -> Unit,
    onSend: () -> Unit,
) {
    if (!open) {
        AppOutlinedButton(onClick = { onOpen(true) }, enabled = !busy, modifier = Modifier.bigButton()) { Text(CUSTOM_LABEL) }
        return
    }
    AppTextField(
        value = text,
        onValueChange = onTextChange,
        label = CUSTOM_LABEL,
        enabled = !busy,
        modifier = Modifier.fillMaxWidth(),
    )
    MutedText("${text.length} / $MAX_SEND_CHARS")
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        AppButton(onClick = onSend, enabled = !busy && text.isNotEmpty(), modifier = Modifier.heightIn(min = TouchTargetHeight)) {
            Text("送信")
        }
        AppOutlinedButton(onClick = { onOpen(false) }, enabled = !busy, modifier = Modifier.heightIn(min = TouchTargetHeight)) {
            Text("閉じる")
        }
    }
}

private const val RECOMMENDED_LABEL = "おすすめ"
private const val CUSTOM_LABEL = "その他（自分で書く）"
