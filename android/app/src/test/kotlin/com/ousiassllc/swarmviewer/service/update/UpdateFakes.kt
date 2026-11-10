package com.ousiassllc.swarmviewer.service.update

import com.ousiassllc.swarmviewer.data.update.ApkInstaller
import com.ousiassllc.swarmviewer.data.update.ApkSigners
import com.ousiassllc.swarmviewer.data.update.AutoCheckSetting
import com.ousiassllc.swarmviewer.data.update.ReleaseSource
import com.ousiassllc.swarmviewer.domain.AppVersion
import com.ousiassllc.swarmviewer.domain.Release
import com.ousiassllc.swarmviewer.domain.SHA256SUMS_ASSET
import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import com.ousiassllc.swarmviewer.domain.androidAssetName
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import java.io.File
import java.io.IOException
import java.security.MessageDigest

const val NEW_TAG = "v0.2.0"
const val APK_URL = "https://github.com/ousiassllc/swarm-viewer/releases/download/v0.2.0/apk"
const val SUMS_URL = "https://github.com/ousiassllc/swarm-viewer/releases/download/v0.2.0/SHA256SUMS"
val APK_BYTES = "pretend this is an apk".toByteArray()

fun sha256(bytes: ByteArray): String = MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }

fun releaseOf(
    tag: String = NEW_TAG,
    assets: Map<String, String> = mapOf(androidAssetName(tag) to APK_URL, SHA256SUMS_ASSET to SUMS_URL),
) = Release(tag, AppVersion(0, 2, 0), assets)

/** 偽の取得元。呼び出しを数え、失敗・途中切れ・SHA256SUMS の中身を差し替えられる。 */
class FakeReleaseSource : ReleaseSource {
    var release: Release? = releaseOf()
    var latestError: UpdateException? = null
    var sums: String = "${sha256(APK_BYTES)}  ${androidAssetName(NEW_TAG)}\n"
    var sumsError: UpdateException? = null
    var apkBytes: ByteArray = APK_BYTES

    /** 設定すると、APK の一部を書いてからこの失敗で止まる（途中切れ）。 */
    var truncateWith: UpdateFailure? = null
    var latestCalls = 0
    var textCalls = 0
    var downloadCalls = 0

    /** 設定すると、SHA256SUMS の取得はこれが完了するまで待つ（更新中の状態を作る）。 */
    var sumsGate: CompletableDeferred<Unit>? = null

    override suspend fun latest(): Release? {
        latestCalls++
        latestError?.let { throw it }
        return release
    }

    override suspend fun text(url: String): String {
        textCalls++
        sumsGate?.await()
        sumsError?.let { throw it }
        return sums
    }

    override suspend fun download(
        url: String,
        dest: File,
    ) {
        downloadCalls++
        dest.parentFile?.mkdirs()
        truncateWith?.let {
            dest.writeBytes(apkBytes.copyOf(apkBytes.size / 2))
            throw UpdateException(it) // 本物は途中で止まったとき、ファイルを自分で消す。偽物は消さず、サービスの後始末も確かめる
        }
        dest.writeBytes(apkBytes)
    }
}

class FakeSigners(
    var running: Set<String> = setOf("aa11"),
    var downloaded: Set<String> = setOf("aa11"),
) : ApkSigners {
    var checkedFiles = mutableListOf<File>()

    override fun running(): Set<String> = running

    override fun of(apk: File): Set<String> {
        checkedFiles += apk
        return downloaded
    }
}

class FakeInstaller(
    var allowed: Boolean = true,
) : ApkInstaller {
    val installed = mutableListOf<File>()
    var installedBytes = mutableListOf<ByteArray>()
    var settingsOpened = 0
    var installError: UpdateException? = null

    override fun canInstall(): Boolean = allowed

    override fun openInstallPermissionSettings() {
        settingsOpened++
    }

    override fun install(apk: File) {
        installError?.let { throw it }
        installed += apk
        installedBytes += apk.readBytes() // 開いた時点のファイルの中身
    }
}

class FakeAutoCheck(
    initial: Boolean = true,
) : AutoCheckSetting {
    private val state = MutableStateFlow(initial)
    override val enabled: StateFlow<Boolean> = state

    override fun set(enabled: Boolean) {
        state.value = enabled
    }
}

/** 通信できない（オフライン）ときの失敗。 */
fun offline(): IOException = IOException("offline")
