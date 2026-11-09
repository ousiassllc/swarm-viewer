package com.ousiassllc.swarmviewer.ui.pane

import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.LifecycleStartEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.ui.NEEDS_REPAIR_LABEL
import com.ousiassllc.swarmviewer.ui.PROMPT_VIEW_ONLY_HINT
import com.ousiassllc.swarmviewer.ui.main.Badge
import com.ousiassllc.swarmviewer.ui.main.DisplayTone
import com.ousiassllc.swarmviewer.ui.main.KindBadge
import com.ousiassllc.swarmviewer.ui.main.MutedText
import com.ousiassllc.swarmviewer.ui.main.NoticeText
import com.ousiassllc.swarmviewer.ui.main.stateTone
import com.ousiassllc.swarmviewer.ui.theme.AlertKind
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppNavTextButton
import com.ousiassllc.swarmviewer.ui.theme.AppOutlinedButton
import com.ousiassllc.swarmviewer.ui.theme.ConsoleFontFamily
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter
import com.ousiassllc.swarmviewer.ui.theme.MonoFontFamily
import kotlinx.coroutines.delay

/** 本文の最大幅。Medium 以上（折りたたみの開き）では、この幅で中央に寄せる。Compact は画面幅いっぱいの 1 列。 */
val ContentMaxWidth = 640.dp

private const val NOTICE_MILLIS = 4_000L

/**
 * ペイン詳細（機能要件 §4.7）。ヘッダ（ペイン位置・状態・種類・タイトル・cwd・メモ・エージェント）、質問カード、`pane_tail`（等幅・テキスト）、
 * 送信欄、中断・終了。操作の部品は [PaneControls] が決める（見るだけの接続では何も出ない）。
 * 開いている間だけ（STARTED 以上）5 秒ごとに更新する（[LifecycleStartEffect]）。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaneDetailScreen(
    viewModel: PaneDetailViewModel,
    target: PaneTarget,
    onBack: () -> Unit,
    onStartSession: () -> Unit,
    onRepair: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(target) { viewModel.open(target) }
    LifecycleStartEffect(viewModel) {
        viewModel.onStart()
        onStopOrDispose { viewModel.onStop() }
    }
    LaunchedEffect(Unit) { viewModel.leave.collect { onBack() } }
    LaunchedEffect(state.notice) {
        if (state.notice != null) {
            delay(NOTICE_MILLIS)
            viewModel.noticeShown()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("ペイン ${target.pos}", fontFamily = MonoFontFamily, maxLines = 1) },
                navigationIcon = { AppNavTextButton("戻る", onBack) },
            )
        },
    ) { padding ->
        Box(modifier = Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.TopCenter) {
            Column(
                modifier =
                    Modifier
                        .widthIn(max = ContentMaxWidth)
                        .fillMaxSize()
                        .imePadding()
                        .verticalScroll(rememberScrollState())
                        .padding(LocalGutter.current),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                PaneDetailBody(state, viewModel, onStartSession, onRepair)
            }
        }
    }

    state.pendingConfirm?.let { PaneConfirmDialog(it, onConfirm = viewModel::confirmPending, onDismiss = viewModel::dismissConfirm) }
}

@Composable
private fun PaneDetailBody(
    state: PaneDetailUiState,
    viewModel: PaneDetailViewModel,
    onStartSession: () -> Unit,
    onRepair: () -> Unit,
) {
    val controls = state.controls
    val pane = state.pane
    if (state.needsRepair) {
        AppAlert(NEEDS_REPAIR_LABEL) { AppButton(onClick = onRepair, modifier = Modifier.bigButton()) { Text("再ペアリング") } }
    }
    state.failure?.let { AppAlert(it) }
    state.notice?.let { notice ->
        AppAlert(notice.message, kind = if (notice.error) AlertKind.Danger else AlertKind.Success)
        if (state.retryClose) {
            AppOutlinedButton(onClick = viewModel::requestClose, enabled = !state.busy, modifier = Modifier.bigButton()) { Text("もう一度試す") }
        }
    }
    if (pane == null) {
        if (!state.loaded && state.failure == null) NoticeText("取得中…")
        return
    }
    PaneHeader(pane, state)
    state.card?.let { card ->
        AskCardSection(
            card = card,
            controls = controls,
            busy = state.busy,
            customOpen = state.customOpen,
            customText = state.customText,
            onPick = viewModel::pick,
            onOpenCustom = viewModel::openCustom,
            onCustomTextChange = viewModel::onCustomTextChange,
            onSendCustom = viewModel::pickCustom,
        )
    }
    if (controls.showStart) {
        AppButton(onClick = onStartSession, enabled = !state.busy, modifier = Modifier.bigButton()) { Text("＋ 始める") }
    }
    TailSection(state, viewModel)
    SendSection(controls, state.busy, state.sendText, viewModel::onSendTextChange, viewModel::send)
    if (controls.showViewOnlyPromptHint) MutedText(PROMPT_VIEW_ONLY_HINT)
    EscapeCloseSection(controls, state.busy, viewModel::requestEscape, viewModel::requestClose)
}

/** ヘッダ: ペイン位置・状態・種類のバッジ、エージェント、タイトル・cwd・メモ（`repo`・`items`）。すべてテキストとして描画する。 */
@Composable
private fun PaneHeader(
    pane: Pane,
    state: PaneDetailUiState,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(pane.pos, style = MaterialTheme.typography.bodyMedium, fontFamily = MonoFontFamily)
            pane.agent?.takeIf { it.isNotEmpty() }?.let { Badge(it, DisplayTone.Muted) }
            Badge(pane.state.label, stateTone(pane.state), marker = true)
            KindBadge(state.kind)
        }
        if (state.connectionName.isNotEmpty()) MutedText(state.connectionName)
        Text(pane.title, style = MaterialTheme.typography.bodyLarge)
        if (pane.cwd.isNotEmpty()) MutedText(pane.cwd)
        pane.note?.let { note ->
            if (note.repo.isNotEmpty()) MutedText(note.repo)
            note.items.forEach { Text("・$it", style = MaterialTheme.typography.bodyMedium) }
        }
    }
}

/** `pane_tail` の本文（等幅・テキスト。HTML として解釈しない。Nerd Font のアイコンも字形で出す）と、更新・「もっと見る」。 */
@Composable
private fun TailSection(
    state: PaneDetailUiState,
    viewModel: PaneDetailViewModel,
) {
    val tail = state.tail
    if (tail != null) {
        Box(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = 96.dp)
                    .border(1.dp, MaterialTheme.colorScheme.outline, MaterialTheme.shapes.medium)
                    .padding(12.dp)
                    .horizontalScroll(rememberScrollState()),
        ) {
            Text(tail.text, style = MaterialTheme.typography.bodyMedium, fontFamily = ConsoleFontFamily, softWrap = false)
        }
        if (tail.truncated) MutedText("末尾 ${state.lines} 行のみ表示しています")
    }
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        AppOutlinedButton(onClick = viewModel::refresh, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text("更新") }
        if (state.canShowMore) {
            AppOutlinedButton(onClick = viewModel::showMore, modifier = Modifier.heightIn(min = TouchTargetHeight)) { Text("もっと見る") }
        }
    }
}
