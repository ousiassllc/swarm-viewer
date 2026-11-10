package com.ousiassllc.swarmviewer.data.update

import android.content.Context
import android.content.pm.PackageManager
import java.io.File
import java.security.MessageDigest

/**
 * APK の署名者の確認（#109）。署名者は、署名証明書の SHA-256（小文字の 16 進）の集合で表す。読めないときは空集合。
 * 実装は [PackageApkSigners]。service 層のテストは偽物に差し替える。
 */
interface ApkSigners {
    /** 実行中のアプリの署名者。 */
    fun running(): Set<String>

    /** ダウンロードした APK の署名者。パッケージ名がこのアプリと違う・読めない APK は空集合。 */
    fun of(apk: File): Set<String>
}

/** `PackageManager`（`GET_SIGNING_CERTIFICATES`）で署名者を読む [ApkSigners]。 */
class PackageApkSigners(
    context: Context,
) : ApkSigners {
    private val packages = context.applicationContext.packageManager
    private val packageName = context.applicationContext.packageName

    override fun running(): Set<String> =
        try {
            digests(packages.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES))
        } catch (e: PackageManager.NameNotFoundException) {
            emptySet()
        }

    override fun of(apk: File): Set<String> {
        val info = packages.getPackageArchiveInfo(apk.path, PackageManager.GET_SIGNING_CERTIFICATES) ?: return emptySet()
        return if (info.packageName == packageName) digests(info) else emptySet()
    }

    private fun digests(info: android.content.pm.PackageInfo): Set<String> =
        info.signingInfo
            ?.apkContentsSigners
            .orEmpty()
            .map { signature -> MessageDigest.getInstance("SHA-256").digest(signature.toByteArray()).toHex() }
            .toSet()
}

private fun ByteArray.toHex(): String = joinToString("") { "%02x".format(it) }
