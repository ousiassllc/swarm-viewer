package com.ousiassllc.swarmviewer.domain

import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.nio.charset.CharacterCodingException
import java.nio.charset.CodingErrorAction

/** 対応するペイロード版。 */
const val PAIRING_VERSION = 1

/** 接続文字列全体（前後の空白除去後）のサイズ上限（バイト）。 */
const val MAX_PAIRING_BYTES = 4096

/** 表示名・セッションのヒントの最大文字数。 */
const val MAX_NAME_CHARS = 100

private const val PAIRING_SCHEME = "swarm-viewer"
private const val PAIRING_HOST = "pair"
private const val REDACTED = "***"

/** ペアリング失敗の種別（API 仕様 §2.2 の表示に対応する）。 */
enum class PairingError(
    val id: String,
) {
    /** scheme・パラメータ重複・サイズ超過・必須欠落・name 不正など形式の誤り。 */
    Format("format"),

    /** `v` が対応版（1）でない。「アプリを更新してください」。 */
    Version("version"),

    /** token が空・空白や制御文字を含む。 */
    Token("token"),

    /** 接続先 URL が許可範囲外・形式不正（[validateUrl]）。 */
    InvalidUrl("invalid_url"),
}

/** ペアリング失敗。メッセージは種別だけの固定文言で、入力値（token を含みうる）を含めない。 */
class PairingException(
    val error: PairingError,
) : Exception("invalid pairing string: ${error.id}")

/**
 * 解析・検証済みの接続文字列。token を持つ唯一の domain 型。
 * [toString] は token を伏せ、URL も host のみにする（ログ・例外・画面に token を出さない。NFR-SEC-7・NFR-SEC-11）。
 */
data class PairingPayload(
    val version: Int,
    /** [validateUrl] で正規化済み。 */
    val url: String,
    val token: String,
    /** 未指定なら URL の host。 */
    val name: String,
    /** 表示のヒントのみ。権限の正は `sessions_list`。 */
    val sessionsHint: List<String> = emptyList(),
) {
    override fun toString(): String = "PairingPayload(version=$version host=${hostOf(url)} name=$name token=$REDACTED)"
}

/**
 * 接続文字列 `swarm-viewer://pair?v=1&url=…&token=…[&name=…][&sessions=…]` を検証して解析する（API 仕様 §2.1）。
 * 貼り付け・ディープリンク・QR のどれでも同じ。デスクトップ版の ParsePairing と同じ結果になる（testdata/vectors/pairing.json）。
 *
 *  - 前後の空白を除去し、4,096 バイトを超えたら [PairingError.Format]
 *  - scheme・host（pair）は大文字小文字を区別しない。path・userinfo・port・フラグメントは不可
 *  - 同じパラメータが複数あれば [PairingError.Format]（未知のパラメータも対象）。未知のパラメータは無視
 *  - v が 1 以外は [PairingError.Version]（欠落は [PairingError.Format]）
 *  - url は [validateUrl]（失敗は [PairingError.InvalidUrl]）
 *  - token は空でなく、空白・制御文字を含まない（失敗は [PairingError.Token]）
 *  - name は任意で 1〜100 文字・制御文字なし（無ければ url の host）。不正は [PairingError.Format]
 *  - sessions はカンマ区切りのヒント。空要素・制御文字を含む要素・100 文字超の要素は捨てる
 *
 * クエリは標準のフォームエンコードとして復号する（`+` は空白）。token に `+` を含める場合は %2B にする。
 */
fun parsePairing(raw: String): PairingPayload {
    val text = raw.trimGoSpace()
    val size = text.utf8LengthOrNull()
    if (size == null || size == 0 || size > MAX_PAIRING_BYTES || text.contains('#')) throw PairingException(PairingError.Format)
    val params = pairingParams(text)
    when (params["v"]) {
        null -> throw PairingException(PairingError.Format)
        PAIRING_VERSION.toString() -> Unit
        else -> throw PairingException(PairingError.Version)
    }
    val url =
        try {
            validateUrl(params["url"].orEmpty())
        } catch (e: InvalidUrlException) {
            throw PairingException(PairingError.InvalidUrl)
        }
    val token = params["token"].orEmpty()
    if (token.isEmpty() || token.any { isGoSpace(it) || isGoControl(it) }) throw PairingException(PairingError.Token)
    return PairingPayload(
        version = PAIRING_VERSION,
        url = url,
        token = token,
        name = pairingName(params["name"], url),
        sessionsHint = sessionsHint(params["sessions"].orEmpty()),
    )
}

// scheme・host を検証し、クエリを重複なしの map にして返す。
private fun pairingParams(text: String): Map<String, String> {
    // Go の url.Parse は ASCII の制御文字を含む URL を拒否する。
    if (text.any { it.code < 0x20 || it.code == 0x7F }) throw PairingException(PairingError.Format)
    val prefix = "$PAIRING_SCHEME://"
    if (!text.startsWith(prefix, ignoreCase = true)) throw PairingException(PairingError.Format)
    val rest = text.substring(prefix.length)
    val query = rest.substringAfter('?', "")
    val authorityAndPath = rest.substringBefore('?')
    val slash = authorityAndPath.indexOf('/')
    val authority = if (slash < 0) authorityAndPath else authorityAndPath.substring(0, slash)
    val path = if (slash < 0) "" else authorityAndPath.substring(slash)
    if (!authority.equals(PAIRING_HOST, ignoreCase = true) || (path != "" && path != "/")) {
        throw PairingException(PairingError.Format)
    }
    return parseQuery(query)
}

// Go の url.ParseQuery と同じ: `&` 区切り、`;` を含む組は不正、空の組は無視、キー・値はフォームエンコードの復号。
private fun parseQuery(query: String): Map<String, String> {
    val params = LinkedHashMap<String, String>()
    for (pair in query.split('&')) {
        if (pair.contains(';')) throw PairingException(PairingError.Format)
        if (pair.isEmpty()) continue
        val key = queryUnescape(pair.substringBefore('='))
        val value = queryUnescape(pair.substringAfter('=', ""))
        if (params.put(key, value) != null) throw PairingException(PairingError.Format)
    }
    return params
}

// `+` は空白、`%XX` は 1 バイト。不正なエスケープ・UTF-8 として不正なバイト列は Format（Go は後者を許すが、token に不正なバイトは入れない）。
private fun queryUnescape(text: String): String {
    val bytes = ByteArrayOutputStream(text.length)
    var i = 0
    while (i < text.length) {
        val c = text[i]
        when (c) {
            '%' -> {
                val high = text.getOrNull(i + 1)?.let { Character.digit(it, 16) } ?: -1
                val low = text.getOrNull(i + 2)?.let { Character.digit(it, 16) } ?: -1
                if (high < 0 || low < 0 || text[i + 1].code >= 0x80 || text[i + 2].code >= 0x80) {
                    throw PairingException(PairingError.Format)
                }
                bytes.write(high * 16 + low)
                i += 3
            }
            '+' -> {
                bytes.write(' '.code)
                i++
            }
            else -> {
                val end = if (Character.isHighSurrogate(c) && i + 1 < text.length) i + 2 else i + 1
                bytes.write(text.substring(i, end).toByteArray(Charsets.UTF_8))
                i = end
            }
        }
    }
    val decoder =
        Charsets.UTF_8
            .newDecoder()
            .onMalformedInput(CodingErrorAction.REPORT)
            .onUnmappableCharacter(CodingErrorAction.REPORT)
    return try {
        decoder.decode(ByteBuffer.wrap(bytes.toByteArray())).toString()
    } catch (e: CharacterCodingException) {
        throw PairingException(PairingError.Format)
    }
}

private fun pairingName(
    raw: String?,
    normalizedUrl: String,
): String {
    if (raw == null) return hostOf(normalizedUrl)
    val name = raw.trimGoSpace()
    if (name.codePointCount(0, name.length) !in 1..MAX_NAME_CHARS || name.hasControl()) {
        throw PairingException(PairingError.Format)
    }
    return name
}

private fun sessionsHint(raw: String): List<String> =
    raw
        .split(',')
        .map { it.trimGoSpace() }
        .filter { it.isNotEmpty() && it.codePointCount(0, it.length) <= MAX_NAME_CHARS && !it.hasControl() }
