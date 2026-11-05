package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.ScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.material3.windowsizeclass.WindowWidthSizeClass
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.LifecycleStartEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.ousiassllc.swarmviewer.R
import com.ousiassllc.swarmviewer.data.store.Snapshot
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.service.ConnectionItem
import com.ousiassllc.swarmviewer.ui.NEEDS_REPAIR_LABEL
import com.ousiassllc.swarmviewer.ui.theme.AppAlert
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.AppEmptyState
import com.ousiassllc.swarmviewer.ui.theme.AppNavTextButton
import com.ousiassllc.swarmviewer.ui.theme.AppPullToRefreshIndicator
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter
import java.time.Instant

/**
 * メイン画面（A2。機能要件 §12.2）。接続名（複数あるときは切替）と設定、セッションのタブ、要対応・警告、ペイン、利用上限、ホスト負荷。
 * 操作系のコントロール（ペインへの送信・選択・起動・終了）を持たない。
 *
 * [widthClass] が Compact なら縦 1 列のリスト（1 つのスクロール）、Medium / Expanded なら左に `layout` のグリッド、右に利用上限・ホスト負荷。
 * 選択中のセッション・取得済みの表示は [MainViewModel] に、スクロール位置は `rememberScrollState`（`rememberSaveable`）に置き、
 * 折りたたみ⇔展開の切替（構成変更・区分の変化）で保たれる。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen(
    viewModel: MainViewModel,
    widthClass: WindowWidthSizeClass,
    onManageConnections: () -> Unit,
    onSettings: () -> Unit,
    onRepair: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    // 画面が見えている間（STARTED 以上）だけポーリングする。戻ったとき・画面に入ったときに即時取得する。
    LifecycleStartEffect(viewModel) {
        viewModel.onStart()
        onStopOrDispose { viewModel.onStop() }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { ConnectionTitle(state.connections, viewModel::selectConnection) },
                actions = {
                    AppNavTextButton("接続", onManageConnections)
                    AppNavTextButton("設定", onSettings)
                },
            )
        },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding)) {
            when {
                state.connections == null -> NoticeText("読み込み中…", modifier = Modifier.padding(LocalGutter.current))
                state.noConnections -> EmptyConnections(onManageConnections)
                else -> {
                    SessionTabs(state.sessions, state.session, viewModel::selectSession)
                    val pullState = rememberPullToRefreshState()
                    PullToRefreshBox(
                        isRefreshing = state.refreshing,
                        onRefresh = viewModel::refresh,
                        modifier = Modifier.fillMaxSize(),
                        state = pullState,
                        // 既定のインジケーターは影を付ける（ガイド S3: 影はモーダルだけ）ので、影なしの自前の部品に置き換える。
                        indicator = { AppPullToRefreshIndicator(state = pullState, isRefreshing = state.refreshing) },
                    ) {
                        MainContent(state, widthClass != WindowWidthSizeClass.Compact, onRepair)
                    }
                }
            }
        }
    }
}

@Composable
private fun EmptyConnections(onManageConnections: () -> Unit) {
    AppEmptyState(
        "接続がありません。接続文字列を貼り付けるか、QR を読み取って追加してください。",
        modifier = Modifier.padding(LocalGutter.current),
    ) {
        AppButton(onClick = onManageConnections) { Text("接続を追加") }
    }
}

/** アプリバーのタイトル。接続が複数あるときだけ切替（ドロップダウン）を出す。接続名は信頼できない入力としてテキストで描画する。 */
@Composable
private fun ConnectionTitle(
    connections: List<ConnectionItem>?,
    onSelect: (String) -> Unit,
) {
    val selected = connections?.firstOrNull { it.selected }
    if (connections == null || selected == null || connections.size < 2) {
        Text(selected?.name ?: "swarm-viewer", maxLines = 1)
        return
    }
    var open by remember { mutableStateOf(false) }
    TextButton(onClick = { open = true }) {
        Text(selected.name, style = MaterialTheme.typography.titleLarge, maxLines = 1)
        Spacer(Modifier.width(4.dp))
        // Iconoir の nav-arrow-down。色は周囲の文字色（LocalContentColor）に従う。
        Icon(painterResource(R.drawable.ic_nav_arrow_down), contentDescription = null)
    }
    DropdownMenu(
        expanded = open,
        onDismissRequest = { open = false },
        // ガイド S3・§6: 影はモーダルだけ。メニューは 1px の border（M3 の既定は影 3dp・枠なし）。
        shadowElevation = 0.dp,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        connections.forEach { item ->
            DropdownMenuItem(
                // labelLarge はボタン用の SemiBold。メニュー項目は Regular（T2）なので bodyMedium を明示する。
                text = { Text(item.name, style = MaterialTheme.typography.bodyMedium, maxLines = 1) },
                onClick = {
                    open = false
                    onSelect(item.id)
                },
            )
        }
    }
}

/** 許可セッションのタブ（横スクロール）。セッションが 1 つだけでもタブを出す。 */
@Composable
private fun SessionTabs(
    sessions: List<String>,
    selected: String?,
    onSelect: (String) -> Unit,
) {
    if (sessions.isEmpty()) return
    ScrollableTabRow(selectedTabIndex = sessions.indexOf(selected).coerceAtLeast(0), edgePadding = LocalGutter.current) {
        sessions.forEach { id ->
            Tab(
                selected = id == selected,
                onClick = { onSelect(id) },
                selectedContentColor = MaterialTheme.colorScheme.onSurface,
                unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant,
                text = { Text(id, style = MaterialTheme.typography.bodyMedium, maxLines = 1) },
            )
        }
    }
}

@Composable
private fun MainContent(
    state: MainUiState,
    wide: Boolean,
    onRepair: () -> Unit,
) {
    val snapshot = state.snapshot
    val report = snapshot?.report ?: Report()
    val now = remember(snapshot) { Instant.now() }
    val primaryScroll = rememberScrollState()
    val secondaryScroll = rememberScrollState()
    val gap = Arrangement.spacedBy(12.dp)
    val gutter = LocalGutter.current
    val padding = Modifier.padding(horizontal = gutter, vertical = 8.dp)

    val status: @Composable () -> Unit = { StatusLine(state, onRepair) }
    val panes: @Composable () -> Unit = {
        when {
            snapshot != null -> {
                AttentionBar(report.attention, report.warnings, report.auth, state.stale)
                PaneSection(report.panes, report.layout, report.attention, state.stale, wide)
            }
            state.session == null && state.sessionsLoaded && state.failure == null -> AppEmptyState("許可されたセッションがありません")
            state.refreshing || state.failure == null -> NoticeText("取得中…")
            else -> Unit
        }
    }
    val panels: @Composable () -> Unit = {
        if (snapshot != null) {
            UsagePanel(report.usage, state.stale, now)
            HostPanel(report.host, state.stale)
        }
    }

    if (wide) {
        Row(modifier = Modifier.fillMaxSize().then(padding), horizontalArrangement = Arrangement.spacedBy(gutter)) {
            Column(modifier = Modifier.weight(2f).verticalScroll(primaryScroll), verticalArrangement = gap) {
                status()
                panes()
            }
            Column(modifier = Modifier.weight(1f).verticalScroll(secondaryScroll), verticalArrangement = gap) { panels() }
        }
    } else {
        Column(modifier = Modifier.fillMaxSize().verticalScroll(primaryScroll).then(padding), verticalArrangement = gap) {
            status()
            panes()
            panels()
        }
    }
}

/** 取得の状態。再ペアリングが必要なときは導線、失敗は固定文言、保存済み・失敗後の内容は「前回取得 HH:MM」、成功後は「最終更新 HH:MM」。 */
@Composable
private fun StatusLine(
    state: MainUiState,
    onRepair: () -> Unit,
) {
    val snapshot: Snapshot? = state.snapshot
    Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (state.needsRepair) {
            AppAlert(NEEDS_REPAIR_LABEL) {
                AppButton(onClick = onRepair) { Text("再ペアリング") }
            }
        }
        val fetched =
            snapshot?.let { "${if (state.showsPreviousFetch) "前回取得" else "最終更新"} ${formatClock(it.fetchedAt)}" }
        // 失敗は見出し、前回取得の時刻は本文（ガイド §4 のインラインアラート）。失敗がなければ時刻だけを控えめに出す。
        val failure = state.failure
        if (failure != null) {
            AppAlert(fetched.orEmpty(), title = failure)
        } else if (fetched != null) {
            MutedText(fetched)
        }
    }
}
