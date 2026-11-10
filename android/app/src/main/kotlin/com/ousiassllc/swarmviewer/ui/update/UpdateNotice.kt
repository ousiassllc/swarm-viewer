package com.ousiassllc.swarmviewer.ui.update

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.toggleable
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.service.UpdateState
import com.ousiassllc.swarmviewer.ui.main.NoticeText
import com.ousiassllc.swarmviewer.ui.settings.SettingsSection
import com.ousiassllc.swarmviewer.ui.theme.AlertKind
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppSwitch
import com.ousiassllc.swarmviewer.ui.theme.ListRowMinHeight
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter

private val ProgressHeight = 6.dp

/**
 * 新しい版の通知（メイン画面の上部。#109）。「新しい版 vX.Y.Z があります」と「更新する」、更新中は「更新中…」（ボタンは押せない）と進行表示。
 * 失敗は 1 行の小さな文言だけで、ペイン一覧・操作は止めない。新しい版が無いときは、確認の失敗だけを 1 行で出す（何もなければ何も出さない）。
 */
@Composable
fun UpdateNotice(viewModel: UpdateViewModel) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val release = state.release
    val message = state.message()
    if (release == null && message == null) return
    Column(
        modifier = Modifier.fillMaxWidth().padding(horizontal = LocalGutter.current).padding(top = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        if (release != null) {
            AppAlert("新しい版 v${release.version} があります", kind = AlertKind.Info) {
                AppButton(onClick = viewModel::update, enabled = !state.updating) { Text(if (state.updating) "更新中…" else "更新する") }
                if (state.updating) {
                    LinearProgressIndicator(
                        color = MaterialTheme.colorScheme.primary,
                        trackColor = MaterialTheme.colorScheme.surfaceVariant,
                        strokeCap = StrokeCap.Round,
                        gapSize = 0.dp,
                        modifier = Modifier.fillMaxWidth().height(ProgressHeight),
                    )
                }
            }
        }
        message?.let { NoticeText(it) }
    }
}

private fun UpdateState.message(): String? =
    when {
        needsInstallPermission -> INSTALL_PERMISSION_GUIDE
        failure != null -> updateFailureMessage(failure)
        else -> null
    }

/** 設定画面の「アップデート」の項目（#109）。新しい版を自動で確認するかのスイッチ（既定はオン。オフの間は確認の通信をしない）。 */
@Composable
fun UpdateSettings(viewModel: UpdateViewModel) {
    val enabled by viewModel.autoCheckEnabled.collectAsStateWithLifecycle()
    SettingsSection("アップデート") {
        Row(
            modifier =
                Modifier.fillMaxWidth().heightIn(min = ListRowMinHeight).toggleable(
                    value = enabled,
                    role = Role.Switch,
                    onValueChange = viewModel::setAutoCheck,
                ),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text("自動で確認する")
            AppSwitch(checked = enabled)
        }
        NoticeText("起動時と 6 時間ごとに、新しい版を GitHub で確認します。")
    }
}
