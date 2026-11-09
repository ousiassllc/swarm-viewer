package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.domain.AskCard
import com.ousiassllc.swarmviewer.domain.AskOption
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.domain.PaneState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

private val ASKING_CARD = AskCard(true, "どうしますか？", listOf(AskOption(1, "A", true), AskOption(2, "B")), cursor = 1, allowsCustom = true)

/** ペイン詳細で何を描画するかの判定（見るだけの接続で操作 UI が出ないことの証明）。 */
class PaneControlsTest {
    @Test
    fun aViewOnlyConnectionNeverShowsAnyOperationInAnyState() {
        for (state in PaneState.entries) {
            for (card in listOf(null, ASKING_CARD, ASKING_CARD.copy(allowsCustom = false))) {
                val controls = paneControls(ConnectionKind.View, state, card)
                assertFalse("$state: 操作の部品が出ている", controls.anyOperation)
                assertFalse("$state: 送信欄", controls.showSendField)
                assertFalse("$state: 中断", controls.showEscape)
                assertFalse("$state: 終了", controls.showClose)
                assertFalse("$state: 選択肢のボタン", controls.showCardButtons)
                assertFalse("$state: その他の入力", controls.showCustomEntry)
                assertFalse("$state: 送れる", controls.sendEnabled)
            }
        }
    }

    @Test
    fun aViewOnlyAskingCardIsAListWithAHintInsteadOfButtons() {
        val controls = paneControls(ConnectionKind.View, PaneState.Asking, ASKING_CARD)

        assertTrue(controls.showCardList)
        assertTrue(controls.showViewOnlyHint)
        assertFalse(controls.showCardButtons)

        val quiet = paneControls(ConnectionKind.View, PaneState.Working, null)
        assertFalse(quiet.showCardList)
        assertFalse(quiet.showViewOnlyHint)
    }

    @Test
    fun anOpsAskingCardShowsButtonsAndTheCustomEntryOnlyWhenAllowed() {
        val controls = paneControls(ConnectionKind.Ops, PaneState.Asking, ASKING_CARD)
        assertTrue(controls.showCardList && controls.showCardButtons && controls.showCustomEntry)
        assertFalse(controls.showViewOnlyHint)

        val noCustom = paneControls(ConnectionKind.Ops, PaneState.Asking, ASKING_CARD.copy(allowsCustom = false))
        assertTrue(noCustom.showCardButtons)
        assertFalse(noCustom.showCustomEntry)

        val notAsking = paneControls(ConnectionKind.Ops, PaneState.Asking, AskCard(asking = false))
        assertFalse(notAsking.showCardList || notAsking.showCardButtons)
    }

    @Test
    fun theSendFieldIsEnabledOnlyForIdleAndStalledWithAOneLineReasonOtherwise() {
        val reasons =
            mapOf(
                PaneState.Working to "作業中のため送れません。終わるまで待つか、中断してください",
                PaneState.Asking to "質問に答えてから送れます",
                PaneState.Shell to "エージェントが動いていません。「始める」から起動してください",
                PaneState.Unknown to "このペインには送れません",
            )
        for ((state, reason) in reasons) {
            val controls = paneControls(ConnectionKind.Ops, state, null)
            assertTrue("$state: 送信欄は出す", controls.showSendField)
            assertFalse("$state: 無効", controls.sendEnabled)
            assertEquals(state.name, reason, controls.sendDisabledReason)
        }
        for (state in listOf(PaneState.Idle, PaneState.Stalled)) {
            val controls = paneControls(ConnectionKind.Ops, state, null)
            assertTrue(state.name, controls.sendEnabled)
            assertNull(controls.sendDisabledReason)
        }
    }

    @Test
    fun escapeIsOfferedForWorkingStalledAskingIdleAndCloseOnlyForIdleAndShell() {
        val escapable = setOf(PaneState.Working, PaneState.Stalled, PaneState.Asking, PaneState.Idle)
        val closable = setOf(PaneState.Idle, PaneState.Shell)
        for (state in PaneState.entries) {
            val controls = paneControls(ConnectionKind.Ops, state, null)
            assertEquals("$state: 中断", state in escapable, controls.showEscape)
            assertEquals("$state: 終了", state in closable, controls.showClose)
        }
    }

    @Test
    fun nothingIsShownBeforeThePaneIsKnown() {
        assertFalse(paneControls(ConnectionKind.Ops, null, null).anyOperation)
    }

    @Test
    fun theStartButtonIsOfferedOnlyForAShellPaneOfAnOpsConnection() {
        for (state in PaneState.entries) {
            assertEquals("$state: ops", state == PaneState.Shell, paneControls(ConnectionKind.Ops, state, null).showStart)
            assertFalse("$state: 見るだけ", paneControls(ConnectionKind.View, state, null).showStart) // 見るだけでは出ない
            assertFalse("$state: 見るだけ（質問あり）", paneControls(ConnectionKind.View, state, ASKING_CARD).showStart)
        }
        assertFalse(paneControls(ConnectionKind.Ops, null, null).showStart)
        assertTrue(paneControls(ConnectionKind.Ops, PaneState.Shell, null).anyOperation)
    }
}
