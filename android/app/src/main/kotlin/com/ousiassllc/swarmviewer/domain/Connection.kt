package com.ousiassllc.swarmviewer.domain

import java.time.Instant

/** swarm-console の `/report` の待ち受けポート。tailnet の IP ではこのポートに限る。 */
const val REPORT_PORT = "48120"

/** 接続先 URL のパス。 */
const val REPORT_PATH = "/report"

private const val SCHEME_PREFIX = "http://"
private const val MAX_PORT = 65535

/** 接続先 URL が許可された範囲外・形式不正のとき（NFR-SEC-4・5）。メッセージは固定で、入力値を含めない。 */
class InvalidUrlException : Exception("connection url is not allowed")

/**
 * 保存済みの接続。token を持たない（token は Keystore で保護した領域だけ）。
 * [lastSession]・[baselinedAt] は未設定（DB の NULL）のとき null。
 */
data class Connection(
    val id: String,
    val name: String,
    val url: String,
    val lastSession: String? = null,
    val baselinedAt: Instant? = null,
    val createdAt: Instant? = null,
    val updatedAt: Instant? = null,
) {
    /** 初回の取得パスが完了済み（通知を出してよい段階）か。 */
    val baselined: Boolean get() = baselinedAt != null

    /** URL の host のみ（ログ・表示用。NFR-SEC-11）。解釈できなければ空文字。 */
    val host: String get() = hostOf(url)
}

/** URL から host のみを取り出す。IPv6 は角括弧なし。解釈できなければ空文字。 */
fun hostOf(url: String): String {
    val authority = url.substringAfter("://", "").substringBefore('/').substringAfterLast('@')
    return when {
        authority.startsWith('[') -> authority.substringAfter('[').substringBefore(']')
        else -> authority.substringBefore(':')
    }
}

/**
 * 接続先 URL を検証し、正規化した URL を返す（NFR-SEC-4・5）。許可するのは次の形だけ。違えば [InvalidUrlException]（理由は区別しない）。
 *  - scheme は http（大文字小文字は問わず小文字に正規化）
 *  - host は tailnet（100.64.0.0/10、fd7a:115c:a1e0::/48）またはループバック（127.0.0.0/8、::1）の IP リテラル。
 *    ホスト名・0.0.0.0・IPv6 ゾーン・IPv4 射影アドレス・省略形の IPv4 は不可
 *  - port は明示が必須。tailnet は 48120 のみ、ループバックは 1〜65535（先頭ゼロ不可）
 *  - path は /report ちょうど（末尾スラッシュ・パーセントエンコード別表記は不可）
 *  - userinfo・クエリ（空の `?` を含む）・フラグメントは不可
 *
 * 前後の空白は除去してから解析する。デスクトップ版の ValidateURL と同じ結果になる（testdata/vectors/url.json）。
 */
fun validateUrl(raw: String): String {
    val text = raw.trimGoSpace()
    // Go の url.Parse は ASCII の制御文字を含む URL を拒否する。
    if (text.isEmpty() || text.contains('#') || text.any { it.code < 0x20 || it.code == 0x7F }) throw InvalidUrlException()
    if (!text.startsWith(SCHEME_PREFIX, ignoreCase = true)) throw InvalidUrlException()
    val rest = text.substring(SCHEME_PREFIX.length)
    val slash = rest.indexOf('/')
    val authority = if (slash < 0) rest else rest.substring(0, slash)
    val path = if (slash < 0) "" else rest.substring(slash)
    if (rest.contains('?') || path != REPORT_PATH || authority.contains('@')) throw InvalidUrlException()

    val (host, port) = splitHostPort(authority)
    val address = IpAddress.parse(host) ?: throw InvalidUrlException()
    if (address.isV4InV6) throw InvalidUrlException()
    val normalizedPort = normalizePort(port, address)
    val hostText = if (address.isV4) address.toString() else "[$address]"
    return "$SCHEME_PREFIX$hostText:$normalizedPort$REPORT_PATH"
}

// `[v6]:port`・`v4:port`（port は省略・空も可）。角括弧なしの IPv6 は不可。
private fun splitHostPort(authority: String): Pair<String, String> {
    val host: String
    val tail: String
    if (authority.startsWith('[')) {
        val end = authority.indexOf(']')
        if (end < 0) throw InvalidUrlException()
        host = authority.substring(1, end)
        tail = authority.substring(end + 1)
    } else {
        val colon = authority.indexOf(':')
        host = if (colon < 0) authority else authority.substring(0, colon)
        tail = if (colon < 0) "" else authority.substring(colon)
    }
    if (tail.isEmpty()) return host to ""
    if (!tail.startsWith(':') || !tail.drop(1).all { it in '0'..'9' }) throw InvalidUrlException()
    return host to tail.substring(1)
}

private fun normalizePort(
    port: String,
    address: IpAddress,
): String =
    when {
        address.isTailnet -> if (port == REPORT_PORT) port else throw InvalidUrlException()
        address.isLoopback -> {
            val number = port.toIntOrNull()
            if (number == null || number !in 1..MAX_PORT || number.toString() != port) throw InvalidUrlException()
            port
        }
        else -> throw InvalidUrlException()
    }

/** `sessions_list` の 1 件（viewer が使う項目のみ）。 */
data class Session(
    val id: String,
    val allowed: Boolean,
)
