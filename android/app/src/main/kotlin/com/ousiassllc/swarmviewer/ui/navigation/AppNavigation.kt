package com.ousiassllc.swarmviewer.ui.navigation

import androidx.activity.compose.BackHandler
import androidx.compose.material3.windowsizeclass.WindowWidthSizeClass
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsScreen
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsViewModel
import com.ousiassllc.swarmviewer.ui.main.MainScreen
import com.ousiassllc.swarmviewer.ui.main.MainViewModel
import com.ousiassllc.swarmviewer.ui.pairing.PairingScreen
import com.ousiassllc.swarmviewer.ui.pairing.PairingViewModel
import com.ousiassllc.swarmviewer.ui.pairing.QrScanScreen
import com.ousiassllc.swarmviewer.ui.settings.SettingsScreen
import com.ousiassllc.swarmviewer.ui.settings.SettingsViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/** 画面。 */
enum class Route {
    /** メイン（A2）。起点。接続が 0 件のときは、起動直後に一度だけ接続管理へ進む。 */
    Main,

    /** 設定（A3）。 */
    Settings,

    /** 接続管理（S1）。 */
    Connections,

    /** ペアリング（貼り付け・確認）。ディープリンク・QR の結果も、この画面の確認に入る。 */
    Pairing,

    /** QR 読み取り。 */
    Scan,
}

/**
 * 画面の遷移（バックスタック）。ディープリンクのように Activity から画面を切り替える必要があるため、ViewModel に置く
 * （構成変更で保たれる。プロセスの終了後は [Route.Main] から始まる）。
 */
class NavigationViewModel : ViewModel() {
    private val stack = MutableStateFlow(listOf(Route.Main))
    private val mutableCurrent = MutableStateFlow(Route.Main)
    val current: StateFlow<Route> = mutableCurrent.asStateFlow()

    fun push(route: Route) = set { it + route }

    /** 1 つ戻る。起点では何もしない（Activity の戻る操作に任せる）。 */
    fun pop() = set { if (it.size > 1) it.dropLast(1) else it }

    /** どの画面にいても、ペアリング画面を前面に出す（ディープリンク）。接続管理の上に重ねる。 */
    fun showPairing() = set { listOf(Route.Main, Route.Connections, Route.Pairing) }

    /** 接続管理を前面に出す（接続が 0 件のときの起点）。 */
    fun showConnections() = set { listOf(Route.Main, Route.Connections) }

    /** メインへ戻る（ペアリングの保存の完了後）。 */
    fun showMain() = set { listOf(Route.Main) }

    private fun set(transform: (List<Route>) -> List<Route>) {
        stack.update(transform)
        mutableCurrent.value = stack.value.last()
    }
}

/** ナビゲーションの骨格。システムの戻る操作は 1 つ前の画面に戻る（メインでは Activity に任せる）。 */
@Composable
fun AppNavigation(
    navigation: NavigationViewModel,
    main: MainViewModel,
    settings: SettingsViewModel,
    connections: ConnectionsViewModel,
    pairing: PairingViewModel,
    widthClass: WindowWidthSizeClass,
) {
    val route by navigation.current.collectAsStateWithLifecycle()
    val mainState by main.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { pairing.saved.collect { navigation.showMain() } }
    BackHandler(enabled = route != Route.Main) { navigation.pop() }

    // 接続が 0 件のまま起動したときだけ、一度だけ接続管理（A1）へ進む（機能要件 §12.2）。ディープリンクなどで他の画面が先なら何もしない。
    var startupChecked by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(route, mainState.connections) {
        if (startupChecked || route != Route.Main || mainState.connections == null) return@LaunchedEffect
        startupChecked = true
        if (mainState.noConnections) navigation.showConnections()
    }

    when (route) {
        Route.Main ->
            MainScreen(
                viewModel = main,
                widthClass = widthClass,
                onManageConnections = { navigation.push(Route.Connections) },
                onSettings = { navigation.push(Route.Settings) },
                onRepair = { navigation.push(Route.Pairing) }, // 同じ接続先を貼り付けると token が差し替わる
            )
        Route.Settings ->
            SettingsScreen(
                viewModel = settings,
                onOpenConnections = { navigation.push(Route.Connections) },
                onBack = { navigation.pop() },
            )
        Route.Connections ->
            ConnectionsScreen(
                viewModel = connections,
                onAdd = { navigation.push(Route.Pairing) },
                onRepair = { navigation.push(Route.Pairing) }, // 同じ接続先を貼り付けると token が差し替わる
                onBack = { navigation.pop() },
            )
        Route.Pairing ->
            PairingScreen(
                viewModel = pairing,
                onScan = { navigation.push(Route.Scan) },
                onBack = { navigation.pop() },
            )
        Route.Scan ->
            QrScanScreen(
                onScanned = { raw ->
                    navigation.pop()
                    pairing.receive(raw)
                },
                onBack = { navigation.pop() },
            )
    }
}
