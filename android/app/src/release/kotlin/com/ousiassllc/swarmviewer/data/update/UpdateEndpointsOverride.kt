package com.ousiassllc.swarmviewer.data.update

/** リリースビルドは更新の確認先を差し替えられない（常に GitHub の本番。デバッグビルドの同名の関数は `src/debug`）。 */
internal fun updateEndpointsOverride(): UpdateEndpoints? = null
