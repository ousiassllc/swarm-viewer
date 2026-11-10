package com.ousiassllc.swarmviewer.data.update

import org.junit.Assert.assertNull
import org.junit.Test

/** リリースビルドは、更新の確認先を差し替えられない（`src/release` の関数は常に null。#109 §7）。リリースのユニットテストでだけ動く。 */
class ReleaseBuildHasNoOverrideTest {
    @Test
    fun theReleaseBuildAlwaysUsesTheGithubProductionEndpoints() {
        assertNull(updateEndpointsOverride())
    }
}
