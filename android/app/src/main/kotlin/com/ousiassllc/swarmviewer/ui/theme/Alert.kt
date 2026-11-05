package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.R

/** インラインアラートの種類（ガイド: お知らせ / 成功 / 注意 / エラー）。 */
enum class AlertKind { Info, Success, Warning, Danger }

private val AlertAccentWidth = 3.dp

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
 * インラインアラート（ガイド C4・O8）: surface の地・1dp の border・左 3dp の状態色の帯・角丸 8dp（r-md）。文字は ink で、状態色はアイコンと帯だけに使う。
 * 状態は色だけでなく Iconoir のアイコンと文言でも伝える。
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
        color = MaterialTheme.colorScheme.surface,
        contentColor = MaterialTheme.colorScheme.onSurface,
        shape = MaterialTheme.shapes.medium,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Row(
            modifier =
                Modifier
                    .drawBehind { drawRect(color, size = Size(AlertAccentWidth.toPx(), size.height)) }
                    .padding(horizontal = 16.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Icon(painterResource(kind.iconRes()), contentDescription = null, tint = color, modifier = Modifier.padding(top = 2.dp))
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                if (title.isNotEmpty()) Text(title, style = MaterialTheme.typography.titleSmall)
                if (message.isNotEmpty()) Text(message, style = MaterialTheme.typography.bodyMedium)
                content()
            }
        }
    }
}
