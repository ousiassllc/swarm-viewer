package com.ousiassllc.swarmviewer.data.report

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.InvalidUrlException
import com.ousiassllc.swarmviewer.domain.classify
import com.ousiassllc.swarmviewer.domain.connectionKindOf
import com.ousiassllc.swarmviewer.domain.validateUrl
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import okhttp3.OkHttpClient
import java.io.IOException
import java.util.concurrent.TimeUnit

/**
 * ツールを 1 回呼ぶ通信（[ReportClient] の下回り）。送信前に URL の path から決めた種類の許可リストでツール名を検査し
 * （[ReportTools.allowedFor]）、送信内容は [GuardInterceptor] が別途検査する（二重の検査。NFR-SEC-1）。
 */
internal class ToolTransport(
    private val http: OkHttpClient,
    private val timeoutMillis: Long,
) {
    /** セッションを開き、ツールを 1 回呼び、閉じる。応答の先頭のテキストを返す。許可リスト外のツールは通信せず `tool_not_allowed`。 */
    suspend fun call(
        rawUrl: String,
        token: () -> String?,
        tool: String,
        args: JsonObject,
    ): String {
        if (tool !in ReportTools.allowedFor(connectionKindOf(rawUrl))) throw FetchException(ErrKind.Protocol, "tool_not_allowed")
        val endpoint =
            try {
                validateUrl(rawUrl)
            } catch (e: InvalidUrlException) {
                throw FetchException(ErrKind.Protocol, "invalid_url")
            }
        // 保護領域（ファイル・Keystore）の読み出しは呼び出し元のスレッドで行わない。
        val bearer =
            try {
                withContext(Dispatchers.IO) { token() }
            } catch (e: IOException) {
                throw FetchException(ErrKind.Unreachable, "secret_store") // 保護領域を読めない（原因・パスは出さない）
            } ?: throw FetchException(ErrKind.Unauthorized, "token_missing")
        // 呼び出し全体の期限と、OkHttp の 1 回の読み取りの期限（既定 10 秒）を、ツール別の倍率に合わせる。
        val limit = timeoutMillis * ReportTools.timeoutScale(tool)
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(limit)
        val client =
            http
                .newBuilder()
                .readTimeout(limit, TimeUnit.MILLISECONDS)
                .addInterceptor(GuardInterceptor(endpoint, bearer))
                .build()
        try {
            return McpExchange(client, endpoint, deadline).callTool(tool, args)
        } catch (e: IOException) {
            currentCoroutineContext().ensureActive() // キャンセルによる失敗は分類せず、キャンセルとして伝える
            throw if (e is RequestRejectedException) {
                FetchException(ErrKind.Protocol, "request_rejected")
            } else {
                classify(e) ?: FetchException(ErrKind.Unreachable)
            }
        }
    }
}
