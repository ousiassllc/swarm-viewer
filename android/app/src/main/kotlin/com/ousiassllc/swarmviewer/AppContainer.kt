package com.ousiassllc.swarmviewer

import android.content.Context
import androidx.work.WorkManager
import com.ousiassllc.swarmviewer.data.report.LogcatFetchLogger
import com.ousiassllc.swarmviewer.data.report.ReportClient
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import com.ousiassllc.swarmviewer.data.secret.TokenStore
import com.ousiassllc.swarmviewer.data.store.RoomViewerStore
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.data.store.ViewerStore
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
import java.io.File

/** token の保存先のディレクトリ名（`noBackupFilesDir` 配下。バックアップ・端末間転送の対象外）。 */
private const val TOKEN_DIRECTORY = "tokens"

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

    companion object {
        @Volatile private var instance: AppContainer? = null

        fun from(context: Context): AppContainer =
            instance ?: synchronized(this) {
                instance ?: AppContainer(context.applicationContext).also { instance = it }
            }
    }
}
