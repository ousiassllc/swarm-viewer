package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.service.AttentionObserver
import com.ousiassllc.swarmviewer.service.RepairFlags
import com.ousiassllc.swarmviewer.service.ReportFetcher
import com.ousiassllc.swarmviewer.service.SessionLister
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import java.time.Clock

/**
 * バックグラウンドの 1 回の確認（機能要件 §8.1・§12.5、FR-AND-3）。保存済みの全接続・全許可セッションを**順次**取得し（並列にしない。
 * NFR-PERF-6）、[observer]（[NotificationService]）に流して `asking` / `stalled` への遷移を通知する。
 *
 *  - 通知がオフなら何もしない（周期ジョブも止めているが、取り消しの合間に走った実行を含む）
 *  - 1 つの接続・セッションの取得失敗は飛ばして次へ進み、通知済みの記録を変えない。リトライしない（次の周期に持ち越す。NFR-AVL-2）
 *  - 401・403 の接続は [flags] に記録して確認の対象から外し、再ペアリングを促す通知を 1 回だけ出す（再ペアリングで記録を外す）
 *  - 新規ペアリング後の最初の一巡は通知せず記録だけにする（接続の `baselined_at`）。全許可セッションが取れた一巡の最後に `baselined_at` を記録する
 *
 * Android の API に依存しない（[com.ousiassllc.swarmviewer.notify.NotificationWorker] が組み立てて呼ぶ。JVM のユニットテストで検査する）。
 */
class AttentionCheck(
    private val store: ViewerStore,
    private val lister: SessionLister,
    private val reports: ReportFetcher,
    private val observer: AttentionObserver,
    private val notifier: AttentionNotifier,
    private val flags: RepairFlags,
    private val clock: Clock = Clock.systemUTC(),
) {
    suspend fun run() {
        if (!store.settings().notificationsEnabled) return
        for (connection in store.connections()) {
            currentCoroutineContext().ensureActive()
            if (flags.isFlagged(connection.id)) continue // 再ペアリングまで確認しない
            check(connection)
        }
    }

    private suspend fun check(connection: Connection) {
        var complete = true
        try {
            for (session in lister.sessions(connection).filter { it.allowed }) {
                try {
                    val report = reports.report(connection, session.id)
                    observer.observe(connection, session.id, report, clock.instant())
                } catch (e: CancellationException) {
                    throw e
                } catch (e: FetchException) {
                    if (needsRepair(connection, e)) return // この接続はもう確認しない
                    complete = false
                } catch (e: Exception) {
                    complete = false // 保存の失敗など。原因（パス・URL を含みうる）は使わず、次の周期で再試行する
                }
            }
            if (complete && !connection.baselined) store.markBaselined(connection.id, clock.instant())
        } catch (e: CancellationException) {
            throw e
        } catch (e: FetchException) {
            needsRepair(connection, e)
        } catch (e: Exception) {
            // 次の周期で再試行する（この接続の記録は変えない）
        }
    }

    /** 401・403 なら記録して通知を 1 回出し、true を返す。それ以外の失敗は false（飛ばして次の周期に持ち越す）。 */
    private fun needsRepair(
        connection: Connection,
        e: FetchException,
    ): Boolean {
        if (e.kind != ErrKind.Unauthorized && e.kind != ErrKind.ForbiddenScope) return false
        if (flags.flag(connection.id)) notifier.needsRepair(connection.id, connection.name)
        return true
    }
}
