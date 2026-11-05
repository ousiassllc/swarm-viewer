package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.data.secret.TokenStore

/**
 * 接続の削除で、token・スナップショット・通知済み状態を一括で消す（NFR-SEC-8、ER §7.3）。
 * スナップショット・通知済み状態は外部キーの `ON DELETE CASCADE` で、token は Keystore で保護した領域のエントリの削除で消える。
 */
class ConnectionRepository(
    private val store: ViewerStore,
    private val tokens: TokenStore,
) {
    /**
     * 接続を削除する。先に token、次に接続の行（ER §7.3）。
     *  - token の削除に失敗したら接続は残したまま例外を伝える（再試行できる。token だけが孤立して残ることはない）
     *  - 行の削除に失敗した場合は token が先に消えているので、接続は残るが「token なし」（再ペアリングが必要）として扱われる。
     *    再ペアリングまたは再削除で片付く
     * 無い id は何もしない。
     */
    suspend fun delete(id: String) {
        tokens.delete(id)
        store.deleteConnection(id)
    }
}
