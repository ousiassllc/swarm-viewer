package com.ousiassllc.swarmviewer.service.update

import com.ousiassllc.swarmviewer.domain.AppVersion
import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import com.ousiassllc.swarmviewer.domain.androidAssetName
import com.ousiassllc.swarmviewer.service.UpdateService
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/** 更新の確認・検証・インストールの判断（#109 §6 の (a)〜(g)）。通信・インストーラ・署名は偽物。 */
class UpdateServiceTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val source = FakeReleaseSource()
    private val signers = FakeSigners()
    private val installer = FakeInstaller()
    private val autoCheck = FakeAutoCheck()
    private val dir by lazy { File(tmp.root, "updates") }
    private val apkFile by lazy { File(dir, androidAssetName(NEW_TAG)) }

    private fun service(current: AppVersion? = AppVersion(0, 1, 0)) =
        UpdateService(source, signers, installer, autoCheck, current, dir, Dispatchers.Unconfined)

    /** 確認して新しい版を見つけ、「更新する」を押す。 */
    private fun checkAndUpdate(service: UpdateService = service()): UpdateService {
        runBlocking {
            service.check()
            assertNotNull(service.state.value.release)
            service.update()
        }
        return service
    }

    private fun assertNotInstalled(
        service: UpdateService,
        failure: UpdateFailure,
    ) {
        assertEquals(failure, service.state.value.failure)
        assertTrue(installer.installed.isEmpty())
        assertFalse(service.state.value.updating)
        assertFalse(apkFile.exists()) // どの失敗でも、ダウンロードした APK は残さない
    }

    @Test
    fun aNewerReleaseIsOfferedWithItsVersion() =
        runBlocking {
            val service = service()
            service.check()
            assertEquals(
                AppVersion(0, 2, 0),
                service.state.value.release
                    ?.version,
            )
            assertNull(service.state.value.failure)
            assertEquals(0, source.downloadCalls) // 確認だけではダウンロードしない
        }

    @Test
    fun sameOlderOrUnreadableCurrentVersionOffersNothing() =
        runBlocking {
            for (current in listOf(AppVersion(0, 2, 0), AppVersion(0, 3, 0))) {
                val service = service(current)
                service.check()
                assertNull(service.state.value.release)
            }
            val unknown = service(current = null)
            val calls = source.latestCalls
            unknown.check()
            assertEquals(calls, source.latestCalls) // 版が読めない（開発ビルドなど）なら通信もしない
            assertNull(unknown.state.value.release)
        }

    @Test
    fun noReleaseToOfferLeavesNoNotice() =
        runBlocking {
            source.release = null // draft・prerelease・タグが不正は、取得元が null にする
            val service = service()
            service.check()
            assertNull(service.state.value.release)
            assertNull(service.state.value.failure)
        }

    // (f)
    @Test
    fun theHappyPathLaunchesTheInstallerExactlyOnceWithTheVerifiedFile() {
        val service = checkAndUpdate()
        assertEquals(listOf(apkFile), installer.installed)
        assertArrayEquals(APK_BYTES, installer.installedBytes.single())
        assertEquals(listOf(apkFile), signers.checkedFiles) // 署名を確かめたのは、インストーラに渡したのと同じファイル
        assertNull(service.state.value.failure)
        assertFalse(service.state.value.updating)
        assertEquals(1, source.downloadCalls)
    }

    @Test
    fun pressingUpdateWhileUpdatingDoesNothing() =
        runBlocking {
            val service = service()
            service.check()
            val gate = CompletableDeferred<Unit>()
            source.sumsGate = gate
            val first = launch(start = CoroutineStart.UNDISPATCHED) { service.update() } // SHA256SUMS の取得で止まる（更新中）
            assertTrue(service.state.value.updating)
            service.update()
            gate.complete(Unit)
            first.join()
            assertEquals(1, source.textCalls)
            assertEquals(1, installer.installed.size)
            assertFalse(service.state.value.updating)
        }

    // (a)
    @Test
    fun aHashMismatchInstallsNothingAndDeletesTheFile() {
        source.apkBytes = "tampered".toByteArray()
        assertNotInstalled(checkAndUpdate(), UpdateFailure.ChecksumMismatch)
    }

    // (b)
    @Test
    fun sha256sumsWithoutTheAssetInstallsNothingAndDownloadsNothing() {
        source.sums = "${sha256(APK_BYTES)}  some-other-file\n"
        val service = checkAndUpdate()
        assertNotInstalled(service, UpdateFailure.ChecksumInvalid)
        assertEquals(0, source.downloadCalls)
    }

    // (b)
    @Test
    fun badlyFormattedOrDuplicateSha256sumsInstallsNothing() {
        for (sums in listOf(
            "garbage",
            "${sha256(APK_BYTES)}  ${androidAssetName(NEW_TAG)}\n${sha256(APK_BYTES)}  ${androidAssetName(NEW_TAG)}",
            "",
        )) {
            source.sums = sums
            assertNotInstalled(checkAndUpdate(), UpdateFailure.ChecksumInvalid)
        }
    }

    @Test
    fun aReleaseWithoutTheApkOrSha256sumsInstallsNothing() {
        source.release = releaseOf(assets = mapOf("SHA256SUMS" to SUMS_URL))
        assertNotInstalled(checkAndUpdate(), UpdateFailure.AssetMissing)
        source.release = releaseOf(assets = mapOf(androidAssetName(NEW_TAG) to APK_URL))
        assertNotInstalled(checkAndUpdate(), UpdateFailure.AssetMissing)
        assertEquals(0, source.downloadCalls)
    }

    // (c)
    @Test
    fun aDifferentSigningCertificateInstallsNothingAndDeletesTheFile() {
        signers.downloaded = setOf("bb22")
        assertNotInstalled(checkAndUpdate(), UpdateFailure.SignatureMismatch)
    }

    @Test
    fun anUnreadableOrPartiallyMatchingSignatureInstallsNothing() {
        signers.downloaded = emptySet() // パッケージ名の違い・壊れた APK
        assertNotInstalled(checkAndUpdate(), UpdateFailure.SignatureMismatch)
        signers.running = emptySet() // 実行中のアプリの署名を読めない
        signers.downloaded = emptySet()
        assertNotInstalled(checkAndUpdate(), UpdateFailure.SignatureMismatch)
        signers.running = setOf("aa11")
        signers.downloaded = setOf("aa11", "bb22") // 署名者が増えている
        assertNotInstalled(checkAndUpdate(), UpdateFailure.SignatureMismatch)
    }

    // (d)
    @Test
    fun anOfflineCheckShowsASmallFailureAndKeepsEverythingElse() =
        runBlocking {
            source.latestError = UpdateException(UpdateFailure.Network)
            val service = service()
            service.check()
            assertEquals(UpdateFailure.Network, service.state.value.failure)
            assertNull(service.state.value.release)
            assertTrue(installer.installed.isEmpty())
        }

    @Test
    fun aFailedLaterCheckKeepsTheNoticeAndASuccessfulOneClearsTheFailure() =
        runBlocking {
            val service = service()
            service.check()
            source.latestError = UpdateException(UpdateFailure.Network)
            service.check()
            assertNotNull(service.state.value.release)
            assertEquals(UpdateFailure.Network, service.state.value.failure)
            source.latestError = null
            service.check()
            assertNull(service.state.value.failure)
        }

    // (d)
    @Test
    fun offlineWhileDownloadingInstallsNothing() {
        source.sumsError = UpdateException(UpdateFailure.Network)
        assertNotInstalled(checkAndUpdate(), UpdateFailure.Network)
    }

    // (d)
    @Test
    fun aTruncatedDownloadInstallsNothingAndDeletesThePartialFile() {
        source.truncateWith = UpdateFailure.Truncated
        assertNotInstalled(checkAndUpdate(), UpdateFailure.Truncated)
    }

    @Test
    fun aFailureToOpenTheInstallerDeletesTheFile() {
        installer.installError = UpdateException(UpdateFailure.Installer)
        assertNotInstalled(checkAndUpdate(), UpdateFailure.Installer)
    }

    @Test
    fun withoutInstallPermissionTheSettingsPageOpensFirstAndNothingIsDownloaded() {
        installer.allowed = false
        val service = checkAndUpdate()
        assertEquals(1, installer.settingsOpened)
        assertTrue(service.state.value.needsInstallPermission)
        assertEquals(0, source.downloadCalls)
        assertEquals(0, source.textCalls)
        assertTrue(installer.installed.isEmpty())
        installer.allowed = true // 許可して、もう一度押す
        runBlocking { service.update() }
        assertEquals(listOf(apkFile), installer.installed)
        assertFalse(service.state.value.needsInstallPermission)
    }

    // (g)
    @Test
    fun withAutoCheckOffNoNetworkCallIsMade() =
        runBlocking {
            autoCheck.set(false)
            val service = service()
            service.check()
            assertEquals(0, source.latestCalls)
            assertEquals(0, source.textCalls)
            assertEquals(0, source.downloadCalls)
            assertNull(service.state.value.release)
            assertNull(service.state.value.failure)
        }

    @Test
    fun staleDownloadsAreClearedBeforeANewDownload() {
        dir.mkdirs()
        File(dir, "old.apk").writeBytes(byteArrayOf(1))
        checkAndUpdate()
        assertEquals(listOf(androidAssetName(NEW_TAG)), dir.list()!!.toList())
    }
}
