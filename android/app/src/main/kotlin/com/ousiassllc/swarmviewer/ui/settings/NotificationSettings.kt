package com.ousiassllc.swarmviewer.ui.settings

import android.Manifest
import android.content.Context
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.toggleable
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.ALLOWED_NOTIFY_POLL_MINUTES
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.notify.NotificationPermission
import com.ousiassllc.swarmviewer.ui.main.NoticeText
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton

/** 通知確認間隔の下に常に出す注記（機能要件 §12.4、FR-AND-7）。 */
const val DOZE_NOTE = "Doze・バッテリー最適化により、通知は設定した間隔より遅れることがあります"

/** 通知の権限が許可されていないときの案内。 */
const val PERMISSION_GUIDE = "通知が許可されていないため、通知は表示されません。端末の通知設定で許可してください。"

/**
 * 設定画面の通知の項目（機能要件 §12.4）。通知のオン / オフ、通知確認間隔（15 / 30 / 60 分）、Doze の注記、
 * 通知の権限の状態と案内（Android 13 以降の `POST_NOTIFICATIONS` が拒否されたときは、アプリの通知設定を開くボタン）。
 * オンにするときに、Android 13 以降で権限が無ければ要求する。
 */
@Composable
fun NotificationSettings(
    state: SettingsUiState,
    settings: Settings,
    viewModel: SettingsViewModel,
) {
    val context = LocalContext.current
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { viewModel.refresh() }
    SettingsSection("通知") {
        Row(
            modifier =
                Modifier.fillMaxWidth().toggleable(
                    value = settings.notificationsEnabled,
                    role = Role.Switch,
                    onValueChange = { enabled ->
                        viewModel.setNotificationsEnabled(enabled)
                        if (enabled &&
                            NotificationPermission.needsRuntimeRequest(context)
                        ) {
                            launcher.launch(Manifest.permission.POST_NOTIFICATIONS)
                        }
                    },
                ),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text("答え待ち・止まってるかもを通知する")
            Switch(checked = settings.notificationsEnabled, onCheckedChange = null)
        }
        if (state.showsPermissionGuide) PermissionGuide(context)
        Text("通知確認間隔（バックグラウンド）", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(top = 12.dp))
        ALLOWED_NOTIFY_POLL_MINUTES.forEach { minutes ->
            RadioRow("$minutes 分", settings.notifyPollMinutes == minutes) { viewModel.setNotifyPollMinutes(minutes) }
        }
        NoticeText(DOZE_NOTE)
    }
}

@Composable
private fun PermissionGuide(context: Context) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(PERMISSION_GUIDE, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
        AppOutlinedButton(
            onClick = { context.startActivity(NotificationPermission.appNotificationSettings(context)) },
        ) { Text("通知設定を開く") }
    }
}
