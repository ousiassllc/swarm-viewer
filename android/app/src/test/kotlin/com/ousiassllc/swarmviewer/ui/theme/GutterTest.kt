package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Test

/** 画面のガターはガイド S5 のとおり、Compact（モバイル）は 16dp・Medium 以上（md 相当）は 24dp であること。 */
class GutterTest {
    @Test
    fun compactIs16dpAndWideIs24dp() {
        assertEquals(16.dp, gutterFor(wide = false))
        assertEquals(24.dp, gutterFor(wide = true))
    }
}
