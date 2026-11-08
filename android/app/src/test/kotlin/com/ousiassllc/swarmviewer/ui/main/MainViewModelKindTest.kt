package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.ConnectionKind
import com.ousiassllc.swarmviewer.service.PaneTarget
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.time.Instant

@OptIn(ExperimentalCoroutinesApi::class)
internal class MainViewModelKindTest : MainViewModelFixture() {
    @Test
    fun aViewOnlyConnectionIsViewAndTheBadgeLabelIsViewOnly() =
        runTest {
            val viewModel = started()

            assertEquals(ConnectionKind.View, viewModel.state.value.kind)
            assertEquals("見るだけ", kindLabel(viewModel.state.value.kind))
            viewModel.onStop()
        }

    @Test
    fun anOpsConnectionIsOpsAndTheBadgeLabelIsOperation() =
        runTest {
            store.connectionRows["ops"] = Connection("ops", "ops", "http://100.64.0.9:48120/ops", createdAt = Instant.EPOCH.plusSeconds(1))
            tokens.put("ops", TOKEN)
            val viewModel = started()
            viewModel.selectConnection("ops")
            testScheduler.runCurrent()

            assertEquals(ConnectionKind.Ops, viewModel.state.value.kind)
            assertEquals("操作", kindLabel(viewModel.state.value.kind))
            viewModel.onStop()
        }

    @Test
    fun theTargetOfAPaneCardIsTheSelectedConnectionAndSessionAndNullBeforeTheyAreKnown() =
        runTest {
            assertNull(newViewModel().state.value.target("T1"))

            val viewModel = started()

            assertEquals(PaneTarget("a", "s0", "T1"), viewModel.state.value.target("T1"))
            viewModel.onStop()
        }
}
