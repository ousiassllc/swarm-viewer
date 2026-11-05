package com.ousiassllc.swarmviewer.ui.navigation

import org.junit.Assert.assertEquals
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
}
