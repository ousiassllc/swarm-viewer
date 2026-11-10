package com.ousiassllc.swarmviewer.data.update

import com.ousiassllc.swarmviewer.BuildConfig

/**
 * デバッグビルドだけの、更新の確認先の差し替え（実機・エミュレーターで、手元の偽のリリースサーバーに向けて動作を見せるため）。
 * Gradle の `-PupdateLatestUrl=http://10.0.2.2:PORT/repos/ousiassllc/swarm-viewer/releases/latest` を指定したビルドだけが差し替わり、
 * 指定が無ければ（CI の assembleDebug も）null で本番と同じ。リリースビルドは `src/release` の同名の関数が常に null を返す。
 */
internal fun updateEndpointsOverride(): UpdateEndpoints? =
    BuildConfig.UPDATE_LATEST_URL.takeIf { it.isNotEmpty() }?.let(UpdateEndpoints::forTesting)
