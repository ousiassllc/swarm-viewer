package com.ousiassllc.swarmviewer

import android.Manifest
import android.content.Intent
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.windowsizeclass.ExperimentalMaterial3WindowSizeClassApi
import androidx.compose.material3.windowsizeclass.calculateWindowSizeClass
import androidx.compose.ui.Modifier
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.ousiassllc.swarmviewer.notify.SessionLink
import com.ousiassllc.swarmviewer.ui.connections.ConnectionsViewModel
import com.ousiassllc.swarmviewer.ui.main.MainViewModel
import com.ousiassllc.swarmviewer.ui.navigation.AppNavigation
import com.ousiassllc.swarmviewer.ui.navigation.NavigationViewModel
import com.ousiassllc.swarmviewer.ui.pairing.PairingViewModel
import com.ousiassllc.swarmviewer.ui.settings.SettingsViewModel
import com.ousiassllc.swarmviewer.ui.theme.SwarmViewerTheme
import kotlinx.coroutines.launch

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
        viewModelFactory { initializer { SettingsViewModel(container.settings, container.permission::isAllowed) } }
    }

    // 通知の権限の要求結果。許可されなくても使える（通知が出ないだけで、設定画面に案内が出る）ので、結果は使わない。
    private val requestNotificationPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    @OptIn(ExperimentalMaterial3WindowSizeClassApi::class)
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        // 構成変更での再生成（savedInstanceState あり）では、すでに処理したリンクを再び処理しない。
        if (savedInstanceState == null) {
            handleDeepLink(intent)
            handleSessionLink(intent)
            startNotifications()
        }
        setContent {
            SwarmViewerTheme {
                Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
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
        handleSessionLink(intent)
    }

    /**
     * 通知の設定どおりに周期ジョブを登録し直し（アプリの起動時。再起動後の再開と合わせて、設定どおりのジョブを保つ）、
     * 初回だけ通知の権限（Android 13 以降）を要求する。拒否されたら、設定画面に案内が出る。
     */
    private fun startNotifications() {
        lifecycleScope.launch {
            val settings = container.settings.get()
            container.scheduler.apply(settings)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                container.permission.takeInitialRequest(settings.notificationsEnabled)
            ) {
                requestNotificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }

    /**
     * 通知のタップ（該当接続×セッションを開く）。外部のアプリからも送れる Intent のため、[SessionLink.parse] で検証した ID だけを使い、
     * 存在しない接続・許可されていないセッションは [MainViewModel] が無視・先頭の許可セッションにする。処理後は Intent から対象を消す。
     */
    private fun handleSessionLink(intent: Intent) {
        val target = SessionLink.parse(intent) ?: return
        SessionLink.consume(intent)
        navigation.showMain()
        main.openSession(target.connectionId, target.session)
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
