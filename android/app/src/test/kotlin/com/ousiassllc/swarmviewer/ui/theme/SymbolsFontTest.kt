package com.ousiassllc.swarmviewer.ui.theme

import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.nio.ByteBuffer

/**
 * 同梱の Symbols Nerd Font Mono（Issue #81）が、実機で確認した omp のペインタイトルのアイコン
 * （デスクトップの Issue #9 と同じ U+F1A99・U+F0EAC・U+F41B・U+F04C2・U+F0584）の字形を持つこと。
 * TTF の cmap（format 12）を直接読むので、フォントを別のファイルや別の版に差し替えて字形が消えたら落ちる。
 */
class SymbolsFontTest {
    private val font = File("src/main/res/font/symbols_nerd_font_mono_regular.ttf")

    @Test
    fun hasGlyphsForPaneTitleIcons() {
        val ranges = cmapFormat12Ranges(ByteBuffer.wrap(font.readBytes()))
        val icons = listOf(0xF1A99, 0xF0EAC, 0xF41B, 0xF04C2, 0xF0584)
        icons.forEach { cp ->
            assertTrue("U+${cp.toString(16).uppercase()} の字形がない", ranges.any { cp in it })
        }
    }

    /** sfnt のテーブルから cmap を探し、format 12 の副表のグリフがある範囲（コードポイント）を返す。 */
    private fun cmapFormat12Ranges(buf: ByteBuffer): List<IntRange> {
        val tableCount = buf.getShort(4).toInt() and 0xFFFF
        val cmapOffset =
            (0 until tableCount)
                .map { 12 + it * 16 }
                .first { tag(buf, it) == "cmap" }
                .let { buf.getInt(it + 8) }
        val subtableCount = buf.getShort(cmapOffset + 2).toInt() and 0xFFFF
        for (i in 0 until subtableCount) {
            val subtable = cmapOffset + buf.getInt(cmapOffset + 4 + i * 8 + 4)
            if ((buf.getShort(subtable).toInt() and 0xFFFF) != 12) continue
            val groups = buf.getInt(subtable + 12)
            return (0 until groups).map {
                val group = subtable + 16 + it * 12
                buf.getInt(group)..buf.getInt(group + 4)
            }
        }
        error("cmap に format 12 の副表がない")
    }

    private fun tag(
        buf: ByteBuffer,
        offset: Int,
    ) = String(CharArray(4) { buf.get(offset + it).toInt().toChar() })
}
