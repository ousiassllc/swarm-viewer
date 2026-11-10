package com.ousiassllc.swarmviewer.data.update

import com.ousiassllc.swarmviewer.data.report.executeAndRead
import com.ousiassllc.swarmviewer.domain.Release
import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import com.ousiassllc.swarmviewer.domain.parseLatestRelease
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import java.io.File
import java.io.IOException

private const val MAX_REDIRECTS = 5
private const val MAX_TEXT_BYTES = 1L shl 20
private const val MAX_DOWNLOAD_BYTES = 200L shl 20
private const val BUFFER_BYTES = 64 * 1024
private const val USER_AGENT = "swarm-viewer-android"

/** 1 回の取得の結果: リダイレクト先か、読み終えた値。 */
private sealed interface Hop<out T> {
    /** [location] は Location ヘッダー（無ければ null）。 */
    data class Redirect(
        val location: String?,
        val base: HttpUrl,
    ) : Hop<Nothing>

    data class Done<T>(
        val value: T,
    ) : Hop<T>
}

/**
 * GitHub Releases から取得する [ReleaseSource]（#109）。通信は [http]（[newUpdateHttpClient]。tailnet 用とは別のクライアント）だけで、
 * 認証情報は送らない。リダイレクトは自分で追い、1 回ごとに [UpdateEndpoints] の許可リスト（https・GitHub のホスト）を検査する
 * （許可外への経路は、接続を開く前に止まる）。
 *
 * 失敗の分類: 許可外の宛先 → [UpdateFailure.Rejected]、通信・HTTP の失敗 → [UpdateFailure.Network]、ダウンロードの途中の読み取りエラー・
 * Content-Length に足りない・上限超え → [UpdateFailure.Truncated]、書き込みエラー → [UpdateFailure.Storage]。
 */
class GithubReleaseSource internal constructor(
    private val http: OkHttpClient,
    private val endpoints: UpdateEndpoints,
) : ReleaseSource {
    constructor(endpoints: UpdateEndpoints) : this(newUpdateHttpClient(endpoints), endpoints)

    override suspend fun latest(): Release? = parseLatestRelease(readText(endpoints.latestUrl, "application/vnd.github+json"))

    override suspend fun text(url: String): String = readText(parse(url), "text/plain, application/octet-stream")

    override suspend fun download(
        url: String,
        dest: File,
    ) {
        try {
            fetch(parse(url), "application/octet-stream") { response -> save(response, dest) }
        } catch (e: Throwable) {
            dest.delete()
            throw e
        }
    }

    private suspend fun readText(
        url: HttpUrl,
        accept: String,
    ): String =
        fetch(url, accept) { response ->
            val source = response.body.source()
            source.request(MAX_TEXT_BYTES + 1)
            if (source.buffer.size > MAX_TEXT_BYTES) throw UpdateException(UpdateFailure.Network)
            source.readUtf8()
        }

    private fun parse(url: String): HttpUrl =
        url.toHttpUrlOrNull()?.takeIf(endpoints::permits) ?: throw UpdateException(UpdateFailure.Rejected)

    /** [url] へ GET し、リダイレクトは 1 回ごとに許可リストを検査して [MAX_REDIRECTS] 回まで追う。200 の応答を [read] で読む。 */
    private suspend fun <T> fetch(
        url: HttpUrl,
        accept: String,
        read: (Response) -> T,
    ): T {
        var current = url
        repeat(MAX_REDIRECTS + 1) {
            val request =
                Request
                    .Builder()
                    .url(current)
                    .header("Accept", accept)
                    .header("Accept-Encoding", "identity") // 圧縮を外し、Content-Length で途中切れを見つけられるようにする
                    .header("User-Agent", USER_AGENT)
                    .build()
            val hop =
                try {
                    http.newCall(request).executeAndRead { response ->
                        when {
                            response.isRedirect -> Hop.Redirect(response.header("Location"), response.request.url)
                            response.code == HTTP_OK -> Hop.Done(read(response))
                            else -> throw UpdateException(UpdateFailure.Network) // 本文は見ない
                        }
                    }
                } catch (e: IOException) {
                    currentCoroutineContext().ensureActive() // キャンセルによる失敗は分類せず、キャンセルとして伝える
                    throw UpdateException(if (e is UpdateRejectedException) UpdateFailure.Rejected else UpdateFailure.Network)
                }
            when (hop) {
                is Hop.Done -> return hop.value
                is Hop.Redirect -> {
                    val location = hop.location ?: throw UpdateException(UpdateFailure.Network)
                    // http・https 以外（ftp など）は URL として解決できない。許可外として止める。許可リストの検査は次の通信の直前（ガード）でも行う。
                    current = hop.base.resolve(location)?.takeIf(endpoints::permits) ?: throw UpdateException(UpdateFailure.Rejected)
                }
            }
        }
        throw UpdateException(UpdateFailure.Network) // リダイレクトが多すぎる
    }

    private fun save(
        response: Response,
        dest: File,
    ) {
        val expected = response.body.contentLength()
        if (expected > MAX_DOWNLOAD_BYTES) throw UpdateException(UpdateFailure.Truncated)
        var total = 0L
        val buffer = ByteArray(BUFFER_BYTES)
        try {
            dest.parentFile?.mkdirs()
            dest.outputStream().use { out ->
                val input = response.body.byteStream()
                while (true) {
                    val n = readChunk(input, buffer)
                    if (n < 0) break
                    total += n
                    if (total > MAX_DOWNLOAD_BYTES) throw UpdateException(UpdateFailure.Truncated)
                    out.write(buffer, 0, n)
                }
            }
        } catch (e: IOException) {
            throw UpdateException(UpdateFailure.Storage) // 読み取りのエラーは readChunk が Truncated にしている
        }
        if (expected >= 0 && total != expected) throw UpdateException(UpdateFailure.Truncated)
    }

    private fun readChunk(
        input: java.io.InputStream,
        buffer: ByteArray,
    ): Int =
        try {
            input.read(buffer)
        } catch (e: IOException) {
            throw UpdateException(UpdateFailure.Truncated)
        }
}

private const val HTTP_OK = 200
