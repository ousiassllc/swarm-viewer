package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.unit.dp

private val LocalExtraColors = staticCompositionLocalOf { LightExtraColors }

/** ガイドの状態色（success / warning / danger / info）とフォーカス色。現在のテーマ（ライト / ダーク）の値を返す。 */
val MaterialTheme.extraColors: ExtraColors
    @Composable
    @ReadOnlyComposable
    get() = LocalExtraColors.current

private val GUTTER_COMPACT = 16.dp
private val GUTTER_WIDE = 24.dp

/**
 * 画面の左右・上下のガター（ガイド S5: モバイル `--s-4` 16・`md` 以上 `--s-6` 24）。
 * Android の `md` 相当は幅の区分が Compact でない幅（Medium 以上 = 600dp 以上）で、メインの 2 列表示の切替と同じ境目にする。
 * [SwarmViewerTheme] の外では 16dp。
 */
val LocalGutter = staticCompositionLocalOf { GUTTER_COMPACT }

/** 幅の区分（[wide] = Compact でない）に応じたガター。 */
fun gutterFor(wide: Boolean) = if (wide) GUTTER_WIDE else GUTTER_COMPACT

/** ガイドの角丸: r-sm 4（タグ・チェック）/ r-md 8（入力・ボタン）/ r-lg 12（カード）/ r-xl 16（モーダル）。 */
private val GuideShapes =
    Shapes(
        extraSmall = RoundedCornerShape(8.dp),
        small = RoundedCornerShape(4.dp),
        medium = RoundedCornerShape(8.dp),
        large = RoundedCornerShape(12.dp),
        extraLarge = RoundedCornerShape(16.dp),
    )

@Composable
fun SwarmViewerTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    CompositionLocalProvider(LocalExtraColors provides if (darkTheme) DarkExtraColors else LightExtraColors) {
        MaterialTheme(
            colorScheme = if (darkTheme) DarkColors else LightColors,
            typography = GuideTypography,
            shapes = GuideShapes,
            content = content,
        )
    }
}
