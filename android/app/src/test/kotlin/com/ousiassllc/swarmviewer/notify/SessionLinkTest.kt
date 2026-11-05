package com.ousiassllc.swarmviewer.notify

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SessionLinkTest {
    @Test
    fun aValidPairOpensThatConnectionAndSession() {
        assertEquals(SessionTarget("c1", "main"), SessionTarget.validated("c1", "main"))
    }

    @Test
    fun missingOrEmptyValuesAreRejected() {
        assertNull(SessionTarget.validated(null, "main"))
        assertNull(SessionTarget.validated("c1", null))
        assertNull(SessionTarget.validated("", "main"))
        assertNull(SessionTarget.validated("c1", ""))
    }

    @Test
    fun oversizedValuesFromOtherAppsAreRejected() {
        assertEquals(SessionTarget("c".repeat(64), "s".repeat(256)), SessionTarget.validated("c".repeat(64), "s".repeat(256)))
        assertNull(SessionTarget.validated("c".repeat(65), "main"))
        assertNull(SessionTarget.validated("c1", "s".repeat(257)))
    }
}
