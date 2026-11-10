package com.ousiassllc.swarmviewer.domain

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

/** 版の比較・リリース JSON・SHA256SUMS の読み取り（#109）。 */
class UpdateVersionTest {
    private fun newer(
        latest: String,
        current: String,
    ): Boolean = parseTag(latest)!! > parseVersionName(current)!!

    @Test
    fun versionsAreComparedNumericallyPerComponent() {
        assertTrue(newer("v0.1.13", "0.1.9")) // 文字列の比較なら "13" < "9" で逆になる
        assertTrue(newer("v0.2.0", "0.1.99"))
        assertTrue(newer("v1.0.0", "0.999.999"))
        assertTrue(newer("v0.10.0", "0.9.0"))
        assertEquals(false, newer("v0.1.9", "0.1.13"))
    }

    @Test
    fun equalAndOlderVersionsAreNotNewer() {
        assertEquals(false, newer("v0.1.0", "0.1.0"))
        assertEquals(false, newer("v0.1.0", "0.1.1"))
        assertEquals(false, newer("v0.9.9", "1.0.0"))
    }

    @Test
    fun aTagMustBeVPrefixedMajorMinorPatch() {
        assertEquals(AppVersion(0, 1, 13), parseTag("v0.1.13"))
        for (tag in listOf(
            "0.1.13",
            "V0.1.13",
            "v0.1",
            "v0.1.2.3",
            "v0.1.x",
            "v01.2.3",
            "v0.1.2-rc1",
            "v0.1.2+b",
            " v0.1.2",
            "v0.1.2\n",
            "vv0.1.2",
            "",
            "latest",
            "v1.2.-3",
            "v9999999999.0.0",
        )) {
            assertNull(tag, parseTag(tag))
        }
    }

    @Test
    fun theRunningVersionNameIsReadWithoutThePrefix() {
        assertEquals(AppVersion(0, 1, 0), parseVersionName("0.1.0"))
        assertNull(parseVersionName("v0.1.0"))
        assertNull(parseVersionName("0.1.0-debug"))
        assertEquals("0.2.0", AppVersion(0, 2, 0).toString())
    }

    private fun json(
        tag: String = "v0.2.0",
        extra: String = "",
        assets: String =
            """[{"name":"swarm-viewer_v0.2.0_android_universal.apk","browser_download_url":"https://github.com/x/apk"},""" +
                """{"name":"SHA256SUMS","browser_download_url":"https://github.com/x/sums"}]""",
    ) = """{"tag_name":"$tag"$extra,"assets":$assets}"""

    @Test
    fun aLatestReleaseIsReadWithItsAssets() {
        val release = parseLatestRelease(json())!!
        assertEquals("v0.2.0", release.tag)
        assertEquals(AppVersion(0, 2, 0), release.version)
        assertEquals("https://github.com/x/apk", release.assets[androidAssetName("v0.2.0")])
        assertEquals("https://github.com/x/sums", release.assets[SHA256SUMS_ASSET])
    }

    @Test
    fun draftAndPrereleaseAreIgnored() {
        assertNull(parseLatestRelease(json(extra = ""","draft":true""")))
        assertNull(parseLatestRelease(json(extra = ""","prerelease":true""")))
        assertNotNull(parseLatestRelease(json(extra = ""","draft":false,"prerelease":false""")))
    }

    @Test
    fun anUnparsableTagIsIgnoredWithoutError() {
        for (tag in listOf("nightly", "v0.2", "0.2.0", "v0.2.0-rc1", "v00.2.0")) assertNull(tag, parseLatestRelease(json(tag = tag)))
        assertNull(parseLatestRelease("""{"assets":[]}"""))
        assertNull(parseLatestRelease("""{"tag_name":7}"""))
    }

    @Test
    fun malformedJsonIsANetworkFailure() {
        for (body in listOf("", "not json", "[1]", "null", "{")) {
            assertEquals(body, UpdateFailure.Network, assertThrows(UpdateException::class.java) { parseLatestRelease(body) }.failure)
        }
    }

    @Test
    fun malformedAssetEntriesAreSkipped() {
        val release =
            parseLatestRelease(
                json(assets = """[1,{"name":"a"},{"browser_download_url":"u"},{"name":"b","browser_download_url":"u2"}]"""),
            )!!
        assertEquals(mapOf("b" to "u2"), release.assets)
    }

    private val apk = "swarm-viewer_v0.2.0_android_universal.apk"
    private val hash = "a".repeat(64)

    @Test
    fun theChecksumOfExactlyThatAssetIsFound() {
        val sums = "${"b".repeat(
            64,
        )}  swarm-viewer_v0.2.0_windows_amd64_installer.exe\n${hash.uppercase()}  $apk\n\n${"c".repeat(64)} *other\r\n"
        assertEquals(hash, expectedSha256(sums, apk))
    }

    @Test
    fun aMissingDuplicateOrMalformedEntryIsRejected() {
        val other = "${"b".repeat(64)}  other.bin"
        val cases =
            mapOf(
                "empty" to "",
                "missing" to other,
                "duplicate" to "$hash  $apk\n${"c".repeat(64)}  $apk",
                "short hash" to "${"a".repeat(63)}  $apk",
                "non hex" to "${"g".repeat(64)}  $apk",
                "no name" to hash,
                "one space only and a name with prefix" to "$hash  x$apk",
                "garbage line next to a good one" to "$hash  $apk\nthis is not a checksum line",
            )
        for ((name, sums) in cases) {
            assertEquals(
                name,
                UpdateFailure.ChecksumInvalid,
                assertThrows(name, UpdateException::class.java) { expectedSha256(sums, apk) }.failure,
            )
        }
    }
}
