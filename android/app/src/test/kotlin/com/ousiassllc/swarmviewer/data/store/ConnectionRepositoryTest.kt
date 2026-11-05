package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.domain.Connection
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test
import java.io.IOException

private suspend inline fun <reified T : Throwable> assertFailsWith(block: () -> Unit) {
    try {
        block()
    } catch (e: Throwable) {
        if (e is T) return
        throw e
    }
    throw AssertionError("${T::class.simpleName} が投げられなかった")
}

class ConnectionRepositoryTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val repository = ConnectionRepository(store, tokens)

    private suspend fun seed() {
        store.saveConnection(Connection(id = "c1", name = "a", url = "http://100.64.0.1:48120/report"))
        tokens.put("c1", "t1")
    }

    @Test
    fun deleteRemovesTheTokenAndTheRow() =
        runTest {
            seed()

            repository.delete("c1")

            assertNull(tokens.get("c1"))
            assertNull(store.connection("c1"))
        }

    @Test
    fun aTokenDeletionFailureKeepsTheConnectionSoItCanBeRetried() =
        runTest {
            seed()
            tokens.failDelete = true

            assertFailsWith<IOException> { repository.delete("c1") }

            assertNotNull(store.connection("c1"))
            assertEquals("t1", tokens.get("c1"))
            tokens.failDelete = false
            repository.delete("c1")
            assertNull(store.connection("c1"))
        }

    @Test
    fun aRowDeletionFailureLeavesTheConnectionWithoutToken() =
        runTest {
            seed()
            store.failDelete = true

            assertFailsWith<IllegalStateException> { repository.delete("c1") }

            assertNotNull(store.connection("c1")) // 接続は残る
            assertNull(tokens.get("c1")) // token は先に消えている（再ペアリングまたは再削除で片付く）
            store.failDelete = false
            repository.delete("c1")
            assertNull(store.connection("c1"))
        }

    @Test
    fun deletingAnUnknownConnectionIsANoOp() =
        runTest {
            repository.delete("nope")
        }
}
