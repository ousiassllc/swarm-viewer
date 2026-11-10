package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.update.ApkInstaller
import com.ousiassllc.swarmviewer.data.update.ApkSigners
import com.ousiassllc.swarmviewer.data.update.AutoCheckSetting
import com.ousiassllc.swarmviewer.data.update.ReleaseSource
import com.ousiassllc.swarmviewer.data.update.sha256Hex
import com.ousiassllc.swarmviewer.domain.AppVersion
import com.ousiassllc.swarmviewer.domain.Release
import com.ousiassllc.swarmviewer.domain.SHA256SUMS_ASSET
import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import com.ousiassllc.swarmviewer.domain.androidAssetName
import com.ousiassllc.swarmviewer.domain.expectedSha256
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException

/**
 * アプリの更新の状態（#109）。[release] は導入できる新しい版（無ければ null）。[updating] はダウンロード・検証・インストーラの起動の間 true。
 * [failure] は直近の確認・更新の失敗（小さな表示だけ。メイン画面の操作は止めない）。[needsInstallPermission] は、
 * 「提供元不明のアプリのインストール」の許可が無く、その設定画面を開いたあと（許可して、もう一度「更新する」を押す）。
 */
data class UpdateState(
    val release: Release? = null,
    val updating: Boolean = false,
    val failure: UpdateFailure? = null,
    val needsInstallPermission: Boolean = false,
)

/**
 * 新しい版の確認と更新（#109）。確認は [source] の最新のリリースを実行中の版 [current] と比べ、新しいときだけ [UpdateState.release] に出す。
 * 更新は、`SHA256SUMS` の項目と APK の SHA-256 が一致し、かつ APK の署名者が実行中のアプリと同じときだけ、[installer] を 1 回開く。
 * どの失敗でも APK は [dir] から消し、インストールしない。
 *
 *  - 自動確認が切（[autoCheck]）のとき、[check] は通信しない
 *  - [current] が null（版を読めない）のときは、通信せず何も出さない
 *  - 通信・検証の失敗は [UpdateState.failure] に入れるだけで、例外にしない（コルーチンのキャンセルはそのまま伝わる）
 */
class UpdateService(
    private val source: ReleaseSource,
    private val signers: ApkSigners,
    private val installer: ApkInstaller,
    private val autoCheck: AutoCheckSetting,
    private val current: AppVersion?,
    /** ダウンロードの保存先（FileProvider が公開するディレクトリ）。ここ以外には書かない。 */
    private val dir: File,
    private val io: CoroutineDispatcher = Dispatchers.IO,
) {
    private val mutableState = MutableStateFlow(UpdateState())
    val state: StateFlow<UpdateState> = mutableState.asStateFlow()

    /** 新しい版を確認する。自動確認が切・更新中・版が読めないときは何もしない（通信しない）。 */
    suspend fun check() {
        val installed = current
        if (installed == null || !autoCheck.enabled.value || mutableState.value.updating) return
        if (mutableState.value.release == null) withContext(io) { clearDownloads() }
        try {
            val latest = source.latest()?.takeIf { it.version > installed }
            mutableState.update { if (it.updating) it else it.copy(release = latest, failure = null) }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            mutableState.update { if (it.updating) it else it.copy(failure = failureOf(e)) }
        }
    }

    /** 見つけた新しい版をダウンロードし、検証して、インストーラを開く。 */
    suspend fun update() {
        val before = mutableState.value
        val release = before.release
        if (release == null || before.updating) return
        if (!mutableState.compareAndSet(before, before.copy(updating = true, failure = null, needsInstallPermission = false))) return
        var failure: UpdateFailure? = null
        var needsPermission = false
        try {
            withContext(io) { needsPermission = !prepareAndInstall(release) }
        } catch (e: CancellationException) {
            mutableState.update { it.copy(updating = false) }
            throw e
        } catch (e: Exception) {
            failure = failureOf(e)
        }
        mutableState.update { it.copy(updating = false, failure = failure, needsInstallPermission = needsPermission) }
    }

    /** 許可が無く設定画面を開いたときは false。検証に通ったときだけインストーラを開く。失敗は例外（APK は消す）。 */
    private suspend fun prepareAndInstall(release: Release): Boolean {
        if (!installer.canInstall()) {
            installer.openInstallPermissionSettings()
            return false
        }
        val assetName = androidAssetName(release.tag)
        val apkUrl = release.assets[assetName] ?: throw UpdateException(UpdateFailure.AssetMissing)
        val sumsUrl = release.assets[SHA256SUMS_ASSET] ?: throw UpdateException(UpdateFailure.AssetMissing)
        // 大きな APK を落とす前に、期待する SHA-256 を確かめる（項目が無い・形式が不正なら落とさない）。
        val expected = expectedSha256(source.text(sumsUrl), assetName)
        clearDownloads()
        val apk = File(dir, assetName)
        try {
            source.download(apkUrl, apk)
            verify(apk, expected)
            installer.install(apk)
        } catch (e: Throwable) {
            apk.delete() // 検証に通らなかった・開けなかった APK は残さない（キャンセルも含む）
            throw e
        }
        return true
    }

    private fun verify(
        apk: File,
        expected: String,
    ) {
        if (sha256Hex(apk) != expected) throw UpdateException(UpdateFailure.ChecksumMismatch)
        val running = signers.running()
        val downloaded = signers.of(apk)
        // 読めない（空）・実行中のアプリと違う署名は、どちらも導入しない。
        if (running.isEmpty() || downloaded != running) throw UpdateException(UpdateFailure.SignatureMismatch)
    }

    /** 前回までの APK を消す（更新の前・確認のときの掃除）。 */
    private fun clearDownloads() {
        dir.listFiles()?.forEach { it.delete() }
    }

    private fun failureOf(e: Exception): UpdateFailure =
        when (e) {
            is UpdateException -> e.failure
            is IOException -> UpdateFailure.Storage
            else -> UpdateFailure.Network // 想定外の失敗も、固定の種別に丸める（例外の本文は使わない）
        }
}
