package com.ousiassllc.swarmviewer.notify

import com.ousiassllc.swarmviewer.domain.PaneState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

class NotificationChannelsTest {
    @Test
    fun askingAndStalledUseSeparateChannelsSoTheUserCanTuneThemApart() {
        assertEquals("asking", NotificationChannels.idFor(PaneState.Asking))
        assertEquals("stalled", NotificationChannels.idFor(PaneState.Stalled))
        assertNotEquals(NotificationChannels.idFor(PaneState.Asking), NotificationChannels.idFor(PaneState.Stalled))
        assertNotEquals(NotificationChannels.REPAIR, NotificationChannels.idFor(PaneState.Asking))
    }
}
