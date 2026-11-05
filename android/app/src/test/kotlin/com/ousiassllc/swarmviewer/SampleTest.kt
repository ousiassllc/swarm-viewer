package com.ousiassllc.swarmviewer

import kotlinx.coroutines.test.runTest
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Test

class SampleTest {
    @Serializable
    private data class Sample(
        val name: String,
    )

    @Test
    fun jsonRoundTrip() =
        runTest {
            val json = Json.encodeToString(Sample.serializer(), Sample("swarm-viewer"))
            assertEquals(Sample("swarm-viewer"), Json.decodeFromString(Sample.serializer(), json))
        }
}
