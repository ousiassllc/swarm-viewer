package com.ousiassllc.swarmviewer.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsScreen
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsViewModel
import com.ousiassllc.swarmviewer.ui.pairing.PairingScreen
import com.ousiassllc.swarmviewer.ui.pairing.PairingViewModel
import com.ousiassllc.swarmviewer.ui.pairing.QrScanScreen
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/** 画面。メイン画面（A2）・設定画面（A3）は後続の Issue（#20）がここに足す。 */
enum class Route {
    /** 接続管理（S1）。起点。 */
    Connections,

    /** ペアリング（貼り付け・確認）。ディープリンク・QR の結果も、この画面の確認に入る。 */
    Pairing,

    /** QR 読み取り。 */
    Scan,
}

/**
 * 画面の遷移（バックスタック）。ディープリンクのように Activity から画面を切り替える必要があるため、ViewModel に置く
 * （構成変更で保たれる。プロセスの終了後は [Route.Connections] から始まる）。
 */
class NavigationViewModel : ViewModel() {
    private val stack = MutableStateFlow(listOf(Route.Connections))
    private val mutableCurrent = MutableStateFlow(Route.Connections)
    val current: StateFlow<Route> = mutableCurrent.asStateFlow()

    fun push(route: Route) = set { it + route }

    /** 1 つ戻る。起点では何もしない（Activity の戻る操作に任せる）。 */
    fun pop() = set { if (it.size > 1) it.dropLast(1) else it }

    /** どの画面にいても、ペアリング画面を前面に出す（ディープリンク）。接続管理の上に重ねる。 */
    fun showPairing() = set { listOf(Route.Connections, Route.Pairing) }

    /** 接続管理へ戻る（保存の完了後）。 */
    fun showConnections() = set { listOf(Route.Connections) }

    private fun set(transform: (List<Route>) -> List<Route>) {
        stack.update(transform)
        mutableCurrent.value = stack.value.last()
    }
}

/** 最小のナビゲーション骨格。 */
@Composable
fun AppNavigation(
    navigation: NavigationViewModel,
    connections: ConnectionsViewModel,
    pairing: PairingViewModel,
) {
    val route by navigation.current.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { pairing.saved.collect { navigation.showConnections() } }

    when (route) {
        Route.Connections ->
            ConnectionsScreen(
                viewModel = connections,
                onAdd = { navigation.push(Route.Pairing) },
                onRepair = { navigation.push(Route.Pairing) }, // 同じ接続先を貼り付けると token が差し替わる
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
