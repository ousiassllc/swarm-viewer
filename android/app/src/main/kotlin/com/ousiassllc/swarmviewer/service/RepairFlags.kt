package com.ousiassllc.swarmviewer.service

/**
 * 再ペアリングが必要と分かった接続の記録（実装は `notify/PrefsRepairFlags`。プロセスをまたいで保つ）。バックグラウンドの確認は、記録のある接続を再ペアリングまで
 * 確認の対象から外し、再ペアリングを促す通知を接続ごとに 1 回だけ出す（機能要件 §10、API 仕様 §1.5）。
 * 再ペアリングの成功・接続の削除で [clear] する。
 */
interface RepairFlags {
    /** [connectionId] が記録済みか。 */
    fun isFlagged(connectionId: String): Boolean

    /** [connectionId] を記録する。新しく記録したときだけ true（呼び出し側はそのときだけ通知する）。 */
    fun flag(connectionId: String): Boolean

    /** 記録を外す。 */
    fun clear(connectionId: String)
}

/** メモリ上の [RepairFlags]（既定値・JVM のテスト用）。 */
class InMemoryRepairFlags : RepairFlags {
    private val ids = HashSet<String>()

    @Synchronized
    override fun isFlagged(connectionId: String): Boolean = connectionId in ids

    @Synchronized
    override fun flag(connectionId: String): Boolean = ids.add(connectionId)

    @Synchronized
    override fun clear(connectionId: String) {
        ids.remove(connectionId)
    }
}
