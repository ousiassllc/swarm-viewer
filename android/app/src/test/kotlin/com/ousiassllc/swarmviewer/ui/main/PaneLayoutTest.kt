package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Attention
import com.ousiassllc.swarmviewer.domain.Layout
import com.ousiassllc.swarmviewer.domain.Pane
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PaneLayoutTest {
    private fun panes(vararg pos: String) = pos.map { Pane(pos = it, id = "%$it") }

    @Test
    fun gridRowsSplitsPanesByTheLayoutColumns() {
        val rows = gridRows(Layout(rows = 2, cols = listOf(3, 2)), panes("T1", "T2", "T3", "B1", "B2"))

        assertEquals(listOf(listOf("T1", "T2", "T3"), listOf("B1", "B2")), rows?.map { row -> row.map { it.pos } })
    }

    @Test
    fun gridRowsIsNullWhenTheLayoutIsMissingOrInconsistent() {
        val five = panes("T1", "T2", "T3", "B1", "B2")
        assertNull(gridRows(Layout(), five)) // 欠落
        assertNull(gridRows(Layout(rows = 2, cols = listOf(3, 3)), five)) // 合計が合わない
        assertNull(gridRows(Layout(rows = 1, cols = listOf(3, 2)), five)) // 段数が合わない
        assertNull(gridRows(Layout(rows = 1, cols = listOf(0)), emptyList())) // ペインが 0 件
    }

    @Test
    fun hasAttentionMatchesThePaneAndKindOnlyAndIgnoresEmptyPositions() {
        val attention = listOf(Attention("asking", "T2", "q"), Attention("report_waiting", "", "path"))

        assertTrue(hasAttention(attention, "T2", "asking", "stalled"))
        assertFalse(hasAttention(attention, "T2", "report_waiting"))
        assertFalse(hasAttention(attention, "T1", "asking"))
        assertFalse(hasAttention(attention, "", "report_waiting")) // pane が空の項目はどのカードにも付けない
    }

    @Test
    fun groupAttentionSeparatesPaneItemsFromAgentItemsAndSortsBoth() {
        val groups =
            groupAttention(
                listOf(
                    Attention("stalled", "B1", "3m"),
                    Attention("auth_expired", "", "codex"),
                    Attention("asking", "T2", "q"),
                    Attention("auth_expiring", "", "claude"),
                    Attention("custom", "", "x"), // 未知の kind で pane が無い → ログイン関連の側
                    Attention("auth_expired", "T1", "claude"), // auth は pane があってもログイン関連
                    Attention("asking", "T10", "q"),
                    Attention("asking", "T1", "q"),
                    Attention("asking", "Z9", "q"),
                ),
            )

        assertEquals(listOf("T1", "T2", "T10", "B1", "Z9"), groups.panes.map { it.pane })
        assertEquals(listOf("claude", "codex", "x"), groups.agents.map { it.agent })
        assertEquals(
            2,
            groups.agents
                .first()
                .items.size,
        ) // 同じエージェントは 1 ブロック
    }

    @Test
    fun groupAttentionIsEmptyForNoItems() {
        val groups = groupAttention(emptyList())
        assertTrue(groups.panes.isEmpty() && groups.agents.isEmpty())
    }
}
