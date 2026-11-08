package com.ousiassllc.swarmviewer.ui.navigation

import com.ousiassllc.swarmviewer.service.PaneTarget
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class NavigationViewModelTest {
    @Test
    fun theAppStartsOnTheMainScreenAndBackReturnsToIt() {
        val navigation = NavigationViewModel()
        assertEquals(Route.Main, navigation.current.value)

        navigation.push(Route.Settings)
        navigation.push(Route.Connections)
        assertEquals(Route.Connections, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Settings, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Main, navigation.current.value)
        navigation.pop() // 起点では何もしない
        assertEquals(Route.Main, navigation.current.value)
    }

    @Test
    fun aDeepLinkOpensPairingOnTopOfConnectionsAndBackWalksDown() {
        val navigation = NavigationViewModel()
        navigation.push(Route.Settings)

        navigation.showPairing()
        assertEquals(Route.Pairing, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Connections, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Main, navigation.current.value)
    }

    @Test
    fun savingAConnectionReturnsToTheMainScreenAndConnectionsCanBeShownDirectly() {
        val navigation = NavigationViewModel()
        navigation.showConnections()
        assertEquals(Route.Connections, navigation.current.value)

        navigation.push(Route.Pairing)
        navigation.showMain()
        assertEquals(Route.Main, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Main, navigation.current.value)
    }

    @Test
    fun openingAPaneDetailKeepsItsTargetAndBackReturnsToTheMainScreen() {
        val navigation = NavigationViewModel()
        val target = PaneTarget("c1", "s0", "T1")

        navigation.showPaneDetail(target)
        assertEquals(Route.PaneDetail, navigation.current.value)
        assertEquals(target, navigation.target.value)

        navigation.pop()
        assertEquals(Route.Main, navigation.current.value)
        assertNull(navigation.target.value)
    }

    @Test
    fun startingASessionReplacesTheStartScreenWithThePaneDetail() {
        val navigation = NavigationViewModel()
        val shell = PaneTarget("c1", "s0", "B1")

        navigation.showStartSession(shell)
        assertEquals(Route.StartSession, navigation.current.value)
        navigation.finishStart(shell)

        assertEquals(Route.PaneDetail, navigation.current.value)
        assertEquals(shell, navigation.target.value)
        navigation.pop()
        assertEquals(Route.Main, navigation.current.value) // 新規セッションの画面には戻らない
    }

    @Test
    fun startingFromAPaneDetailReturnsToThatDetailWithoutStackingAnother() {
        val navigation = NavigationViewModel()
        val shell = PaneTarget("c1", "s0", "B1")
        navigation.showPaneDetail(shell)
        navigation.showStartSession(shell)

        navigation.finishStart(shell)

        assertEquals(Route.PaneDetail, navigation.current.value)
        navigation.pop()
        assertEquals(Route.Main, navigation.current.value)
    }
}
