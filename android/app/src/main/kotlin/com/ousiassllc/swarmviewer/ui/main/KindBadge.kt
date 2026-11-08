package com.ousiassllc.swarmviewer.ui.main

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.ousiassllc.swarmviewer.domain.ConnectionKind

/** 接続の種類の表示名（バッジの文言）。 */
fun kindLabel(kind: ConnectionKind): String =
    when (kind) {
        ConnectionKind.Ops -> "操作"
        ConnectionKind.View -> "見るだけ"
    }

/**
 * 接続の種類のバッジ（接続一覧・メイン・ペイン詳細のヘッダに出す）。色だけに頼らず、「操作」「見るだけ」の文言と点で区別する。
 * 操作は注意の色（ペインを動かせる接続）、見るだけは中立。
 */
@Composable
fun KindBadge(
    kind: ConnectionKind,
    modifier: Modifier = Modifier,
) {
    Badge(kindLabel(kind), if (kind == ConnectionKind.Ops) DisplayTone.Warning else DisplayTone.Muted, modifier, marker = true)
}
