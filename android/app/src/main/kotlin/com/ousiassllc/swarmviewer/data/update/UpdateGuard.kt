package com.ousiassllc.swarmviewer.data.update

import okhttp3.Dns
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import java.io.IOException
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress
import java.net.Proxy
import java.net.UnknownHostException
import java.util.concurrent.TimeUnit

/** 許可リスト外の宛先・GET 以外を止めたとき。メッセージは固定で、URL を含めない。 */
internal class UpdateRejectedException : IOException("update: request rejected")

private const val CONNECT_TIMEOUT_SECONDS = 15L
private const val READ_TIMEOUT_SECONDS = 30L

/** 送らないヘッダー。GitHub へはコンソールの token も cookie も、どんな認証情報も送らない。 */
private val CREDENTIAL_HEADERS = listOf("Authorization", "Proxy-Authorization", "Cookie")

/**
 * 更新用の [OkHttpClient]（#109）。tailnet の report/ops 用（`newReportHttpClient`）とは別のインスタンスで、
 *  - 環境のプロキシを使わず、リダイレクトに追従しない（追従は [GithubReleaseSource] が 1 回ごとに許可リストを検査して行う）
 *  - cookie を持たない（`CookieJar.NO_COOKIES`）。認証関連のヘッダーは [UpdateGuardInterceptor] が必ず外す
 *  - 本番は名前解決の結果から、ループバック・プライベート・tailnet（100.64.0.0/10）・リンクローカルのアドレスを除く（[PublicOnlyDns]）
 */
internal fun newUpdateHttpClient(endpoints: UpdateEndpoints): OkHttpClient =
    OkHttpClient
        .Builder()
        .proxy(Proxy.NO_PROXY)
        .followRedirects(false)
        .followSslRedirects(false)
        .dns(if (endpoints.relaxed) Dns.SYSTEM else PublicOnlyDns)
        .connectTimeout(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
        .readTimeout(READ_TIMEOUT_SECONDS, TimeUnit.SECONDS)
        .addInterceptor(UpdateGuardInterceptor(endpoints))
        .build()

/** 送信前に、宛先が [UpdateEndpoints] の許可リストの中か・GET かを検査し、認証関連のヘッダーを外す。 */
internal class UpdateGuardInterceptor(
    private val endpoints: UpdateEndpoints,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        if (request.method != "GET" || !endpoints.permits(request.url)) throw UpdateRejectedException()
        val builder = request.newBuilder()
        CREDENTIAL_HEADERS.forEach { builder.removeHeader(it) }
        return chain.proceed(builder.build())
    }
}

/** システムの名前解決から、公開されたアドレスだけを返す。残らなければ解決失敗（tailnet・社内の宛先へ向かわせない）。 */
internal object PublicOnlyDns : Dns {
    override fun lookup(hostname: String): List<InetAddress> =
        Dns.SYSTEM
            .lookup(hostname)
            .filter(::isPublicAddress)
            .ifEmpty { throw UnknownHostException(hostname) }
}

private const val CGNAT_FIRST_OCTET = 100
private const val CGNAT_SECOND_MASK = 0xC0
private const val CGNAT_SECOND_VALUE = 64
private const val ULA_MASK = 0xFE
private const val ULA_VALUE = 0xFC

/** ループバック・未指定・リンクローカル・プライベート（RFC 1918 と IPv6 ULA）・マルチキャスト・CGNAT（100.64.0.0/10。tailnet）以外か。 */
internal fun isPublicAddress(address: InetAddress): Boolean {
    if (address.isAnyLocalAddress || address.isLoopbackAddress || address.isLinkLocalAddress) return false
    if (address.isSiteLocalAddress || address.isMulticastAddress) return false
    val bytes = address.address
    return when (address) {
        is Inet4Address ->
            !((bytes[0].toInt() and 0xFF) == CGNAT_FIRST_OCTET && (bytes[1].toInt() and CGNAT_SECOND_MASK) == CGNAT_SECOND_VALUE)
        is Inet6Address -> (bytes[0].toInt() and ULA_MASK) != ULA_VALUE
        else -> false
    }
}
