package com.ousiassllc.swarmviewer.data.update

import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl

/**
 * 更新の確認・ダウンロードが通信してよい宛先（#109）。tailnet の `/report`・`/ops` の許可リスト（`GuardInterceptor`）とは別で、
 * GitHub のホストは向こうには加えない。
 *
 * 本番は [PRODUCTION] だけ（GitHub API と、Releases のダウンロードのホスト。https のみ・443 のみ）。リダイレクト先も同じ検査を通す。
 * 緩めた設定（[forTesting]）は、デバッグビルド（`src/debug` の [updateEndpointsOverride]）と JVM のテストだけが作る。リリースビルドには
 * それを作るコードが無い（`src/release` の [updateEndpointsOverride] は常に null）。
 */
class UpdateEndpoints private constructor(
    val latestUrl: HttpUrl,
    private val hosts: Set<String>,
    /** true のときだけ、[hosts] に限り http と 443 以外のポートを許す（デバッグビルドの偽サーバー・JVM のテスト用）。 */
    val relaxed: Boolean,
) {
    /** [url] が許可された宛先か。本番は https・ポート 443・許可リストのホスト。 */
    fun permits(url: HttpUrl): Boolean {
        if (url.host !in hosts) return false
        return relaxed || (url.isHttps && url.port == HTTPS_PORT)
    }

    // 宛先の一覧は出す必要がない。
    override fun toString(): String = "UpdateEndpoints"

    companion object {
        private const val HTTPS_PORT = 443

        /** 最新のリリースを返す GitHub API。 */
        const val LATEST_URL = "https://api.github.com/repos/ousiassllc/swarm-viewer/releases/latest"

        /** 通信してよいホスト（API と、Releases の成果物のダウンロード元とそのリダイレクト先）。 */
        val GITHUB_HOSTS: Set<String> =
            setOf("api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com")

        val PRODUCTION = UpdateEndpoints(LATEST_URL.toHttpUrl(), GITHUB_HOSTS, relaxed = false)

        /** [latestUrl] のホストだけを許す、http・任意のポートも通す設定（デバッグビルドの偽サーバー・テスト用。本番では作られない）。 */
        internal fun forTesting(latestUrl: String): UpdateEndpoints {
            val url = latestUrl.toHttpUrl()
            return UpdateEndpoints(url, setOf(url.host), relaxed = true)
        }
    }
}
