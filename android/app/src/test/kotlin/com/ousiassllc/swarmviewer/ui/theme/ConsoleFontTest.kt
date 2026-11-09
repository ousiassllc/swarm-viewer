package com.ousiassllc.swarmviewer.ui.theme

import com.ousiassllc.swarmviewer.R
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * ペイン詳細のコンソール本文（`pane_tail`）の書体は、JetBrains Mono の次に同梱の Symbols Nerd Font Mono へ代替すること（Issue #102）。
 * Symbols が抜けると、本文の Nerd Font のアイコンが □ になる（字形があることは [SymbolsFontTest] が確かめる）。
 */
class ConsoleFontTest {
    @Test
    fun consoleFallsBackFromJetBrainsMonoToSymbolsNerdFont() {
        assertEquals(listOf(R.font.jetbrains_mono_regular, R.font.symbols_nerd_font_mono_regular), CONSOLE_FONT_CHAIN)
    }
}
