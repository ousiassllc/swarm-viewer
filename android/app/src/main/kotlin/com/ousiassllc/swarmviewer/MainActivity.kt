package com.ousiassllc.swarmviewer

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.material3.windowsizeclass.ExperimentalMaterial3WindowSizeClassApi
import androidx.compose.material3.windowsizeclass.calculateWindowSizeClass
import androidx.compose.ui.Modifier
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsViewModel
import com.ousiassllc.swarmviewer.ui.main.MainViewModel
import com.ousiassllc.swarmviewer.ui.navigation.AppNavigation
import com.ousiassllc.swarmviewer.ui.navigation.NavigationViewModel
import com.ousiassllc.swarmviewer.ui.pairing.PairingViewModel
import com.ousiassllc.swarmviewer.ui.settings.SettingsViewModel
import com.ousiassllc.swarmviewer.ui.theme.SwarmViewerTheme

/** ディープリンク（`swarm-viewer://pair?...`）の scheme（マニフェストの intent-filter と同じ）。 */
private const val PAIRING_SCHEME = "swarm-viewer"

class MainActivity : ComponentActivity() {
    private val container by lazy { AppContainer.from(this) }
    private val navigation: NavigationViewModel by viewModels()
    private val pairing: PairingViewModel by viewModels { viewModelFactory { initializer { PairingViewModel(container.pairing) } } }
    private val connections: ConnectionsViewModel by viewModels {
        viewModelFactory { initializer { ConnectionsViewModel(container.connections) } }
    }
    private val main: MainViewModel by viewModels {
        viewModelFactory { initializer { MainViewModel(container.snapshots, container.connections, container.settings) } }
    }
    private val settings: SettingsViewModel by viewModels {
        viewModelFactory { initializer { SettingsViewModel(container.settings) } }
    }

    @OptIn(ExperimentalMaterial3WindowSizeClassApi::class)
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        // 構成変更での再生成（savedInstanceState あり）では、すでに処理したリンクを再び処理しない。
        if (savedInstanceState == null) handleDeepLink(intent)
        setContent {
            SwarmViewerTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    // 幅の区分（Compact / Medium / Expanded）で表示を切り替える。折りたたみ⇔展開で変わる。
                    AppNavigation(
                        navigation = navigation,
                        main = main,
                        settings = settings,
                        connections = connections,
                        pairing = pairing,
                        widthClass = calculateWindowSizeClass(this).widthSizeClass,
                    )
                }
            }
        }
    }

    /** アプリが起動済みのときのディープリンク（`launchMode="singleTask"`）。 */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleDeepLink(intent)
    }

    /**
     * `swarm-viewer://pair?...` を、貼り付けと同じ経路（確認画面）に渡す。リンクは他のアプリからも送れる信頼できない入力として、
     * 検証は `parsePairing` が行い、利用者が確定するまで保存しない。リンク（token を含む）はログに出さず、処理後は Intent のデータを消す。
     */
    private fun handleDeepLink(intent: Intent) {
        val data = intent.data ?: return
        if (intent.action != Intent.ACTION_VIEW || !PAIRING_SCHEME.equals(data.scheme, ignoreCase = true)) return
        val raw = data.toString()
        intent.data = null
        navigation.showPairing()
        pairing.receive(raw)
    }
}
