package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.data.secret.TokenStore
import java.io.IOException

/**
 * メモリ上の [TokenStore]（JVM のテスト用）。[failPut]・[failDelete] を立てると、保存・削除が [IOException] で失敗する。
 * [unreadable] の接続は、復号できない token として [get] が null を返す。
 */
internal class FakeTokenStore : TokenStore {
    val tokens = HashMap<String, String>()
    var failPut = false
    var failDelete = false
    val unreadable = HashSet<String>()

    override fun put(
        connectionId: String,
        token: String,
    ) {
        if (failPut) throw IOException("cannot write")
        tokens[connectionId] = token
    }

    override fun get(connectionId: String): String? = tokens[connectionId].takeIf { connectionId !in unreadable }

    override fun delete(connectionId: String) {
        if (failDelete) throw IOException("cannot delete")
        tokens.remove(connectionId)
    }
}
