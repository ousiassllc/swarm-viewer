package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull

/** 更新の失敗の種別（#109）。[id] は固定の種別名（ログ・テスト用）。画面の文言は ui 層が種別から引く。 */
enum class UpdateFailure(
    val id: String,
) {
    /** 通信できない（オフライン・DNS・接続・HTTP エラー・応答の形が想定外）。 */
    Network("network"),

    /** ダウンロードが途中で切れた（読み取りエラー・Content-Length より短い・上限超え）。 */
    Truncated("truncated"),

    /** リリースに APK または `SHA256SUMS` が無い。 */
    AssetMissing("asset_missing"),

    /** `SHA256SUMS` に APK の項目が無い・形式が不正・項目が重複している。 */
    ChecksumInvalid("checksum_invalid"),

    /** APK の SHA-256 が `SHA256SUMS` と違う。 */
    ChecksumMismatch("checksum_mismatch"),

    /** APK の署名が、実行中のアプリの署名と違う。 */
    SignatureMismatch("signature_mismatch"),

    /** 許可リスト外の宛先・https 以外への通信を止めた。 */
    Rejected("rejected"),

    /** 端末の保存領域に書けない・APK を開けない。 */
    Storage("storage"),

    /** システムのパッケージインストーラを開けない。 */
    Installer("installer"),
}

/** 更新の失敗。メッセージは種別だけの固定文言（URL・応答本文を含めない）。 */
class UpdateException(
    val failure: UpdateFailure,
) : Exception("update: ${failure.id}")

/** リリースの成果物の名前（タグ [tag] は `vX.Y.Z`）。 */
fun androidAssetName(tag: String): String = "swarm-viewer_${tag}_android_universal.apk"

/** SHA-256 の一覧のファイル名。 */
const val SHA256SUMS_ASSET = "SHA256SUMS"

/**
 * 最新のリリース。[assets] は成果物名 → ダウンロード URL（`browser_download_url`）。URL は信頼しない入力で、通信の直前に
 * 許可リスト（`UpdateEndpoints`）で検査する。
 */
data class Release(
    val tag: String,
    val version: AppVersion,
    val assets: Map<String, String>,
)

/**
 * `GET /repos/{owner}/{repo}/releases/latest` の応答を読む。draft・prerelease・タグが `vX.Y.Z` でないものは null（通知しない）。
 * JSON が壊れている・形が想定外なら [UpdateException]（[UpdateFailure.Network]）。
 */
fun parseLatestRelease(body: String): Release? {
    val root =
        try {
            Json.parseToJsonElement(body) as? JsonObject
        } catch (e: IllegalArgumentException) {
            null // SerializationException を含む。例外の本文（入力の一部）は捨てる
        } ?: throw UpdateException(UpdateFailure.Network)
    if (root.flag("draft") || root.flag("prerelease")) return null
    val tag = root.text("tag_name") ?: return null
    val version = parseTag(tag) ?: return null
    return Release(tag, version, root.assets())
}

private fun JsonObject.flag(key: String): Boolean = (this[key] as? JsonPrimitive)?.booleanOrNull == true

private fun JsonObject.text(key: String): String? = (this[key] as? JsonPrimitive)?.takeIf { it.isString }?.contentOrNull

private fun JsonObject.assets(): Map<String, String> =
    buildMap {
        (this@assets["assets"] as? JsonArray)?.forEach { element ->
            val asset = element as? JsonObject ?: return@forEach
            val name = asset.text("name")
            val url = asset.text("browser_download_url")
            if (name != null && url != null) putIfAbsent(name, url)
        }
    }

private val SUM_LINE = Regex("([0-9a-fA-F]{64}) [ *](.+)")

/**
 * `SHA256SUMS`（`sha256sum` の形式。1 行 `<64 桁の 16 進>  <ファイル名>`）から、[assetName] の SHA-256（小文字）を返す。
 * 空行以外で形式に合わない行が 1 つでもあれば、ファイル全体を信用せず [UpdateFailure.ChecksumInvalid]。
 * 項目が無い・同じ名前が複数あるときも [UpdateFailure.ChecksumInvalid]（どちらも導入しない）。
 */
fun expectedSha256(
    sums: String,
    assetName: String,
): String {
    val hashes = mutableListOf<String>()
    for (line in sums.lineSequence().map { it.removeSuffix("\r") }.filter { it.isNotBlank() }) {
        val match = SUM_LINE.matchEntire(line) ?: throw UpdateException(UpdateFailure.ChecksumInvalid)
        val (hash, name) = match.destructured
        if (name == assetName) hashes += hash.lowercase()
    }
    return hashes.singleOrNull() ?: throw UpdateException(UpdateFailure.ChecksumInvalid)
}
