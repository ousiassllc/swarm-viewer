package com.ousiassllc.swarmviewer.data.report

/**
 * 呼んでよいツールの許可リスト（API 仕様 §1.2、NFR-SEC-1）。ツール名は定数で、外部から渡す経路を公開しない。
 * `/ops` 専用ツールや `tools/list` を呼ぶコードパスは存在しない。
 * [ReportClient] が送信前に、[GuardInterceptor] が送信内容に対して二重に検査する。
 */
internal object ReportTools {
    const val SESSIONS_LIST = "sessions_list"
    const val REPORT = "report"

    val allowed: Set<String> = setOf(SESSIONS_LIST, REPORT)

    /** 送ってよい JSON-RPC メソッド。`tools/list` など他のメソッドは通さない。`tools/call` は更にツール名を [allowed] で絞る。 */
    const val RPC_INITIALIZE = "initialize"
    const val RPC_INITIALIZED = "notifications/initialized"
    const val RPC_TOOLS_CALL = "tools/call"

    val allowedRpcMethods: Set<String> = setOf(RPC_INITIALIZE, RPC_INITIALIZED, RPC_TOOLS_CALL)
}
