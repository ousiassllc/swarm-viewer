package com.ousiassllc.swarmviewer.ui.pane

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.service.AttentionObserver
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.FakePaneApi
import com.ousiassllc.swarmviewer.service.PaneService
import com.ousiassllc.swarmviewer.service.PaneTarget
import com.ousiassllc.swarmviewer.service.ReportFetcher
import com.ousiassllc.swarmviewer.service.SessionLister
import com.ousiassllc.swarmviewer.service.SnapshotService
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Before
import java.time.Instant

internal const val VIEW_ID = "view-conn"
internal const val OPS_ID = "ops-conn"

/** ペイン詳細・新しいセッションの ViewModel テストの土台。見るだけの接続（[VIEW_ID]）と操作の接続（[OPS_ID]）を 1 つずつ持つ。 */
@OptIn(ExperimentalCoroutinesApi::class)
internal abstract class PaneFixture {
    protected val store = FakeViewerStore()
    protected val tokens = FakeTokenStore()
    protected val api = FakePaneApi()
    protected val connections = ConnectionService(store, tokens, io = Dispatchers.Unconfined)
    protected val panes = PaneService(api, store, connections)
    protected var state = PaneState.Idle

    /** 新しいセッションを始める空きペイン（B1）の report 上の状態。起動後の再送の検査（最新の `report` で `idle` か）に使う。 */
    protected var startedPaneState = PaneState.Shell
    protected var reportFailure: FetchException? = null
    protected var reportCalls = 0

    protected val snapshots =
        SnapshotService(
            lister = SessionLister { listOf(Session("s0", true)) },
            reports =
                ReportFetcher { _, _ ->
                    reportCalls++
                    reportFailure?.let { throw it }
                    Report(
                        panes =
                            listOf(
                                Pane(pos = "T1", id = "%1", agent = "omp", state = state, title = "work", cwd = "/work/app"),
                                Pane(pos = "T2", id = "%2", agent = "claude", state = PaneState.Idle, title = "other", cwd = "/work/app"),
                                Pane(pos = "B1", id = "%3", agent = null, state = startedPaneState, title = "", cwd = "/home/x"),
                            ),
                    )
                },
            store = store,
            attention = AttentionObserver { _, _, _, _ -> },
        )

    @Before
    fun setUp() {
        Dispatchers.setMain(StandardTestDispatcher())
        store.connectionRows[VIEW_ID] = Connection(VIEW_ID, "view", "http://100.64.0.1:48120/report", createdAt = Instant.EPOCH)
        store.connectionRows[OPS_ID] = Connection(OPS_ID, "ops", "http://100.64.0.2:48120/ops", createdAt = Instant.EPOCH.plusSeconds(1))
        tokens.put(VIEW_ID, "test-token-not-secret")
        tokens.put(OPS_ID, "test-token-not-secret")
    }

    @After
    fun tearDown() = Dispatchers.resetMain()

    protected fun target(
        id: String,
        pos: String = "T1",
    ) = PaneTarget(id, "s0", pos)

    private val detailViewModels = mutableListOf<PaneDetailViewModel>()

    protected fun newDetail() = PaneDetailViewModel(snapshots, panes, connections).also { detailViewModels += it }

    /**
     * `runTest` に、使った [PaneDetailViewModel] を終わりに必ず止める後始末を足したもの。自動更新が動いたままだと、アサーションが
     * 失敗したときに仮想時間の消化が終わらず、失敗が報告されずにテストが止まってしまう。
     */
    protected fun runPaneTest(block: suspend TestScope.() -> Unit) =
        runTest {
            try {
                block()
            } finally {
                detailViewModels.forEach { it.onStop() }
            }
        }

    protected fun TestScope.opened(
        id: String,
        pos: String = "T1",
    ): PaneDetailViewModel {
        val viewModel = newDetail()
        viewModel.open(target(id, pos))
        viewModel.onStart()
        runCurrent()
        return viewModel
    }

    protected fun newStart() = StartSessionViewModel(panes, snapshots)

    protected fun TestScope.openedStart(id: String): StartSessionViewModel =
        newStart().also {
            it.open(target(id, "B1"))
            runCurrent()
        }

    protected fun TestScope.passTime(millis: Long) {
        advanceTimeBy(millis)
        runCurrent()
    }

    protected fun operationCalls() = api.calls.filterNot { it.startsWith("pane_tail") || it.startsWith("ask_card") }
}
