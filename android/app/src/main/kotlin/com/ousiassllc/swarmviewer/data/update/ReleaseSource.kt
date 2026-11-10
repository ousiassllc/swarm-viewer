package com.ousiassllc.swarmviewer.data.update

import com.ousiassllc.swarmviewer.domain.Release
import java.io.File

/**
 * 更新の取得元（GitHub Releases）。失敗は [com.ousiassllc.swarmviewer.domain.UpdateException]（コルーチンのキャンセルはそのまま伝わる）。
 * 実装は [GithubReleaseSource]。service 層のテストは偽物に差し替える。
 */
interface ReleaseSource {
    /** 最新のリリース。通知の対象がない（draft・prerelease・タグが `vX.Y.Z` でない）ときは null。 */
    suspend fun latest(): Release?

    /** 小さなテキストの成果物（`SHA256SUMS`）の本文。 */
    suspend fun text(url: String): String

    /** 成果物を [dest] へ書く。途中で切れたとき・失敗したときは [dest] を残さず例外にする。 */
    suspend fun download(
        url: String,
        dest: File,
    )
}
