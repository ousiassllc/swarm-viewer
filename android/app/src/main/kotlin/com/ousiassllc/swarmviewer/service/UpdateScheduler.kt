package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.update.AutoCheckSetting
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import kotlin.time.Duration
import kotlin.time.Duration.Companion.hours

/** 新しい版の確認の間隔（既定 6 時間。#109）。 */
val UPDATE_CHECK_INTERVAL: Duration = 6.hours

/**
 * 新しい版の定期確認（#109）。[start] から、自動確認が入のあいだ、すぐに 1 回と、その後 [interval] ごとに [UpdateService.check] を呼ぶ。
 * 切にすると止まり（通信しない）、入に戻すとすぐに確認し直す。アプリのプロセスが生きている間だけ動く（周期ジョブは持たない）。
 */
class UpdateScheduler(
    private val service: UpdateService,
    private val autoCheck: AutoCheckSetting,
    private val interval: Duration = UPDATE_CHECK_INTERVAL,
) {
    private var job: Job? = null

    /** 開始する。すでに動いていれば何もしない。 */
    @Synchronized
    fun start(scope: CoroutineScope) {
        if (job?.isActive == true) return
        job =
            scope.launch {
                autoCheck.enabled.collectLatest { enabled ->
                    while (enabled) {
                        service.check()
                        delay(interval)
                    }
                }
            }
    }
}
