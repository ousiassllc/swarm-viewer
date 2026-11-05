package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.ProvideTextStyle
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshState
import androidx.compose.material3.pulltorefresh.pullToRefreshIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
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

/** 確認ダイアログの幅（Material の AlertDialog の下限・上限と同じ）。 */
private val DialogMinWidth = 280.dp
private val DialogMaxWidth = 560.dp

/**
 * 確認ダイアログ（ガイド O2）: surface の面・1dp の border・角丸 16dp（r-xl）・影は 1dp（`--shadow-modal`。影はモーダルだけ）。
 * Material の `AlertDialog` は枠も影も持たないため、`BasicAlertDialog` に面を組んで置き換える。
 * ボタンは `dismissButton`・`confirmButton` の順に右寄せで並べる。
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
            Column(modifier = Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                ProvideTextStyle(MaterialTheme.typography.titleLarge) { title() }
                ProvideTextStyle(MaterialTheme.typography.bodyMedium) { text() }
                Row(
                    modifier = Modifier.fillMaxWidth(),
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

/**
 * スイッチ（ガイド `.sw2`）: オフはトラック accent-soft（surfaceVariant）・border の枠・つまみ surface、オンはトラック accent（primary）・つまみ bone（background）。
 * Material の既定はオフのつまみもトラックも border / accent-soft と同じ値になり、つまみがトラックに溶けるので色を明示する。
 * 操作は呼び出し側の行（toggleable）が受けるので、`onCheckedChange` は null のまま使う。
 */
@Composable
fun AppSwitch(
    checked: Boolean,
    modifier: Modifier = Modifier,
) {
    val scheme = MaterialTheme.colorScheme
    Switch(
        checked = checked,
        onCheckedChange = null,
        modifier = modifier,
        colors =
            SwitchDefaults.colors(
                checkedThumbColor = scheme.background,
                checkedTrackColor = scheme.primary,
                checkedBorderColor = scheme.primary,
                uncheckedThumbColor = scheme.surface,
                uncheckedTrackColor = scheme.surfaceVariant,
                uncheckedBorderColor = scheme.outline,
            ),
    )
}

/** 引っ張って更新のインジケーターの外径（M3 の `SpinnerContainerSize` と同じ）と、中の円形進捗の径。 */
private val RefreshIndicatorSize = 40.dp
private val RefreshSpinnerSize = 24.dp

/**
 * 引っ張って更新のインジケーター（ガイド S3・§6: 影はモーダルだけ）: surface の円・1dp の border・影なし・中は primary の円形進捗。
 * M3 の既定（`PullToRefreshDefaults.Indicator`）は引っ張り始めから更新中まで 2 段の影を付けるので、影を 0 にした自前の面に置き換える。
 * 引っ張っている間は進捗が引っ張り量に追従し、更新中は不定の回転になる。`PullToRefreshBox` の `indicator` に渡す。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BoxScope.AppPullToRefreshIndicator(
    state: PullToRefreshState,
    isRefreshing: Boolean,
) {
    val scheme = MaterialTheme.colorScheme
    Box(
        modifier =
            Modifier
                .align(Alignment.TopCenter)
                .size(RefreshIndicatorSize)
                .pullToRefreshIndicator(
                    state = state,
                    isRefreshing = isRefreshing,
                    shape = CircleShape,
                    containerColor = scheme.surface,
                    elevation = 0.dp,
                ).border(1.dp, scheme.outline, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        val spinnerModifier = Modifier.size(RefreshSpinnerSize)
        if (isRefreshing) {
            CircularProgressIndicator(
                modifier = spinnerModifier,
                color = scheme.primary,
                strokeWidth = 2.dp,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Round,
            )
        } else {
            CircularProgressIndicator(
                progress = { state.distanceFraction.coerceIn(0f, 1f) },
                modifier = spinnerModifier,
                color = scheme.primary,
                strokeWidth = 2.dp,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Round,
            )
        }
    }
}
