package com.ousiassllc.swarmviewer.service

import com.ousiassllc.swarmviewer.data.store.ViewerStore
import com.ousiassllc.swarmviewer.domain.Settings
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * アプリ全体の設定（1 行固定。ER §7.2）の読み書き。保存のたびに [onSaved] を呼ぶ（通知用の周期ジョブを設定どおりに登録し直す）。読み取り→変更→保存を直列にして、画面ごとの更新が互いを上書きしないようにする。
 * 許容外の値は [com.ousiassllc.swarmviewer.domain.SettingsException]（保存しない）。
 */
class SettingsService(
    private val store: ViewerStore,
    private val onSaved: (Settings) -> Unit = {},
) {
    private val mutex = Mutex()

    suspend fun get(): Settings = store.settings()

    /** 設定を [transform] で変えて保存し、保存した設定を返す。 */
    suspend fun update(transform: Settings.() -> Settings): Settings =
        mutex.withLock {
            transform(store.settings()).also {
                store.saveSettings(it)
                onSaved(it)
            }
        }

    /** 画面更新間隔（秒。5 / 10 / 30 / 60）を保存する。 */
    suspend fun setScreenPollSeconds(seconds: Int): Settings = update { copy(screenPollSeconds = seconds) }

    /** 通知のオン / オフを保存する。オフの間は周期ジョブも止まる。 */
    suspend fun setNotificationsEnabled(enabled: Boolean): Settings = update { copy(notificationsEnabled = enabled) }

    /** 通知確認間隔（分。15 / 30 / 60）を保存する。 */
    suspend fun setNotifyPollMinutes(minutes: Int): Settings = update { copy(notifyPollMinutes = minutes) }
}
