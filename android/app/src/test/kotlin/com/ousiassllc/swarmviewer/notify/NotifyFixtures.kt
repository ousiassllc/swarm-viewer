package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.service.RepairFlags
import com.ousiassllc.swarmviewer.service.ReportFetcher
import com.ousiassllc.swarmviewer.service.SessionLister
import kotlinx.coroutines.yield

/** 出した通知を記録するフェイク。 */
internal class RecordingNotifier : AttentionNotifier {
    val notices = ArrayList<AttentionNotice>()
    val repairs = ArrayList<String>() // 接続 ID

    override fun attention(notice: AttentionNotice) {
        notices += notice
    }

    override fun needsRepair(
        connectionId: String,
        connectionName: String,
    ) {
        repairs += connectionId
    }
}

/** [RepairFlags] のフェイク（メモリ上）。 */
internal class FakeRepairFlags : RepairFlags {
    val ids = LinkedHashSet<String>()

    override fun isFlagged(connectionId: String) = connectionId in ids

    override fun flag(connectionId: String) = ids.add(connectionId)

    override fun clear(connectionId: String) {
        ids.remove(connectionId)
    }
}

internal fun pane(
    id: String,
    state: PaneState,
    pos: String = "T1",
) = Pane(pos = pos, id = id, state = state, title = "secret title", activity = "secret activity")

internal fun reportOf(vararg panes: Pane) = Report(panes = panes.toList())

/** 接続×セッションごとの応答を差し替えられる、フェイクの `ReportClient`（`sessions_list` と `report`）。 */
internal class FakeConsole {
    val events = ArrayList<String>() // 呼び出しの順（開始と終了）
    val sessions = HashMap<String, List<Session>>()
    val reports = HashMap<Pair<String, String>, Report>()
    val failures = HashMap<Pair<String, String>, FetchException>()
    val sessionFailures = HashMap<String, FetchException>()
    var inFlight = 0
    var maxInFlight = 0

    val lister =
        SessionLister { connection ->
            enter("sessions:${connection.id}")
            try {
                sessionFailures[connection.id]?.let { throw it }
                sessions[connection.id].orEmpty()
            } finally {
                leave()
            }
        }

    val fetcher =
        ReportFetcher { connection, session ->
            enter("report:${connection.id}/$session")
            try {
                failures[connection.id to session]?.let { throw it }
                reports[connection.id to session] ?: Report()
            } finally {
                leave()
            }
        }

    private suspend fun enter(event: String) {
        events += event
        inFlight++
        maxInFlight = maxOf(maxInFlight, inFlight)
        yield() // 並列に呼ばれていれば、ここで重なる
    }

    private fun leave() {
        inFlight--
    }
}
