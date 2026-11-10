package com.ousiassllc.swarmviewer.ui.navigation

import com.ousiassllc.swarmviewer.service.PaneTarget
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 左上の「戻る」（とシステムの戻る）の契約: どの画面にいても、[NavigationViewModel.pop] を押し続ければ必ずメインへ着く
 * （画面が変わらないまま動かない行き止まりが無い）。Issue #100（「メイン画面に戻る」が効かない）の再発を、画面の遷移の側で固定する。
 * 実際のボタンの押下（ヒットテスト）は Compose の描画に依存するため、ここでは検査しない（エミュレータで確認する）。
 */
class NavigationBackContractTest {
    private val pane = PaneTarget("c1", "s0", "T1")
    private val shell = PaneTarget("c1", "s0", "B1")

    /** 画面への入り方。「戻る」を持つ全画面と、その入口（メイン・詳細・通知・ディープリンク・起動直後の接続管理）を網羅する。 */
    private val entries: Map<String, (NavigationViewModel) -> Unit> =
        mapOf(
            "main → settings" to { n -> n.push(Route.Settings) },
            "main → connections" to { n -> n.push(Route.Connections) },
            "main → pairing（再ペアリング）" to { n -> n.push(Route.Pairing) },
            "main → pane detail" to { n -> n.showPaneDetail(pane) },
            "main → start session（shell ペインのカード）" to { n -> n.showStartSession(shell) },
            "pane detail → start session" to { n ->
                n.showPaneDetail(shell)
                n.showStartSession(shell)
            },
            "pane detail → pairing（再ペアリング）" to { n ->
                n.showPaneDetail(pane)
                n.push(Route.Pairing)
            },
            "start session → pairing（再ペアリング）" to { n ->
                n.showStartSession(shell)
                n.push(Route.Pairing)
            },
            "settings → connections → pairing → scan" to { n ->
                n.push(Route.Settings)
                n.push(Route.Connections)
                n.push(Route.Pairing)
                n.push(Route.Scan)
            },
            "deep link（ペアリング）" to { n -> n.showPairing() },
            "起動直後の接続管理（接続が 0 件）" to { n -> n.showConnections() },
            "start session → 起動完了で詳細（詳細を積んでいない）" to { n ->
                n.showStartSession(shell)
                n.finishStart(shell)
            },
        )

    @Test
    fun poppingFromAnyScreenAlwaysReachesTheMainScreenWithoutGettingStuck() {
        for ((name, enter) in entries) {
            val navigation = NavigationViewModel()
            enter(navigation)
            assertNotEquals("$name: 入った先がメインのまま", Route.Main, navigation.current.value)

            var pops = 0
            while (navigation.current.value != Route.Main) {
                val before = navigation.current.value
                navigation.pop()
                pops++
                assertNotEquals("$name: 戻っても $before から動かない（行き止まり）", before, navigation.current.value)
                assertTrue("$name: 戻る回数が深さの上限を超えた", pops <= MAX_DEPTH)
            }
            assertNull("$name: メインでは対象のペインが残らない", navigation.target.value)
        }
    }

    @Test
    fun theTargetFollowsTheScreenOnTheTopWhileWalkingBack() {
        val navigation = NavigationViewModel()
        navigation.showPaneDetail(shell)
        navigation.showStartSession(shell)
        assertEquals(shell, navigation.target.value)

        navigation.pop() // 新しいセッション → 詳細（対象は同じペイン）
        assertEquals(Route.PaneDetail, navigation.current.value)
        assertEquals(shell, navigation.target.value)

        navigation.pop() // 詳細 → メイン
        assertEquals(Route.Main, navigation.current.value)
        assertNull(navigation.target.value)
    }

    @Test
    fun aDeepLinkWhileADetailIsOpenReplacesTheStackSoBackStillEndsOnTheMainScreen() {
        val navigation = NavigationViewModel()
        navigation.showPaneDetail(pane)

        navigation.showPairing()
        assertNull(navigation.target.value) // 前のペインの対象を引きずらない
        navigation.pop()
        navigation.pop()

        assertEquals(Route.Main, navigation.current.value)
    }

    private companion object {
        /** 積めるスタックの最大の深さ（設定 → 接続 → ペアリング → QR + メイン）。 */
        const val MAX_DEPTH = 4
    }
}
