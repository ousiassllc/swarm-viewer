package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.windowsizeclass.WindowWidthSizeClass
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
private val GUTTER_MEDIUM = 24.dp
private val GUTTER_EXPANDED = 32.dp

/**
 * 画面の四辺のガター（NFR-DSG-10）。幅の区分で Compact 16・Medium（600dp〜。メインの 2 列表示の切替と同じ境目）24・Expanded（840dp〜）32。
 * Web は `layout/base.templ` の `p-4 medium:p-6 expanded:p-8` で同じ境目・同じ値にする。
 * システムバー・切り欠きの inset はこの外側（Scaffold・[com.ousiassllc.swarmviewer.MainActivity]）が受け持ち、ガターは inset の内側に足す。
 * [SwarmViewerTheme] の外では 16dp。
 */
val LocalGutter = staticCompositionLocalOf { GUTTER_COMPACT }

/** 幅の区分に応じたガター。 */
fun gutterFor(widthClass: WindowWidthSizeClass) =
    when (widthClass) {
        WindowWidthSizeClass.Compact -> GUTTER_COMPACT
        WindowWidthSizeClass.Medium -> GUTTER_MEDIUM
        else -> GUTTER_EXPANDED
    }

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
