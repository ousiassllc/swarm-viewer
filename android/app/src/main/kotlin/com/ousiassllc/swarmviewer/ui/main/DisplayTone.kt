package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Tone

/**
 * 表示色の意味（デスクトップ版の internal/web/format/tone.go と同じ対応）。色だけに依存せず、必ずラベルを併記する（機能要件 §3）。
 * しきい値の判定は domain（[com.ousiassllc.swarmviewer.domain.percentTone]・[com.ousiassllc.swarmviewer.domain.usageTone]）が行い、
 * ここは色への対応だけを持つ。
 */
enum class DisplayTone { Accent, Warning, Success, Error, Muted }

/** domain の使用率の色（normal / warning / error）を表示色に対応づける。normal は accent。 */
fun displayTone(tone: Tone): DisplayTone =
    when (tone) {
        Tone.Normal -> DisplayTone.Accent
        Tone.Warning -> DisplayTone.Warning
        Tone.Error -> DisplayTone.Error
    }

/** ペイン状態の表示色: working=accent、asking=warning、idle=success、stalled=error、shell・unknown=muted。 */
fun stateTone(state: PaneState): DisplayTone =
    when (state) {
        PaneState.Working -> DisplayTone.Accent
        PaneState.Asking -> DisplayTone.Warning
        PaneState.Idle -> DisplayTone.Success
        PaneState.Stalled -> DisplayTone.Error
        PaneState.Shell, PaneState.Unknown -> DisplayTone.Muted
    }

/** `attention.kind` の表示色。未知の kind は muted。 */
fun attentionTone(kind: String): DisplayTone =
    when (kind) {
        "asking", "auth_expiring" -> DisplayTone.Warning
        "stalled", "auth_expired" -> DisplayTone.Error
        "report_waiting" -> DisplayTone.Accent
        else -> DisplayTone.Muted
    }

private val warningColor = Color(0xFFF59E0B)
private val successColor = Color(0xFF10B981)

/** 表示色の実際の色（テーマに合わせる）。 */
@Composable
fun DisplayTone.color(): Color =
    when (this) {
        DisplayTone.Accent -> MaterialTheme.colorScheme.primary
        DisplayTone.Warning -> warningColor
        DisplayTone.Success -> successColor
        DisplayTone.Error -> MaterialTheme.colorScheme.error
        DisplayTone.Muted -> MaterialTheme.colorScheme.onSurfaceVariant
    }

/** 値（数値・金額）の文字色。しきい値に達していない（accent）ときは本文の色にする。 */
@Composable
fun DisplayTone.valueColor(): Color = if (this == DisplayTone.Accent) MaterialTheme.colorScheme.onSurface else color()
