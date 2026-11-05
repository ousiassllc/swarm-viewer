package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.NotifiedKey
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.detect
import com.ousiassllc.swarmviewer.service.AttentionObserver
import java.time.Instant

/**
 * 取得した `report` から通知を判定して出す（機能要件 §8.2・§12.5）。画面更新の取得（[com.ousiassllc.swarmviewer.service.SnapshotService]）と
 * バックグラウンドの確認（[AttentionCheck]）の両方がこれを通る。
 *
 * domain の `detect` で遷移を判定し、通知済みの記録（`notified_states`）を更新する。通知を出すのは、記録に**実際に挿入できた**遷移だけ
 * （主キー `(接続, セッション, ペイン, 状態)` の制約が、2 つの経路が同じ遷移を同時に見たときの二重通知を防ぐ）。
 * 新規ペアリング後の最初の一巡（`baselined_at` が未記録）と、通知がオフの間は、記録だけ更新して通知しない
 * （オンに戻したとき、オフの間に入った状態が一斉に通知されるのを防ぐ）。
 */
class NotificationService(
    private val store: ViewerStore,
    private val notifier: AttentionNotifier,
) : AttentionObserver {
    override suspend fun observe(
        connection: Connection,
        session: String,
        report: Report,
        at: Instant,
    ) {
        val baselining = !connection.baselined
        val diff = detect(store.notifiedKeys(connection.id, session), report.panes, baselining)
        if (diff.insert.isEmpty() && diff.delete.isEmpty()) return
        val inserted = store.applyNotified(connection.id, session, diff, at).toSet()
        if (baselining || !store.settings().notificationsEnabled) return
        for (event in diff.notify) {
            if (NotifiedKey(event.pane.id, event.state) !in inserted) continue // 別の経路が先に記録した
            notifier.attention(
                AttentionNotice(connection.id, connection.name, session, event.pane.id, event.pane.pos, event.state),
            )
        }
    }
}
