package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.unit.dp

private const val BADGE_BACKGROUND_ALPHA = 0.15f
private const val BADGE_BORDER_ALPHA = 0.4f
private const val PILL_PERCENT = 50
private const val STALE_ALPHA = 0.5f
private val BAR_HEIGHT = 6.dp
private val MARKER_SIZE = 6.dp
private val MARKER_GAP = 6.dp

/** 取得に失敗して直前の内容を残しているとき、見た目を薄くする（グレーアウト）。 */
fun Modifier.grayedOut(stale: Boolean): Modifier = if (stale) alpha(STALE_ALPHA) else this

/**
 * 状態・agent・要対応の種別を示す小さなバッジ。色だけに依存せず、必ずラベルを持つ。[text] は常にテキストとして描画する。
 * 状態を示すバッジは [marker] を true にし、文言の前に文字と同じ色の小さな点（ガイドのステータスピルの見本 `● 完了`）を置く。
 * agent 名のように状態でないバッジは点なし。
 * muted の文字は情報（agent 名・shell / unknown の状態）なので、`onSurfaceVariant`（mute）でなく `onSurface`（ink）で描く（ガイド C6）。
 * 地は mute のまま。
 */
@Composable
fun Badge(
    text: String,
    tone: DisplayTone,
    modifier: Modifier = Modifier,
    marker: Boolean = false,
) {
    val color = tone.color()
    val textColor = if (tone == DisplayTone.Muted) MaterialTheme.colorScheme.onSurface else color
    Surface(
        color = color.copy(alpha = BADGE_BACKGROUND_ALPHA),
        shape = RoundedCornerShape(percent = PILL_PERCENT),
        border = BorderStroke(1.dp, color.copy(alpha = BADGE_BORDER_ALPHA)),
        modifier = modifier,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(MARKER_GAP),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (marker) Box(modifier = Modifier.size(MARKER_SIZE).background(textColor, CircleShape))
            Text(text, color = textColor, style = MaterialTheme.typography.labelMedium)
        }
    }
}

/**
 * 使用率のバー。[percent] は 0〜100 に丸める。[tone] は domain のしきい値判定の結果。
 * M3 の既定（塗りとトラックの 4dp の隙間・終端の点）はガイドの「連続したトラックの上に塗りが載る」形と違うので、
 * `gapSize = 0.dp` と `drawStopIndicator = {}` で消す（O8）。
 */
@Composable
fun ToneBar(
    percent: Double,
    tone: DisplayTone,
    modifier: Modifier = Modifier,
) {
    LinearProgressIndicator(
        progress = { barFraction(percent) },
        color = tone.color(),
        trackColor = MaterialTheme.colorScheme.surfaceVariant,
        strokeCap = StrokeCap.Round,
        gapSize = 0.dp,
        drawStopIndicator = {},
        modifier = modifier.height(BAR_HEIGHT),
    )
}

/** キャプション・ラベル用（muted、14sp）。ライトテーマで AA に届かないので、文には使わず NoticeText を使う。 */
@Composable
fun MutedText(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = MaterialTheme.colorScheme.onSurfaceVariant,
) {
    Text(text, style = MaterialTheme.typography.bodyMedium, color = color, modifier = modifier)
}

/** 文章の案内（ink）。muted は light テーマで AA に届かないので、キャプション・ラベル以外の文は必ずこちらを使う。 */
@Composable
fun NoticeText(
    text: String,
    modifier: Modifier = Modifier,
) {
    Text(text, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurface, modifier = modifier)
}
