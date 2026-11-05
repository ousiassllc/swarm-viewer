package com.ousiassllc.swarmviewer.notify

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.ousiassllc.swarmviewer.AppContainer
import kotlinx.coroutines.CancellationException

/**
 * 周期ジョブの本体（WorkManager）。[AttentionCheck] を 1 回実行する。取得の失敗で [Result.retry] を返さない
 * （リトライしない。次の周期に持ち越す。NFR-AVL-2）ため、常に成功で終える。キャンセル（制約が外れた・ジョブの取り消し）は伝える。
 */
class NotificationWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        try {
            AppContainer.from(applicationContext).attentionCheck.run()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // 次の周期で再試行する。例外の本文（パス・URL を含みうる）はログに出さない。
        }
        return Result.success()
    }
}
