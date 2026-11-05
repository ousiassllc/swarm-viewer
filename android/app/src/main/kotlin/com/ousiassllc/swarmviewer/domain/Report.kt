package com.ousiassllc.swarmviewer.domain

import java.time.Instant

/**
 * ペインの状態（swarm-console の `panes[].state`）と表示ラベル（機能要件 §3）。
 * 未知の値は [Unknown] に正規化する（NFR-CMP-1）。
 */
enum class PaneState(
    val id: String,
    val label: String,
) {
    Working("working", "作業中"),
    Asking("asking", "答え待ち"),
    Idle("idle", "指示待ち"),
    Stalled("stalled", "止まってるかも"),
    Shell("shell", "空き"),
    Unknown("unknown", "判定できない"),
    ;

    /** 通知対象の状態（asking / stalled）か。 */
    val notifiable: Boolean get() = this == Asking || this == Stalled

    companion object {
        /** 文字列を [PaneState] に変換する。既知の 6 値以外（null・空文字を含む）は [Unknown]。 */
        fun parse(value: String?): PaneState = entries.firstOrNull { it.id == value } ?: Unknown
    }
}

/** ペインのメモ要約。viewer は [repo] と [items] だけ表示する。 */
data class Note(
    val from: String = "",
    val repo: String = "",
    val items: List<String> = emptyList(),
    val next: String = "",
)

/** ペイン 1 つの状態。null・欠落の文字列項目は空文字になる。 */
data class Pane(
    val pos: String = "",
    /** tmux ペイン ID（例 `%12`）。通知の同一性判定に使う。 */
    val id: String = "",
    val agent: String? = null,
    val state: PaneState = PaneState.Unknown,
    val title: String = "",
    val cwd: String = "",
    val elapsed: String = "",
    val activity: String = "",
    val activityTruncated: Boolean = false,
    val changedAgo: String = "",
    val note: Note? = null,
)

/** ペイングリッドの形。[cols] は各段の列数。 */
data class Layout(
    val rows: Int = 0,
    val cols: List<Int> = emptyList(),
) {
    /**
     * [paneCount] 個のペインがこのレイアウトのグリッドとして成立するか。
     * 段数が 1 以上、[cols] の長さが [rows] と一致、各段の列数が 1 以上、列数の合計が [paneCount] と一致するとき true。
     */
    fun valid(paneCount: Int): Boolean = rows >= 1 && cols.size == rows && cols.all { it >= 1 } && cols.sum() == paneCount
}

/** 要対応の 1 件。[kind] は未知の値もそのまま保持する。 */
data class Attention(
    val kind: String = "",
    val pane: String = "",
    val detail: String = "",
)

/** 警告 1 件。 */
data class Warning(
    val code: String = "",
    val message: String = "",
)

/** `host.cpu`。[available] が false のとき [usedPercent] は 0。 */
data class Cpu(
    val available: Boolean = false,
    val usedPercent: Double = 0.0,
)

/** `host.mem`・`host.swap`。[totalGb] が 0 でも [available] ならスワップ無し。`host.mem` を返さない古い swarm-console では既定値。 */
data class MemUsage(
    val available: Boolean = false,
    val totalGb: Double = 0.0,
    val usedGb: Double = 0.0,
    val usedPercent: Double = 0.0,
)

/** `host.disk[]` の 1 マウント。 */
data class Disk(
    val mount: String = "",
    val available: Boolean = false,
    val freeGb: Double = 0.0,
    val totalGb: Double = 0.0,
    val usedPercent: Double = 0.0,
)

/** ホスト全体の負荷。`load1`・`mem_free_gb` は使わない（メモリは [mem] の使用率を使う）。 */
data class Host(
    val cpu: Cpu = Cpu(),
    val mem: MemUsage = MemUsage(),
    val swap: MemUsage = MemUsage(),
    val disk: List<Disk> = emptyList(),
)

/** `auth.agents[]` の 1 件。[expiresAt] は不明のとき null。 */
data class AuthAgent(
    val agent: String = "",
    val status: String = "",
    val expiresAt: Instant? = null,
)

/** `report` の `auth`。 */
data class Auth(
    val agents: List<AuthAgent> = emptyList(),
)

/** swarm-console `report`（format: json）から viewer が使う項目だけを持つ。欠落は既定値（NFR-CMP-2）。 */
data class Report(
    val at: Instant? = null,
    val layout: Layout = Layout(),
    val panes: List<Pane> = emptyList(),
    val attention: List<Attention> = emptyList(),
    val usage: Usage = Usage(),
    val host: Host = Host(),
    val warnings: List<Warning> = emptyList(),
    val auth: Auth = Auth(),
)
