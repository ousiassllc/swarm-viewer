package com.ousiassllc.swarmviewer.ui.theme

import android.content.Context
import android.graphics.Typeface
import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.AndroidFont
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontLoadingStrategy
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import com.ousiassllc.swarmviewer.R
import java.io.IOException
import android.graphics.fonts.Font as PlatformFont
import android.graphics.fonts.FontFamily as PlatformFontFamily
import android.graphics.fonts.FontStyle as PlatformFontStyle

/**
 * UI の書体は Sora（欧文）と Noto Sans JP（和文）。Compose の [FontFamily] は 1 つの書体しか選べず、グリフごとの代替を持てない。
 * Sora には日本語のグリフが無いので、Android の [Typeface.CustomFallbackBuilder]（API 29 以上。minSdk は 29）で
 * 「Sora → 同梱の Noto Sans JP → システムのフォント」の順に代替する書体を作り、ウェイトごとに [AndroidFont] として [FontFamily] に並べる。
 * これで日本語は同梱の Noto Sans JP（JIS X 0208 の部分集合）で描かれ、含まれない字だけシステムのフォントになる。
 * ウェイトは Regular（400）と SemiBold（600）だけ。SemiBold は見出しとボタンにだけ使う。
 */
private class UiFont(
    override val weight: FontWeight,
    private val latinRes: Int,
    private val japaneseRes: Int,
) : AndroidFont(FontLoadingStrategy.Blocking, UiFontLoader, FontVariation.Settings()) {
    override val style = FontStyle.Normal

    fun load(context: Context): Typeface =
        try {
            val resources = context.resources

            fun family(res: Int) = PlatformFontFamily.Builder(PlatformFont.Builder(resources, res).build()).build()
            Typeface
                .CustomFallbackBuilder(family(latinRes))
                .addCustomFallback(family(japaneseRes))
                .setSystemFallback("sans-serif")
                .setStyle(PlatformFontStyle(weight.weight, PlatformFontStyle.FONT_SLANT_UPRIGHT))
                .build()
        } catch (e: IOException) {
            // 同梱のフォントが読めない（通常は起きない）ときは、読めないまま文字を消さずにシステムのフォントで描く。
            Typeface.create(Typeface.DEFAULT, weight.weight, false)
        }
}

private object UiFontLoader : AndroidFont.TypefaceLoader {
    override fun loadBlocking(
        context: Context,
        font: AndroidFont,
    ): Typeface = (font as UiFont).load(context)

    override suspend fun awaitLoad(
        context: Context,
        font: AndroidFont,
    ): Typeface = loadBlocking(context, font)
}

/** UI の本文・見出し・ボタン（Sora + Noto Sans JP）。 */
val UiFontFamily =
    FontFamily(
        UiFont(FontWeight.Normal, R.font.sora_regular, R.font.noto_sans_jp_regular),
        UiFont(FontWeight.SemiBold, R.font.sora_semibold, R.font.noto_sans_jp_semibold),
    )

/** コード・桁をそろえる数値（JetBrains Mono。Regular のみ）。 */
val MonoFontFamily = FontFamily(Font(R.font.jetbrains_mono_regular, FontWeight.Normal))

/** サイズ（sp）。ガイドの xs 12 / sm 14 / base 16 / lg 18 / xl 24 / 2xl 32 / 3xl 40 と、その行間（倍率 1.6 / 1.7 / 1.8 / 1.7 / 1.5 / 1.4 / 1.3）。 */
private fun style(
    size: Int,
    lineHeightRatio: Float,
    weight: FontWeight,
) = TextStyle(
    fontFamily = UiFontFamily,
    fontWeight = weight,
    fontSize = size.sp,
    lineHeight = (size * lineHeightRatio).sp,
)

private val Regular = FontWeight.Normal
private val SemiBold = FontWeight.SemiBold

/**
 * Regular が既定で、見出し（display / headline / title）とボタン（labelLarge）だけ SemiBold。
 * 文章は 14sp 以上（bodyMedium 以上）。ボタン（labelLarge）は標準の高さ 40dp なので `base` の 16sp（T3）。
 * 12sp（bodySmall / labelMedium / labelSmall）は注記・バッジだけに使う。
 */
internal val GuideTypography =
    Typography(
        displayLarge = style(40, 1.3f, SemiBold),
        displayMedium = style(32, 1.4f, SemiBold),
        displaySmall = style(24, 1.5f, SemiBold),
        headlineLarge = style(32, 1.4f, SemiBold),
        headlineMedium = style(24, 1.5f, SemiBold),
        headlineSmall = style(18, 1.7f, SemiBold),
        titleLarge = style(18, 1.7f, SemiBold),
        titleMedium = style(16, 1.8f, SemiBold),
        titleSmall = style(14, 1.7f, SemiBold),
        bodyLarge = style(16, 1.8f, Regular),
        bodyMedium = style(14, 1.7f, Regular),
        bodySmall = style(12, 1.6f, Regular),
        labelLarge = style(16, 1.8f, SemiBold),
        labelMedium = style(12, 1.6f, Regular),
        labelSmall = style(12, 1.6f, Regular),
    )
