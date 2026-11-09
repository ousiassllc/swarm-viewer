package com.ousiassllc.swarmviewer.ui.theme

import androidx.compose.material3.windowsizeclass.WindowWidthSizeClass
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Test

/** 画面のガターは幅の区分で決まり、Compact 16dp・Medium 24dp・Expanded 32dp であること（NFR-DSG-10）。 */
class GutterTest {
    @Test
    fun gutterFollowsWidthSizeClass() {
        assertEquals(16.dp, gutterFor(WindowWidthSizeClass.Compact))
        assertEquals(24.dp, gutterFor(WindowWidthSizeClass.Medium))
        assertEquals(32.dp, gutterFor(WindowWidthSizeClass.Expanded))
    }
}
