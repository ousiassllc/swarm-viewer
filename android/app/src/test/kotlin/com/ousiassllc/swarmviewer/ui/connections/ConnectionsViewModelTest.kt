package com.ousiassllc.swarmviewer.ui.connections

import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.ui.UNEXPECTED_ERROR_MESSAGE
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.time.Instant

@OptIn(ExperimentalCoroutinesApi::class)
class ConnectionsViewModelTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val service = ConnectionService(store, tokens, io = Dispatchers.Unconfined)

    @Before
    fun setUp() {
        Dispatchers.setMain(UnconfinedTestDispatcher())
        runBlocking {
            for (id in listOf("a", "b")) {
                store.saveConnection(
                    Connection(id, "dev-$id", "http://100.64.0.${if (id == "a") 1 else 2}:48120/report", createdAt = Instant.EPOCH),
                )
                tokens.put(id, "token-$id")
            }
        }
    }

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun theListShowsSavedConnectionsWithoutTokensOrFullUrls() {
        val viewModel = ConnectionsViewModel(service)

        val connections = viewModel.state.value.connections
        assertEquals(listOf("dev-a", "dev-b"), connections?.map { it.name })
        assertEquals(listOf("100.64.0.1", "100.64.0.2"), connections?.map { it.host })
        val shown = viewModel.state.value.toString()
        assertFalse(shown.contains("token-a") || shown.contains("48120"))
    }

    @Test
    fun selectingSwitchesTheSelectedConnection() {
        val viewModel = ConnectionsViewModel(service)

        viewModel.select("b")

        assertEquals(
            listOf(false, true),
            viewModel.state.value.connections
                ?.map { it.selected },
        )
    }

    @Test
    fun deleteAsksForConfirmationAndOnlyThenRemovesEverything() {
        val viewModel = ConnectionsViewModel(service)
        val target =
            viewModel.state.value.connections!!
                .first()

        viewModel.requestDelete(target)
        assertEquals(target, viewModel.state.value.pendingDelete)
        assertNotNull(store.connectionRows["a"]) // 確認の前は消えない

        viewModel.dismissDelete()
        assertNull(viewModel.state.value.pendingDelete)
        assertNotNull(store.connectionRows["a"])

        viewModel.requestDelete(target)
        viewModel.confirmDelete()

        assertNull(viewModel.state.value.pendingDelete)
        assertNull(store.connectionRows["a"])
        assertNull(tokens.tokens["a"])
        assertEquals(
            listOf("dev-b"),
            viewModel.state.value.connections
                ?.map { it.name },
        )
    }

    @Test
    fun aFailedDeleteKeepsTheConnectionAndShowsAFixedMessage() {
        val viewModel = ConnectionsViewModel(service)
        tokens.failDelete = true

        viewModel.requestDelete(
            viewModel.state.value.connections!!
                .first(),
        )
        viewModel.confirmDelete()

        assertEquals(UNEXPECTED_ERROR_MESSAGE, viewModel.state.value.message)
        assertEquals(
            2,
            viewModel.state.value.connections
                ?.size,
        )
        assertNotNull(store.connectionRows["a"])
        assertEquals("token-a", tokens.tokens["a"])

        tokens.failDelete = false
        viewModel.requestDelete(
            viewModel.state.value.connections!!
                .first(),
        )
        viewModel.confirmDelete()
        assertNull(viewModel.state.value.message)
        assertEquals(
            listOf("dev-b"),
            viewModel.state.value.connections
                ?.map { it.name },
        )
    }

    @Test
    fun aConnectionThatGot401ShowsRePairingNeeded() {
        val viewModel = ConnectionsViewModel(service)
        assertTrue(
            viewModel.state.value.connections!!
                .none { it.needsRepair },
        )

        service.markNeedsRepair("b")

        assertEquals(
            listOf(false, true),
            viewModel.state.value.connections
                ?.map { it.needsRepair },
        )

        service.markRepaired("b")
        assertTrue(
            viewModel.state.value.connections!!
                .none { it.needsRepair },
        )
    }
}
