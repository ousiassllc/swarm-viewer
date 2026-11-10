package com.ousiassllc.swarmviewer.domain

/**
 * アプリの版 `MAJOR.MINOR.PATCH`（数値。#109）。比較は成分ごとに数値で行う（v0.1.9 < v0.1.13）。
 * 先頭の 0・4 成分以上・数字以外・9 桁を超える数は版として読まない（[parseTag]・[parseVersionName] が null）。
 * `.github/scripts/android-version.sh` が受け付ける形と同じ。
 */
data class AppVersion(
    val major: Int,
    val minor: Int,
    val patch: Int,
) : Comparable<AppVersion> {
    override fun compareTo(other: AppVersion): Int = compareValuesBy(this, other, AppVersion::major, AppVersion::minor, AppVersion::patch)

    override fun toString(): String = "$major.$minor.$patch"
}

private const val NUMBER = "(0|[1-9][0-9]{0,8})"
private val VERSION = Regex("$NUMBER\\.$NUMBER\\.$NUMBER")

/** リリースのタグ `vMAJOR.MINOR.PATCH` を読む。`v` の無いもの・それ以外の形は null（通知しない）。 */
fun parseTag(tag: String): AppVersion? = if (tag.startsWith("v")) parseVersionName(tag.substring(1)) else null

/** 実行中のアプリの `versionName`（`MAJOR.MINOR.PATCH`。`v` なし）を読む。読めなければ null。 */
fun parseVersionName(name: String): AppVersion? {
    val match = VERSION.matchEntire(name) ?: return null
    val (major, minor, patch) = match.destructured
    return AppVersion(major.toInt(), minor.toInt(), patch.toInt())
}
