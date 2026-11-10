package com.ousiassllc.swarmviewer

import android.content.Context
import android.content.pm.PackageManager
import androidx.work.WorkManager
import com.ousiassllc.swarmviewer.data.report.LogcatFetchLogger
import com.ousiassllc.swarmviewer.data.report.ReportClient
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import com.ousiassllc.swarmviewer.data.secret.TokenStore
import com.ousiassllc.swarmviewer.data.store.RoomViewerStore
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.data.update.AutoCheckSetting
import com.ousiassllc.swarmviewer.data.update.GithubReleaseSource
import com.ousiassllc.swarmviewer.data.update.PackageApkSigners
import com.ousiassllc.swarmviewer.data.update.PrefsAutoCheck
import com.ousiassllc.swarmviewer.data.update.SystemApkInstaller
import com.ousiassllc.swarmviewer.data.update.UpdateEndpoints
import com.ousiassllc.swarmviewer.data.update.updateEndpointsOverride
import com.ousiassllc.swarmviewer.domain.parseVersionName
import com.ousiassllc.swarmviewer.notify.AttentionCheck
import com.ousiassllc.swarmviewer.notify.AttentionNotifier
import com.ousiassllc.swarmviewer.notify.NotificationPermission
import com.ousiassllc.swarmviewer.notify.NotificationScheduler
import com.ousiassllc.swarmviewer.notify.NotificationService
import com.ousiassllc.swarmviewer.notify.PrefsRepairFlags
import com.ousiassllc.swarmviewer.notify.SystemNotifier
import com.ousiassllc.swarmviewer.notify.WorkManagerJobs
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.PairingService
import com.ousiassllc.swarmviewer.service.PaneService
import com.ousiassllc.swarmviewer.service.ReportFetcher
import com.ousiassllc.swarmviewer.service.SessionLister
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.service.SnapshotService
import com.ousiassllc.swarmviewer.service.UpdateScheduler
import com.ousiassllc.swarmviewer.service.UpdateService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import java.io.File

/** token の保存先のディレクトリ名（`noBackupFilesDir` 配下。バックアップ・端末間転送の対象外）。 */
private const val TOKEN_DIRECTORY = "tokens"

/** 更新の APK の保存先（`cacheDir` 配下。FileProvider の `res/xml/update_file_paths.xml` が公開するのもここだけ）。 */
private const val UPDATE_DIRECTORY = "updates"

/**
 * アプリ全体の依存（data 層とサービス）の組み立て。プロセスごとに 1 つ（Application サブクラスを持たず、最初の利用で作る）。
 */
class AppContainer private constructor(
    context: Context,
) {
    val store: ViewerStore = RoomViewerStore(SwarmDatabase.open(context))
    val tokens: TokenStore = FileTokenStore(File(context.noBackupFilesDir, TOKEN_DIRECTORY), KeystoreTokenCipher())
    val report = ReportClient(tokens = tokens::get, logger = LogcatFetchLogger)
    private val repairFlags = PrefsRepairFlags(context)
    private val sessionLister = SessionLister { connection -> report.sessions(connection) }
    private val reportFetcher = ReportFetcher { connection, session -> report.report(connection, session) }
    val connections = ConnectionService(store, tokens, flags = repairFlags)
    val permission = NotificationPermission(context)
    val scheduler = NotificationScheduler(WorkManagerJobs(WorkManager.getInstance(context)))
    private val notifier: AttentionNotifier = SystemNotifier(context)
    private val notifications = NotificationService(store, notifier)
    val pairing =
        PairingService(
            probe = { url, token -> report.probe(url, token) },
            reports = reportFetcher,
            store = store,
            tokens = tokens,
            connections = connections,
        )

    val settings = SettingsService(store, onSaved = scheduler::apply)
    val panes = PaneService(api = report, store = store, connections = connections)
    val snapshots = SnapshotService(lister = sessionLister, reports = reportFetcher, store = store, attention = notifications)

    /** バックグラウンドの確認（周期ジョブが 1 回ごとに使う）。 */
    val attentionCheck = AttentionCheck(store, sessionLister, reportFetcher, notifications, notifier, repairFlags)

    /** プロセスと同じ寿命のコルーチンのスコープ（新しい版の定期確認）。 */
    val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    /** 新しい版の確認の自動実行（設定の「自動で確認する」。既定はオン）。 */
    val autoCheck: AutoCheckSetting = PrefsAutoCheck(context)

    /**
     * 新しい版の確認と更新。GitHub への通信は、tailnet 用（[report]）とは別のクライアント・別の許可リストで行う。
     * 宛先は本番では [UpdateEndpoints.PRODUCTION]（GitHub だけ）。差し替えられるのはデバッグビルドだけ（[updateEndpointsOverride]）。
     */
    val updates =
        UpdateService(
            source = GithubReleaseSource(updateEndpointsOverride() ?: UpdateEndpoints.PRODUCTION),
            signers = PackageApkSigners(context),
            installer = SystemApkInstaller(context),
            autoCheck = autoCheck,
            current = installedVersion(context),
            dir = File(context.cacheDir, UPDATE_DIRECTORY),
        )
    val updateScheduler = UpdateScheduler(updates, autoCheck)

    companion object {
        @Volatile private var instance: AppContainer? = null

        fun from(context: Context): AppContainer =
            instance ?: synchronized(this) {
                instance ?: AppContainer(context.applicationContext).also { instance = it }
            }
    }
}

/** 実行中のアプリの版（`versionName`）。読めない・`X.Y.Z` でないときは null（更新の確認をしない）。 */
private fun installedVersion(context: Context) =
    try {
        context.packageManager
            .getPackageInfo(context.packageName, 0)
            .versionName
            ?.let(::parseVersionName)
    } catch (e: PackageManager.NameNotFoundException) {
        null
    }
