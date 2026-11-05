package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.layout.padding
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

private const val BADGE_BACKGROUND_ALPHA = 0.15f
private const val STALE_ALPHA = 0.5f

/** 取得に失敗して直前の内容を残しているとき、見た目を薄くする（グレーアウト）。 */
fun Modifier.grayedOut(stale: Boolean): Modifier = if (stale) alpha(STALE_ALPHA) else this

/** 状態・agent・要対応の種別を示す小さなバッジ。色だけに依存せず、必ずラベルを持つ。[text] は常にテキストとして描画する。 */
@Composable
fun Badge(
    text: String,
    tone: DisplayTone,
    modifier: Modifier = Modifier,
) {
    val color = tone.color()
    Surface(color = color.copy(alpha = BADGE_BACKGROUND_ALPHA), shape = MaterialTheme.shapes.small, modifier = modifier) {
        Text(
            text,
            color = color,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
        )
    }
}

/** 使用率のバー。[percent] は 0〜100 に丸める。[tone] は domain のしきい値判定の結果。 */
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
        modifier = modifier,
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
