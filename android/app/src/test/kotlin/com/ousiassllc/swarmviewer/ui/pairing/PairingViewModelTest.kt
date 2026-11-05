package com.ousiassllc.swarmviewer.ui.pairing

import app.cash.turbine.test
import com.ousiassllc.swarmviewer.data.store.FakeTokenStore
import com.ousiassllc.swarmviewer.data.store.FakeViewerStore
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.Session
import com.ousiassllc.swarmviewer.service.ConnectionService
import com.ousiassllc.swarmviewer.service.DRAFT_TTL
import com.ousiassllc.swarmviewer.service.MutableClock
import com.ousiassllc.swarmviewer.service.PairingService
import com.ousiassllc.swarmviewer.service.link
import com.ousiassllc.swarmviewer.ui.DRAFT_EXPIRED_MESSAGE
import com.ousiassllc.swarmviewer.ui.SAVE_FAILED_MESSAGE
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

private const val TOKEN = "test-token-not-secret"

@OptIn(ExperimentalCoroutinesApi::class)
class PairingViewModelTest {
    private val store = FakeViewerStore()
    private val tokens = FakeTokenStore()
    private val clock = MutableClock()
    private var probeFailure: FetchException? = null
    private val service =
        PairingService(
            probe = { _, _ ->
                probeFailure?.let { throw it }
                listOf(Session("0", true))
            },
            store = store,
            tokens = tokens,
            connections = ConnectionService(store, tokens, io = Dispatchers.Unconfined),
            clock = clock,
            io = Dispatchers.Unconfined,
        )
    private lateinit var viewModel: PairingViewModel

    @Before
    fun setUp() {
        Dispatchers.setMain(UnconfinedTestDispatcher())
        viewModel = PairingViewModel(service)
    }

    @After
    fun tearDown() = Dispatchers.resetMain()

    /** 画面に出る・状態に残るどこにも、token・完全な URL・貼り付けた文字列が無いこと。 */
    private fun assertNothingSecretIsShown(raw: String) {
        val shown = viewModel.state.value.toString()
        assertFalse(shown, shown.contains(TOKEN))
        assertFalse(shown, shown.contains("48120"))
        assertFalse(shown, shown.contains(raw))
    }

    @Test
    fun pastingShowsTheConfirmationAndClearsTheInput() {
        val raw = link(name = "devbox", sessions = "0%2C1")
        viewModel.onInputChange(raw)
        assertEquals(raw, viewModel.state.value.input)

        viewModel.submitInput()

        val state = viewModel.state.value
        assertEquals("", state.input) // 解析後はすぐ破棄する
        assertEquals("devbox", state.draft?.name)
        assertEquals("100.64.0.5", state.draft?.host)
        assertEquals(listOf("0", "1"), state.draft?.sessionsHint)
        assertNull(state.message)
        assertFalse(state.busy)
        assertNothingSecretIsShown(raw)
        assertTrue(store.connectionRows.isEmpty()) // 確定するまで保存しない
    }

    @Test
    fun aBlankInputDoesNothing() {
        viewModel.onInputChange("   ")

        viewModel.submitInput()

        assertNull(viewModel.state.value.draft)
        assertNull(viewModel.state.value.message)
    }

    @Test
    fun invalidInputShowsTheSpecifiedMessageAndDropsTheInput() {
        val cases =
            mapOf(
                "http://example.com" to "接続文字列の形式が正しくありません",
                "swarm-viewer://pair?v=2&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=$TOKEN" to "アプリを更新してください",
                link(url = "http%3A%2F%2F192.168.0.1%3A48120%2Freport") to "接続先が許可された範囲外です（Tailscale の IP のみ）",
                link(token = "") to "token が含まれていません",
            )
        for ((raw, message) in cases) {
            viewModel.onInputChange(raw)
            viewModel.submitInput()

            assertEquals(message, viewModel.state.value.message)
            assertEquals("", viewModel.state.value.input)
            assertNull(viewModel.state.value.draft)
            assertNothingSecretIsShown(raw)
        }
    }

    @Test
    fun deepLinksAndQrGoThroughTheSameConfirmation() {
        viewModel.receive(link(name = "from-link"))

        assertEquals(
            "from-link",
            viewModel.state.value.draft
                ?.name,
        )

        viewModel.receive(link(name = "from-qr", token = "other-token")) // 次の入力は下書きを置き換える
        assertEquals(
            "from-qr",
            viewModel.state.value.draft
                ?.name,
        )
    }

    @Test
    fun confirmSavesAndAnnouncesIt() =
        runTest {
            viewModel.receive(link(name = "devbox"))

            viewModel.saved.test {
                viewModel.confirm()
                awaitItem()
            }

            assertEquals("devbox", store.connections().single().name)
            assertEquals(TOKEN, tokens.tokens[store.connections().single().id])
            assertNull(viewModel.state.value.draft) // 画面は初期状態に戻る
            assertFalse(viewModel.state.value.busy)
        }

    @Test
    fun aFailedProbeShowsTheFixedMessageKeepsTheDraftAndSavesNothing() =
        runTest {
            val raw = link(name = "devbox")
            viewModel.receive(raw)
            val messages =
                mapOf(
                    ErrKind.Unreachable to "ホストに届きません。Tailscale 接続を確認してください",
                    ErrKind.Unauthorized to "認証できません。再ペアしてください",
                    ErrKind.ForbiddenScope to "このトークンでは接続できません（report 用が必要）",
                    ErrKind.Protocol to "接続先がこのアプリと互換ではありません",
                )
            for ((kind, message) in messages) {
                probeFailure = FetchException(kind, "some_code")

                viewModel.confirm()

                assertEquals(message, viewModel.state.value.message)
                assertEquals(
                    "devbox",
                    viewModel.state.value.draft
                        ?.name,
                ) // 下書きは残り、再試行できる
                assertFalse(viewModel.state.value.busy)
                assertTrue(store.connectionRows.isEmpty())
                assertTrue(tokens.tokens.isEmpty())
                assertNothingSecretIsShown(raw)
            }

            probeFailure = null // Tailscale を入れ直した
            viewModel.saved.test {
                viewModel.confirm()
                awaitItem()
            }
            assertEquals(1, store.connectionRows.size)
        }

    @Test
    fun anExpiredDraftReturnsToTheInput() {
        viewModel.receive(link())
        clock.now = clock.now.plus(DRAFT_TTL)

        viewModel.confirm()

        assertEquals(DRAFT_EXPIRED_MESSAGE, viewModel.state.value.message)
        assertNull(viewModel.state.value.draft)
        assertTrue(store.connectionRows.isEmpty())
    }

    @Test
    fun aTokenSaveFailureShowsAFixedMessageAndKeepsTheDraft() {
        viewModel.receive(link())
        tokens.failPut = true

        viewModel.confirm()

        assertEquals(SAVE_FAILED_MESSAGE, viewModel.state.value.message)
        assertEquals(
            "100.64.0.5",
            viewModel.state.value.draft
                ?.host,
        )
        assertTrue(store.connectionRows.isEmpty())
    }

    @Test
    fun cancelDiscardsTheDraftAndTheToken() {
        viewModel.receive(link())

        viewModel.cancel()

        assertNull(viewModel.state.value.draft)
        viewModel.receive(link(token = "second-token"))
        viewModel.confirm()
        assertEquals("second-token", tokens.tokens.values.single())
    }

    @Test
    fun confirmWithoutADraftDoesNothing() {
        viewModel.confirm()

        assertTrue(store.connectionRows.isEmpty())
        assertNull(viewModel.state.value.message)
    }
}
