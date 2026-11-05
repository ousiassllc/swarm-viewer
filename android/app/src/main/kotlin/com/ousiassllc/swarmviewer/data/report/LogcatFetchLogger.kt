package com.ousiassllc.swarmviewer.data.report

import android.util.Log

/** [FetchLogger] の Logcat 実装。接続 ID・ツール名・種別・コードだけを出す（URL・token・本文は出さない。NFR-AND-LOG-1）。 */
object LogcatFetchLogger : FetchLogger {
    private const val TAG = "swarm-viewer"

    override fun failed(
        connectionId: String?,
        tool: String,
        kind: String,
        code: String,
    ) {
        Log.w(TAG, "swarm-console request failed: connection=${connectionId.orEmpty()} tool=$tool kind=$kind code=$code")
    }
}
