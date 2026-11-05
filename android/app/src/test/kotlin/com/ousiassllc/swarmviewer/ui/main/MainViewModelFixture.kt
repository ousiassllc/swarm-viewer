package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.ReportFetcher
import com.ousiassllc.swarmviewer.service.SessionLister
import com.ousiassllc.swarmviewer.service.SettingsService
import com.ousiassllc.swarmviewer.service.SnapshotService
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Before
import java.time.Instant

internal const val TOKEN = "test-token-not-secret"
internal const val DEFAULT_INTERVAL_MILLIS = 10_000L

/** 取得を差し替えられる `report`。[gate] があれば、それが開くまで応答しない。 */
internal class ScriptedReports : ReportFetcher {
    val calls = ArrayList<Pair<String, String>>() // 接続 ID・セッション
    var gate: CompletableDeferred<Unit>? = null
    var failure: FetchException? = null
    var title = "first"

    override suspend fun report(
        connection: Connection,
        session: String,
    ): Report {
        calls += connection.id to session
        gate?.await()
        failure?.let { throw it }
        return Report(panes = listOf(Pane(pos = "T1", id = "%1", state = PaneState.Working, title = "$title-$session")))
    }
}

/** メイン画面の ViewModel テストの共通の土台（フェイクのストア・取得・時間の進め方）。 */
@OptIn(ExperimentalCoroutinesApi::class)
internal abstract class MainViewModelFixture {
    protected val store = FakeViewerStore()
    protected val tokens = FakeTokenStore()
    protected val reports = ScriptedReports()
    protected var allowed = listOf(Session("s0", true), Session("s1", true), Session("hidden", false))
    protected var sessionsCalls = 0
    protected var sessionsFailure: FetchException? = null
    protected val connections = ConnectionService(store, tokens, io = Dispatchers.Unconfined)

    @Before
    fun setUp() {
        Dispatchers.setMain(StandardTestDispatcher())
        addConnection("a", "100.64.0.1")
    }

    @After
    fun tearDown() = Dispatchers.resetMain()

    protected fun addConnection(
        id: String,
        host: String,
    ) {
        store.connectionRows[id] = Connection(id, "dev-$id", "http://$host:48120/report", createdAt = Instant.EPOCH)
        tokens.put(id, TOKEN)
    }

    protected fun newViewModel() =
        MainViewModel(
            SnapshotService(
                lister =
                    SessionLister {
                        sessionsCalls++
                        sessionsFailure?.let { throw it }
                        allowed
                    },
                reports = reports,
                store = store,
            ),
            connections,
            SettingsService(store),
        )

    protected fun TestScope.started(): MainViewModel {
        val viewModel = newViewModel()
        viewModel.onStart()
        runCurrent()
        return viewModel
    }

    protected fun TestScope.passTime(millis: Long) {
        advanceTimeBy(millis)
        runCurrent()
    }
}
