package com.ousiassllc.swarmviewer

import android.content.Context
import com.ousiassllc.swarmviewer.data.report.LogcatFetchLogger
import com.ousiassllc.swarmviewer.data.report.ReportClient
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import com.ousiassllc.swarmviewer.data.secret.TokenStore
import com.ousiassllc.swarmviewer.data.store.RoomViewerStore
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.PairingService
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
    val connections = ConnectionService(store, tokens)
    val pairing =
        PairingService(
            probe = { url, token -> report.probe(url, token) },
            reports = { connection, session -> report.report(connection, session) },
            store = store,
            tokens = tokens,
            connections = connections,
        )

    val settings = SettingsService(store)
    val snapshots =
        SnapshotService(
            lister = { connection -> report.sessions(connection) },
            reports = { connection, session -> report.report(connection, session) },
            store = store,
        )

    companion object {
        @Volatile private var instance: AppContainer? = null

        fun from(context: Context): AppContainer =
            instance ?: synchronized(this) {
                instance ?: AppContainer(context.applicationContext).also { instance = it }
            }
    }
}
