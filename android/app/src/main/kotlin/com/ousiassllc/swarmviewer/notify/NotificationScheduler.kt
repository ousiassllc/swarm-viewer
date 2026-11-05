package com.ousiassllc.swarmviewer.notify

import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import com.ousiassllc.swarmviewer.domain.Settings
import java.util.concurrent.TimeUnit

/** 周期ジョブの登録先。実装は [WorkManagerJobs]。テストではフェイクに差し替える。 */
interface PeriodicJobs {
    /** [intervalMinutes] ごとの確認ジョブを登録する（登録済みなら設定を置き換える）。 */
    fun schedule(intervalMinutes: Int)

    /** 確認ジョブを止める。 */
    fun cancel()
}

/**
 * 設定どおりに周期ジョブを登録する（機能要件 §12.1 FR-AND-3）。通知がオンなら設定の通知確認間隔（15 / 30 / 60 分）で登録し、
 * オフならジョブを止める（OS 通知のためだけの取得を行わない。§8.3）。
 * 設定を変えたとき・通知をオンに戻したとき・アプリの起動時に [apply] を呼び、設定どおりのジョブが登録されていることを保つ。
 */
class NotificationScheduler(
    private val jobs: PeriodicJobs,
) {
    fun apply(settings: Settings) {
        if (settings.notificationsEnabled) jobs.schedule(settings.notifyPollMinutes) else jobs.cancel()
    }
}

/** 周期ジョブの一意名（`enqueueUniquePeriodicWork`）。 */
const val ATTENTION_CHECK_WORK_NAME = "attention-check"

/**
 * [PeriodicJobs] の WorkManager 実装。`PeriodicWorkRequest` の最小間隔は 15 分（`PeriodicWorkRequest.MIN_PERIODIC_INTERVAL_MILLIS`）で、
 * 許容する間隔（15 / 30 / 60 分）はこれ以上。制約はネットワーク接続だけ。間隔を変えたときは [ExistingPeriodicWorkPolicy.UPDATE] で置き換える
 * （実行中の確認を止めず、ジョブの再実行時刻を不必要に動かさない）。WorkManager はジョブを永続化し、端末の再起動後に再スケジュールする。
 */
class WorkManagerJobs(
    private val workManager: WorkManager,
) : PeriodicJobs {
    override fun schedule(intervalMinutes: Int) {
        val request =
            PeriodicWorkRequestBuilder<NotificationWorker>(intervalMinutes.toLong(), TimeUnit.MINUTES)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .build()
        workManager.enqueueUniquePeriodicWork(ATTENTION_CHECK_WORK_NAME, ExistingPeriodicWorkPolicy.UPDATE, request)
    }

    override fun cancel() {
        workManager.cancelUniqueWork(ATTENTION_CHECK_WORK_NAME)
    }
}
