package com.ousiassllc.swarmviewer.data

import android.content.Context
import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import com.ousiassllc.swarmviewer.data.store.RoomViewerStore
import com.ousiassllc.swarmviewer.data.store.SwarmDatabase
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.parseReport
import org.junit.After
import org.junit.Before
import java.io.File
import java.security.KeyStore
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset

internal const val ROOM_TOKEN = "test-token-not-secret-0123456789"
internal const val URL_1 = "http://100.64.0.1:48120/report"
internal const val URL_2 = "http://100.64.0.2:48120/report"
internal const val KEY_ALIAS = "swarm-viewer-room-test-key"

/** Room のテストの共通の準備（実際の SQLite・Keystore・トークンの保存先。テストごとに作り直して消す）。 */
internal abstract class RoomStoreTestBase {
    protected val context: Context = ApplicationProvider.getApplicationContext()
    protected val now = Instant.parse("2026-11-05T10:00:00Z")
    protected val clock = Clock.fixed(now, ZoneOffset.UTC)
    protected val tokenDirectory = File(context.noBackupFilesDir, "room-test-tokens")
    protected val dbName = "room-test.db"

    protected lateinit var db: SwarmDatabase
    protected lateinit var store: RoomViewerStore
    protected lateinit var tokens: FileTokenStore

    @Before
    fun setUp() {
        cleanFiles()
        db = Room.databaseBuilder(context, SwarmDatabase::class.java, dbName).build()
        store = RoomViewerStore(db, clock)
        tokens = FileTokenStore(tokenDirectory, KeystoreTokenCipher(KEY_ALIAS))
    }

    @After
    fun tearDown() {
        db.close()
        cleanFiles()
        KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(KEY_ALIAS)
    }

    protected fun cleanFiles() {
        context.deleteDatabase(dbName)
        tokenDirectory.deleteRecursively()
    }

    protected fun connection(
        id: String,
        url: String,
    ) = Connection(id = id, name = "dev-$id", url = url)

    protected fun report(state: String = "asking"): Report =
        parseReport("""{"layout":{"rows":1,"cols":[1]},"panes":[{"pos":"T1","id":"%1","state":"$state"}]}""")

    protected fun key(
        pane: String,
        state: PaneState = PaneState.Asking,
    ) = NotifiedKey(pane, state)

    protected fun count(table: String): Int =
        db.openHelper.readableDatabase.query("SELECT COUNT(*) FROM $table").use {
            it.moveToFirst()
            it.getInt(0)
        }
}
