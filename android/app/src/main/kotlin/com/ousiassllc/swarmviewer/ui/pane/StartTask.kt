package com.ousiassllc.swarmviewer.ui.pane

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.ousiassllc.swarmviewer.domain.MAX_SEND_CHARS
import com.ousiassllc.swarmviewer.domain.Skill
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.operationErrorMessage
import com.ousiassllc.swarmviewer.ui.theme.AlertKind
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppCard
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.AppRadio
import com.ousiassllc.swarmviewer.ui.theme.AppTextField
import com.ousiassllc.swarmviewer.ui.theme.ListRowMinHeight
import com.ousiassllc.swarmviewer.ui.theme.extraColors

// 新しいセッションの最初のタスク（機能要件 §4.8・FR-OPS-11・FR-AND-12）の部品。操作の接続だけで描画する
// （見るだけの接続は StartSessionScreen がフォームごと描画しない）。スキル名・説明・本文はテキストとして描画する。

private const val NO_SKILL_LABEL = "スキルなし"

/**
 * スキルの選択（任意）。エージェントを選ぶまでは何も出さない。取得に失敗しても（古いホストを含む）固定文言を出すだけで、
 * 「スキルなし」で本文だけ・タスクなしの起動は続けられる。各行は最小 48dp（[ListRowMinHeight]）。
 */
@Composable
fun SkillPicker(
    state: StartSessionUiState,
    viewModel: StartSessionViewModel,
) {
    if (state.agent == null) return
    Text("スキル（任意）", style = MaterialTheme.typography.titleSmall)
    when (val listing = state.skills) {
        SkillListing.None -> Unit
        SkillListing.Loading -> MutedText("読み込み中…")
        is SkillListing.Failed -> {
            MutedText(listing.message)
            if (listing.reload) {
                AppOutlinedButton(
                    onClick = viewModel::retrySkills,
                    enabled = !state.busy,
                    modifier = Modifier.bigButton(),
                ) { Text("再読み込み") }
            }
        }
        is SkillListing.Loaded -> if (listing.truncated) MutedText("一部のみ表示しています")
    }
    if (state.skills != SkillListing.Loading) {
        SkillRows((state.skills as? SkillListing.Loaded)?.skills.orEmpty(), state.skill, !state.busy, viewModel::selectSkill)
    }
}

@Composable
private fun SkillRows(
    skills: List<Skill>,
    selected: Skill?,
    enabled: Boolean,
    onSelect: (Skill?) -> Unit,
) {
    AppCard(modifier = Modifier.fillMaxWidth()) {
        Column {
            SkillRow(NO_SKILL_LABEL, null, selected == null, enabled) { onSelect(null) }
            skills.forEach { skill ->
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                SkillRow(skill.name, skill.description, selected == skill, enabled) { onSelect(skill) }
            }
        }
    }
}

@Composable
private fun SkillRow(
    name: String,
    description: String?,
    selected: Boolean,
    enabled: Boolean,
    onSelect: () -> Unit,
) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .heightIn(min = ListRowMinHeight)
                .selectable(selected = selected, enabled = enabled, onClick = onSelect, role = Role.RadioButton),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AppRadio(selected = selected)
        Column(modifier = Modifier.padding(vertical = 8.dp)) {
            Text(name, maxLines = 1)
            if (!description.isNullOrEmpty()) MutedText(description)
        }
    }
}

/** 本文の入力欄（複数行・任意）。検査（8000 字・制御文字）に違反したら、送る前に固定文言を入力欄の下に出す。 */
@Composable
fun TaskField(
    state: StartSessionUiState,
    viewModel: StartSessionViewModel,
) {
    Text("本文（任意）", style = MaterialTheme.typography.titleSmall)
    val problem = state.taskProblem
    AppTextField(
        value = state.task,
        onValueChange = viewModel::onTaskChange,
        label = "最初のタスクの本文",
        enabled = !state.busy,
        isError = problem != null,
        modifier = Modifier.fillMaxWidth(),
    )
    if (problem != null) MutedText(operationErrorMessage(problem), color = MaterialTheme.extraColors.danger)
    state.firstTask.text.let { MutedText("${it.codePointCount(0, it.length)} / $MAX_SEND_CHARS") }
}

/** 起動に成功したが最初のタスクを送っていない・送れなかったときの結果の表示。起動は取り消さない。 */
@Composable
fun FirstTaskNoticeCard(
    notice: FirstTaskNotice,
    busy: Boolean,
    viewModel: StartSessionViewModel,
) {
    AppAlert(notice.message, kind = AlertKind.Warning) {
        notice.reason?.let { Text(it) }
        Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(top = 8.dp)) {
            if (notice.canResend) {
                AppButton(onClick = viewModel::resend, enabled = !busy, modifier = Modifier.bigButton()) { Text("もう一度送る") }
            }
            AppOutlinedButton(onClick = viewModel::openPane, enabled = !busy, modifier = Modifier.bigButton()) { Text("ペイン詳細を開く") }
        }
    }
}
