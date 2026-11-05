package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.material3.ColorScheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Immutable
import androidx.compose.ui.graphics.Color

/**
 * Ousiass 共通デザインガイドのトークン（ライト / ダーク）。Material の既定色は使わず、ここの値から [ColorScheme] を組む。
 * accent は ink 自身（主ボタン・リンク・選択中）で、色味を持たせない。状態の区別は色だけに頼らず、必ずラベルを併記する。
 */
private val InkLight = Color(0xFF050505)
private val InkDark = Color(0xFFFFFFFF)
private val BoneLight = Color(0xFFF5F5F4)
private val BoneDark = Color(0xFF050505)
private val MuteLight = Color(0xFF6B6B6E)
private val MuteDark = Color(0xFF8B8B8E)
private val SurfaceLight = Color(0xFFFFFFFF)
private val SurfaceDark = Color(0xFF141414)
private val BorderLight = Color(0xFFE7E7E5)
private val BorderDark = Color(0xFF2A2A2A)
private val AccentLight = Color(0xFF050505)
private val AccentDark = Color(0xFFF5F5F4)
private val AccentSoftLight = Color(0xFFE7E7E5)
private val AccentSoftDark = Color(0xFF2A2A2A)

/** ガイドの状態色（success / warning / danger / info）とフォーカスリング色。テーマ（ライト / ダーク）に合わせて切り替わる。 */
@Immutable
data class ExtraColors(
    val success: Color,
    val warning: Color,
    val danger: Color,
    val info: Color,
    val focus: Color,
)

internal val LightExtraColors =
    ExtraColors(
        success = Color(0xFF3D5C4A),
        warning = Color(0xFF7A6A3A),
        danger = Color(0xFF7A4545),
        info = Color(0xFF4A5A6A),
        focus = Color(0xFF4A72D0),
    )

internal val DarkExtraColors =
    ExtraColors(
        success = Color(0xFF8FAFA0),
        warning = Color(0xFFC4B87A),
        danger = Color(0xFFC49A9A),
        info = Color(0xFF9AABBC),
        focus = Color(0xFF60A5FA),
    )

/**
 * primary = accent、background = bone、surface = カード、outline = border、surfaceVariant = accent-soft、onSurfaceVariant = mute、error = danger。
 * surfaceTint は透明にする（tonal elevation で surface が灰色に染まり、ダイアログ・メニューの面がトークンの surface から外れるのを防ぐ）。
 */
internal val LightColors: ColorScheme =
    lightColorScheme(
        primary = AccentLight,
        onPrimary = BoneLight,
        primaryContainer = AccentSoftLight,
        onPrimaryContainer = InkLight,
        secondary = AccentLight,
        onSecondary = BoneLight,
        secondaryContainer = AccentSoftLight,
        onSecondaryContainer = InkLight,
        tertiary = LightExtraColors.info,
        onTertiary = BoneLight,
        tertiaryContainer = AccentSoftLight,
        onTertiaryContainer = InkLight,
        background = BoneLight,
        onBackground = InkLight,
        surface = SurfaceLight,
        onSurface = InkLight,
        surfaceVariant = AccentSoftLight,
        onSurfaceVariant = MuteLight,
        surfaceTint = Color.Transparent,
        inverseSurface = InkLight,
        inverseOnSurface = BoneLight,
        inversePrimary = BoneLight,
        error = LightExtraColors.danger,
        onError = BoneLight,
        errorContainer = AccentSoftLight,
        onErrorContainer = LightExtraColors.danger,
        outline = BorderLight,
        outlineVariant = BorderLight,
        scrim = Color.Black,
        surfaceBright = SurfaceLight,
        surfaceDim = BoneLight,
        surfaceContainerLowest = SurfaceLight,
        surfaceContainerLow = SurfaceLight,
        surfaceContainer = SurfaceLight,
        surfaceContainerHigh = SurfaceLight,
        surfaceContainerHighest = AccentSoftLight,
    )

internal val DarkColors: ColorScheme =
    darkColorScheme(
        primary = AccentDark,
        onPrimary = BoneDark,
        primaryContainer = AccentSoftDark,
        onPrimaryContainer = InkDark,
        secondary = AccentDark,
        onSecondary = BoneDark,
        secondaryContainer = AccentSoftDark,
        onSecondaryContainer = InkDark,
        tertiary = DarkExtraColors.info,
        onTertiary = BoneDark,
        tertiaryContainer = AccentSoftDark,
        onTertiaryContainer = InkDark,
        background = BoneDark,
        onBackground = InkDark,
        surface = SurfaceDark,
        onSurface = InkDark,
        surfaceVariant = AccentSoftDark,
        onSurfaceVariant = MuteDark,
        surfaceTint = Color.Transparent,
        inverseSurface = InkDark,
        inverseOnSurface = BoneDark,
        inversePrimary = BoneDark,
        error = DarkExtraColors.danger,
        onError = BoneDark,
        errorContainer = AccentSoftDark,
        onErrorContainer = DarkExtraColors.danger,
        outline = BorderDark,
        outlineVariant = BorderDark,
        scrim = Color.Black,
        surfaceBright = SurfaceDark,
        surfaceDim = BoneDark,
        surfaceContainerLowest = SurfaceDark,
        surfaceContainerLow = SurfaceDark,
        surfaceContainer = SurfaceDark,
        surfaceContainerHigh = SurfaceDark,
        surfaceContainerHighest = AccentSoftDark,
    )
