package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ConnectionKind

/**
 * 呼んでよいツールの許可リスト（API 仕様 §1.2、NFR-SEC-1）。ツール名は定数で、外部から渡す経路を公開しない。
 * 接続の種類（URL の path。[ConnectionKind]）ごとに別のリストを持つ。範囲外のツール（`dispatch`・`note_set`・`note_log`・`file_get`・
 * `panes_status`・`host_status`・`report_read`・`tools/list`）を呼ぶコードパスは存在しない。
 * [ReportClient] が送信前に、[GuardInterceptor] が送信内容に対して二重に検査する。
 */
internal object ReportTools {
    const val SESSIONS_LIST = "sessions_list"
    const val REPORT = "report"
    const val PANE_TAIL = "pane_tail"
    const val ASK_CARD = "ask_card"
    const val ASK_PICK = "ask_pick"
    const val SEND_TEXT = "send_text"
    const val SEND_ESCAPE = "send_escape"
    const val PANE_START = "pane_start"
    const val PANE_CLOSE = "pane_close"
    const val DIR_LIST = "dir_list"

    /** 見るだけ（`/report`）の接続で呼べるツール。 */
    val view: Set<String> = setOf(SESSIONS_LIST, REPORT, PANE_TAIL, ASK_CARD)

    /** 操作専用（`/ops` の接続だけで呼べる）ツール。 */
    val opsOnly: Set<String> = setOf(ASK_PICK, SEND_TEXT, SEND_ESCAPE, PANE_START, PANE_CLOSE, DIR_LIST)

    /** 操作（`/ops`）の接続で呼べるツール（見るだけの 4 つ + 操作専用の 6 つ）。 */
    val ops: Set<String> = view + opsOnly

    /**
     * 1 回の呼び出しの上限（`REQUEST_TIMEOUT_MILLIS`）に掛ける倍率。操作ツールは画面の確認を待つため長い
     * （swarm-console の既定: accept_timeout 10 秒・start_timeout 30 秒。pane_start・pane_close は複数の手順を踏む）。
     * 載っていないツール（読み取り）は 1 倍。デスクトップ版の `toolTimeoutScale` と同じ。
     */
    fun timeoutScale(tool: String): Int =
        when (tool) {
            ASK_PICK, SEND_TEXT, SEND_ESCAPE -> 3
            PANE_CLOSE, PANE_START -> 5
            else -> 1
        }

    /** [kind] の接続で呼べるツール。 */
    fun allowedFor(kind: ConnectionKind): Set<String> =
        when (kind) {
            ConnectionKind.View -> view
            ConnectionKind.Ops -> ops
        }

    /** 送ってよい JSON-RPC メソッド。`tools/list` など他のメソッドは通さない。`tools/call` は更にツール名を [allowedFor] で絞る。 */
    const val RPC_INITIALIZE = "initialize"
    const val RPC_INITIALIZED = "notifications/initialized"
    const val RPC_TOOLS_CALL = "tools/call"

    val allowedRpcMethods: Set<String> = setOf(RPC_INITIALIZE, RPC_INITIALIZED, RPC_TOOLS_CALL)
}
