package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.BasicAlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.ProvideTextStyle
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.R

/** カード（ガイド）: surface の面・1dp の border・角丸 12dp（r-lg）・影なし。内側の余白は呼び出し側が 16dp（小）か 24dp で付ける。 */
@Composable
fun AppCard(
    modifier: Modifier = Modifier,
    borderColor: Color = MaterialTheme.colorScheme.outline,
    borderWidth: Dp = 1.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    Card(
        modifier = modifier,
        shape = MaterialTheme.shapes.large,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp),
        border = BorderStroke(borderWidth, borderColor),
        content = content,
    )
}

/** 確認ダイアログの幅。下限は Material の AlertDialog と同じ、上限はガイドの見本（`.modal` の max-width 420px）。 */
private val DialogMinWidth = 280.dp
private val DialogMaxWidth = 420.dp

/**
 * 確認ダイアログ（ガイド O2）: surface の面・1dp の border・角丸 16dp（r-xl）・影は 1dp（`--shadow-modal`。影はモーダルだけ）。
 * Material の `AlertDialog` は枠も影も持たないため、`BasicAlertDialog` に面を組んで置き換える。
 * 構造もガイドの見本（`.modal`）に合わせる: ヘッダー（[title] と閉じる × のアイコンボタン。外側タップ・戻ると同じ [onDismissRequest]）・
 * 本文（[text]）・フッター（右寄せ・ボタン間 8dp）を 1dp の border の線で区切る。ボタンは `dismissButton`・`confirmButton` の順に並べる。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AppDialog(
    onDismissRequest: () -> Unit,
    title: @Composable () -> Unit,
    confirmButton: @Composable () -> Unit,
    dismissButton: @Composable () -> Unit,
    text: @Composable () -> Unit,
) {
    BasicAlertDialog(onDismissRequest = onDismissRequest) {
        Surface(
            modifier = Modifier.widthIn(min = DialogMinWidth, max = DialogMaxWidth),
            shape = MaterialTheme.shapes.extraLarge,
            color = MaterialTheme.colorScheme.surface,
            contentColor = MaterialTheme.colorScheme.onSurface,
            tonalElevation = 0.dp,
            shadowElevation = 1.dp,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
        ) {
            Column {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(start = 20.dp, end = 12.dp, top = 8.dp, bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(modifier = Modifier.weight(1f)) { ProvideTextStyle(MaterialTheme.typography.titleLarge) { title() } }
                    IconButton(onClick = onDismissRequest, modifier = Modifier.size(32.dp)) {
                        Icon(painterResource(R.drawable.ic_xmark), contentDescription = "閉じる")
                    }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outline)
                Column(modifier = Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    ProvideTextStyle(MaterialTheme.typography.bodyMedium) { text() }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outline)
                Row(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    dismissButton()
                    confirmButton()
                }
            }
        }
    }
}

/** 無効のときの不透明度（ガイド O7: disabled は opacity .4。Web の `disabled:opacity-40`）。 */
private const val DISABLED_ALPHA = 0.4f

/** 危険ボタンの枠の不透明度（`.dan` の `color-mix(in srgb, var(--danger) 45%, transparent)`）。 */
private const val DANGER_BORDER_ALPHA = 0.45f

/** ボタンの左右 padding 18dp（ガイド S2。tokens-preview の `.btn{padding:0 18px}`・Web の `px-[18px]`）。上下は高さ 40dp が決めるので 0。 */
private val ButtonContentPadding = PaddingValues(horizontal = 18.dp)

/** 主ボタン（ガイド）: accent（ink 自身）の面・角丸 8dp（r-md）。文字は SemiBold（labelLarge）。無効のときは面も文字も不透明度 .4（O7）。 */
@Composable
fun AppButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit,
) {
    val scheme = MaterialTheme.colorScheme
    Button(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        shape = MaterialTheme.shapes.medium,
        contentPadding = ButtonContentPadding,
        colors =
            ButtonDefaults.buttonColors(
                disabledContainerColor = scheme.primary.copy(alpha = DISABLED_ALPHA),
                disabledContentColor = scheme.onPrimary.copy(alpha = DISABLED_ALPHA),
            ),
        content = content,
    )
}

/** 副ボタン（ガイド）: 枠だけ・角丸 8dp（r-md）。無効のときは文字も枠も不透明度 .4（O7）。 */
@Composable
fun AppOutlinedButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit,
) {
    val scheme = MaterialTheme.colorScheme
    OutlinedButton(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        shape = MaterialTheme.shapes.medium,
        contentPadding = ButtonContentPadding,
        colors = ButtonDefaults.outlinedButtonColors(disabledContentColor = scheme.primary.copy(alpha = DISABLED_ALPHA)),
        border = BorderStroke(1.dp, if (enabled) scheme.outline else scheme.outline.copy(alpha = DISABLED_ALPHA)),
        content = content,
    )
}

/**
 * 危険ボタン（ガイド O7。tokens-preview の `.dan`）: 透明地・文字は状態色の danger・枠は danger の 45%・角丸 8dp（r-md）。
 * 削除など取り返しのつかない操作の確定・起点に使い、カード上で塗りつぶしを増やさない。無効のときは文字も枠も不透明度 .4（O7）。
 */
@Composable
fun AppDangerButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit,
) {
    val danger = MaterialTheme.extraColors.danger
    OutlinedButton(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        shape = MaterialTheme.shapes.medium,
        contentPadding = ButtonContentPadding,
        colors =
            ButtonDefaults.outlinedButtonColors(
                contentColor = danger,
                disabledContentColor = danger.copy(alpha = DISABLED_ALPHA),
            ),
        border = BorderStroke(1.dp, danger.copy(alpha = if (enabled) DANGER_BORDER_ALPHA else DANGER_BORDER_ALPHA * DISABLED_ALPHA)),
        content = content,
    )
}

/**
 * 画面の移動先を示すだけのテキスト操作（接続管理・設定・戻る。ガイド T2）: ナビ・リンクは Regular。
 * SemiBold はボタンの文字だけなので、ダイアログの確定・キャンセルなど操作の [TextButton] は既定のまま使う。
 */
@Composable
fun AppNavTextButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    TextButton(onClick = onClick, modifier = modifier) { Text(label, style = MaterialTheme.typography.bodyMedium) }
}

/**
 * 入力欄（ガイド）: 地は surface（Web の `bg-surface` と同じ）。フォーカス中の枠と caret はフォーカスリング色（[ExtraColors.focus]）。
 * [isError] のときは枠・caret・ラベルを `--danger`（[ExtraColors.danger]。Web の `FieldInvalidClass`。ガイド O7 の `.fld.bad`）にする。文言は呼び出し側が入力欄の下に出す。
 */
@Composable
fun AppTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    singleLine: Boolean = false,
    isError: Boolean = false,
) {
    val extra = MaterialTheme.extraColors
    val focus = extra.focus
    val danger = extra.danger
    val surface = MaterialTheme.colorScheme.surface
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        label = { Text(label) },
        visualTransformation = visualTransformation,
        keyboardOptions = keyboardOptions,
        singleLine = singleLine,
        isError = isError,
        shape = MaterialTheme.shapes.medium,
        colors =
            OutlinedTextFieldDefaults.colors(
                focusedContainerColor = surface,
                unfocusedContainerColor = surface,
                focusedBorderColor = focus,
                focusedLabelColor = focus,
                cursorColor = focus,
                errorBorderColor = danger,
                errorCursorColor = danger,
                errorLabelColor = danger,
            ),
        modifier = modifier,
    )
}

private const val DASH_LENGTH_DP = 6f
private val EmptyStateHorizontalPadding = 24.dp
private val EmptyStateVerticalPadding = 48.dp
private val EmptyStateCircleSize = 48.dp
private val EmptyStateIconSize = 24.dp

/**
 * 空状態（ガイド S4・§4）: 1dp の破線の枠・角丸 12dp（r-lg）・余白は縦 48dp・横 24dp・48dp の accent-soft の円に載せた 24dp のアイコン＋文言。次の操作（主ボタンなど）は [content] に書く。
 * ローディングは空状態ではないので、これではなく進捗表示を使う。[message] は信頼できない入力でもテキストとして描画する。
 */
@Composable
fun AppEmptyState(
    message: String,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit = {},
) {
    val borderColor = MaterialTheme.colorScheme.outline
    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .drawBehind {
                    val width = 1.dp.toPx()
                    val dash = DASH_LENGTH_DP.dp.toPx()
                    val radius = 12.dp.toPx()
                    drawRoundRect(
                        color = borderColor,
                        topLeft = Offset(width / 2, width / 2),
                        size = Size(size.width - width, size.height - width),
                        cornerRadius = CornerRadius(radius),
                        style = Stroke(width = width, pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                    )
                }.padding(horizontal = EmptyStateHorizontalPadding, vertical = EmptyStateVerticalPadding),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(
            modifier = Modifier.size(EmptyStateCircleSize).background(MaterialTheme.colorScheme.surfaceVariant, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(R.drawable.ic_alert_info),
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(EmptyStateIconSize),
            )
        }
        Text(message, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
        content()
    }
}
