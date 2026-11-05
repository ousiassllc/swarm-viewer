package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
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
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
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

/** 無効のときの不透明度（ガイド O7: disabled は opacity .4。Web の `disabled:opacity-40`）。 */
private const val DISABLED_ALPHA = 0.4f

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
        colors = ButtonDefaults.outlinedButtonColors(disabledContentColor = scheme.primary.copy(alpha = DISABLED_ALPHA)),
        border = BorderStroke(1.dp, if (enabled) scheme.outline else scheme.outline.copy(alpha = DISABLED_ALPHA)),
        content = content,
    )
}

/** 危険なテキストボタン（ガイド O7）: 文字を状態色の danger にする。削除など取り返しのつかない操作の確定・起点に使い、カード上で塗りつぶしを増やさない。 */
@Composable
fun AppDangerTextButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit,
) {
    TextButton(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.extraColors.danger),
        content = content,
    )
}

/** 入力欄（ガイド）: 地は surface（Web の `bg-surface` と同じ）。フォーカス中の枠と caret はフォーカスリング色（[ExtraColors.focus]）。 */
@Composable
fun AppTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    singleLine: Boolean = false,
) {
    val focus = MaterialTheme.extraColors.focus
    val surface = MaterialTheme.colorScheme.surface
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        label = { Text(label) },
        visualTransformation = visualTransformation,
        keyboardOptions = keyboardOptions,
        singleLine = singleLine,
        shape = MaterialTheme.shapes.medium,
        colors =
            OutlinedTextFieldDefaults.colors(
                focusedContainerColor = surface,
                unfocusedContainerColor = surface,
                focusedBorderColor = focus,
                focusedLabelColor = focus,
                cursorColor = focus,
            ),
        modifier = modifier,
    )
}

/** インラインアラートの種類（ガイド: お知らせ / 成功 / 注意 / エラー）。 */
enum class AlertKind { Info, Success, Warning, Danger }

private const val ALERT_BACKGROUND_ALPHA = 0.1f
private const val ALERT_BORDER_ALPHA = 0.6f

@Composable
private fun AlertKind.color(): Color =
    when (this) {
        AlertKind.Info -> MaterialTheme.extraColors.info
        AlertKind.Success -> MaterialTheme.extraColors.success
        AlertKind.Warning -> MaterialTheme.extraColors.warning
        AlertKind.Danger -> MaterialTheme.extraColors.danger
    }

private fun AlertKind.iconRes(): Int =
    when (this) {
        AlertKind.Info -> R.drawable.ic_alert_info
        AlertKind.Success -> R.drawable.ic_alert_success
        AlertKind.Warning -> R.drawable.ic_alert_warning
        AlertKind.Danger -> R.drawable.ic_alert_danger
    }

/**
 * インラインアラート（ガイド C4）: 状態色の枠と薄い地・Iconoir のアイコン＋文言。状態は色だけでなくアイコンと文言でも伝える。
 * [title] は見出し（SemiBold・14sp）で、あれば [message] の前に出す（空なら出さない）。1 文で足りるときは [message] だけでよい。
 * [title]・[message] は信頼できない入力でもテキストとして描画する。続けて操作などを載せるときは [content] に書く。
 * 出現は読み上げに通知する（Web の `role="alert"` / `role="status"` に当たる）。
 */
@Composable
fun AppAlert(
    message: String,
    modifier: Modifier = Modifier,
    kind: AlertKind = AlertKind.Danger,
    title: String = "",
    content: @Composable ColumnScope.() -> Unit = {},
) {
    val color = kind.color()
    Surface(
        modifier = modifier.fillMaxWidth().semantics { liveRegion = LiveRegionMode.Polite },
        color = color.copy(alpha = ALERT_BACKGROUND_ALPHA),
        contentColor = color,
        shape = MaterialTheme.shapes.large,
        border = BorderStroke(1.dp, color.copy(alpha = ALERT_BORDER_ALPHA)),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Icon(painterResource(kind.iconRes()), contentDescription = null, modifier = Modifier.padding(top = 2.dp))
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                if (title.isNotEmpty()) Text(title, style = MaterialTheme.typography.titleSmall)
                if (message.isNotEmpty()) Text(message, style = MaterialTheme.typography.bodyMedium)
                content()
            }
        }
    }
}

private const val DASH_LENGTH_DP = 6f
private val EmptyStatePadding = 24.dp
private val EmptyStateIconSize = 32.dp

/**
 * 空状態（ガイド S4・§4）: 1dp の破線の枠・角丸 12dp（r-lg）・余白 24dp・32dp のアイコン＋文言。次の操作（主ボタンなど）は [content] に書く。
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
                }.padding(EmptyStatePadding),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(
            painterResource(R.drawable.ic_alert_info),
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(EmptyStateIconSize),
        )
        Text(message, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
        content()
    }
}
