package com.ousiassllc.swarmviewer.data.update

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import androidx.core.net.toUri
import com.ousiassllc.swarmviewer.domain.UpdateException
import com.ousiassllc.swarmviewer.domain.UpdateFailure
import java.io.File

/**
 * 検証済みの APK をシステムのパッケージインストーラへ渡す（#109）。実装は [SystemApkInstaller]。service 層のテストは偽物に差し替える。
 */
interface ApkInstaller {
    /** このアプリが「提供元不明のアプリのインストール」を許可されているか。 */
    fun canInstall(): Boolean

    /** 「提供元不明のアプリのインストール」の、このアプリの設定画面を開く。 */
    fun openInstallPermissionSettings()

    /** [apk] を開くインストーラ画面を出す。出せなければ [UpdateException]（[UpdateFailure.Installer]）。 */
    fun install(apk: File)
}

private const val APK_MIME_TYPE = "application/vnd.android.package-archive"

/** FileProvider の authority（マニフェストの `<provider>` と同じ。`${applicationId}.updates`）。 */
fun updateFileAuthority(context: Context): String = context.packageName + ".updates"

/**
 * `REQUEST_INSTALL_PACKAGES` と FileProvider（`res/xml/update_file_paths.xml`。更新の保存先のディレクトリだけ。非公開・一時的な読み取りの許可のみ）
 * で、システムのインストーラ（ACTION_VIEW）を開く [ApkInstaller]。
 */
class SystemApkInstaller(
    context: Context,
) : ApkInstaller {
    private val context = context.applicationContext

    override fun canInstall(): Boolean = context.packageManager.canRequestPackageInstalls()

    override fun openInstallPermissionSettings() {
        val intent =
            Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, "package:${context.packageName}".toUri())
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        start(intent)
    }

    override fun install(apk: File) {
        val uri =
            try {
                FileProvider.getUriForFile(context, updateFileAuthority(context), apk)
            } catch (e: IllegalArgumentException) {
                throw UpdateException(UpdateFailure.Storage) // 保存先のディレクトリの外のファイル
            }
        val intent =
            Intent(Intent.ACTION_VIEW)
                .setDataAndType(uri, APK_MIME_TYPE)
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        start(intent)
    }

    private fun start(intent: Intent) {
        try {
            context.startActivity(intent)
        } catch (e: ActivityNotFoundException) {
            throw UpdateException(UpdateFailure.Installer)
        }
    }
}
