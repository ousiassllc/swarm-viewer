package com.ousiassllc.swarmviewer.ui.theme

import android.content.Context
import android.graphics.Typeface
import androidx.compose.ui.text.font.AndroidFont
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontLoadingStrategy
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import com.ousiassllc.swarmviewer.R
import java.io.IOException
import android.graphics.fonts.Font as PlatformFont
import android.graphics.fonts.FontFamily as PlatformFontFamily
import android.graphics.fonts.FontStyle as PlatformFontStyle

/**
 * `pane_tail` の本文の書体の代替の順（同梱のフォント）: JetBrains Mono → Symbols Nerd Font Mono。最後にシステムの等幅フォントへ落ちる。
 * Symbols を外すと、本文の Nerd Font のアイコンが □ に戻る（Issue #102）。
 */
internal val CONSOLE_FONT_CHAIN: List<Int> = listOf(R.font.jetbrains_mono_regular, R.font.symbols_nerd_font_mono_regular)

/**
 * `pane_tail`（ホストのペインの画面）の本文用の書体。ペインの画面には Nerd Font のアイコン（私用領域 PUA）が混ざり、
 * JetBrains Mono にもシステムの等幅フォントにも字形が無いので、そのままでは □ になる（Issue #102）。
 * Compose の [FontFamily] はグリフごとの代替を持てないので、[UiFontFamily] と同じく
 * Android の [Typeface.CustomFallbackBuilder]（API 29 以上）で [CONSOLE_FONT_CHAIN] の順に代替し、最後はシステムの等幅フォントにする。
 * Symbols は PUA のアイコンだけを持つので、ほかの字（ASCII・日本語）は JetBrains Mono とシステムのフォントで描かれる。
 */
private object ConsoleFont : AndroidFont(FontLoadingStrategy.Blocking, ConsoleFontLoader, FontVariation.Settings()) {
    override val weight = FontWeight.Normal
    override val style = FontStyle.Normal

    fun load(context: Context): Typeface =
        try {
            val resources = context.resources

            fun family(res: Int) = PlatformFontFamily.Builder(PlatformFont.Builder(resources, res).build()).build()
            val builder = Typeface.CustomFallbackBuilder(family(CONSOLE_FONT_CHAIN.first()))
            CONSOLE_FONT_CHAIN.drop(1).forEach { builder.addCustomFallback(family(it)) }
            builder
                .setSystemFallback("monospace")
                .setStyle(PlatformFontStyle(FontWeight.Normal.weight, PlatformFontStyle.FONT_SLANT_UPRIGHT))
                .build()
        } catch (e: IOException) {
            // 同梱のフォントが読めない（通常は起きない）ときは、文字を消さずにシステムの等幅フォントで描く。
            Typeface.MONOSPACE
        }
}

private object ConsoleFontLoader : AndroidFont.TypefaceLoader {
    override fun loadBlocking(
        context: Context,
        font: AndroidFont,
    ): Typeface = (font as ConsoleFont).load(context)

    override suspend fun awaitLoad(
        context: Context,
        font: AndroidFont,
    ): Typeface = loadBlocking(context, font)
}

/** `pane_tail` の本文（等幅。Nerd Font のアイコンも字形で出す）。 */
val ConsoleFontFamily = FontFamily(ConsoleFont)
