package com.ousiassllc.swarmviewer.domain

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class SettingsAndTextTest {
    @Test
    fun defaultSettingsAreValid() {
        val settings = Settings()
        assertEquals(Settings(true, 10, 15, false, null), settings)
        settings.validate()
    }

    @Test
    fun allowedValues() {
        assertEquals(listOf(5, 10, 30, 60), ALLOWED_SCREEN_POLL_SECONDS)
        // WorkManager の周期ジョブの最小間隔が 15 分のため、5・10 分は持たない。
        assertEquals(listOf(15, 30, 60), ALLOWED_NOTIFY_POLL_MINUTES)
        for (n in ALLOWED_SCREEN_POLL_SECONDS) Settings(screenPollSeconds = n).validate()
        for (n in ALLOWED_NOTIFY_POLL_MINUTES) Settings(notifyPollMinutes = n).validate()
    }

    @Test
    fun invalidValuesNameOnlyTheFields() {
        val e = assertThrows(SettingsException::class.java) { Settings(screenPollSeconds = 7, notifyPollMinutes = 10).validate() }
        assertEquals(listOf(FIELD_SCREEN_POLL_SECONDS, FIELD_NOTIFY_POLL_MINUTES), e.fields)
        assertEquals("invalid settings: screen_poll_seconds, notify_poll_minutes", e.message)
        for (n in listOf(-5, 0, 1, 5, 10, 14, 16, 120)) {
            assertThrows(SettingsException::class.java) { Settings(notifyPollMinutes = n).validate() }
        }
    }

    @Test
    fun spaceAndControlFollowGo() {
        // Go の unicode.IsSpace: ASCII の空白、NEL、NBSP、U+1680、U+2000〜U+200A、U+2028・2029、U+202F、U+205F、U+3000。
        for (code in listOf(
            0x09,
            0x0A,
            0x0B,
            0x0C,
            0x0D,
            0x20,
            0x85,
            0xA0,
            0x1680,
            0x2000,
            0x200A,
            0x2028,
            0x2029,
            0x202F,
            0x205F,
            0x3000,
        )) {
            assertTrue("U+%04X は空白".format(code), isGoSpace(code.toChar()))
        }
        // ゼロ幅スペース（U+200B）・U+FEFF・文字は空白でない。
        for (c in listOf('\u200B', '\uFEFF', 'a', '0', '\u0000')) assertFalse("$c", isGoSpace(c))
        assertTrue(isGoControl('\u0000'))
        assertTrue(isGoControl('\u009F'))
        assertTrue(isGoControl('\u007F'))
        assertFalse(isGoControl(' '))
        assertFalse(isGoControl('\u00A0'))
        assertEquals("a b", "\u00A0\u3000a b\u0085\n".trimGoSpace())
    }

    @Test
    fun utf8LengthCountsBytesAndRejectsLoneSurrogates() {
        assertEquals(0, "".utf8LengthOrNull())
        assertEquals(3, "abc".utf8LengthOrNull())
        assertEquals(2, "\u00E9".utf8LengthOrNull())
        assertEquals(3, "あ".utf8LengthOrNull())
        assertEquals(4, "😀".utf8LengthOrNull())
        assertNull("\uD83D".utf8LengthOrNull())
        assertNull("a\uDE00".utf8LengthOrNull())
        assertNull("\uD83Da".utf8LengthOrNull())
    }

    @Test
    fun pairingRejectsLoneSurrogates() {
        val e = assertThrows(PairingException::class.java) { parsePairing("swarm-viewer://pair?v=1&name=\uD83D") }
        assertEquals(PairingError.Format, e.error)
    }

    @Test
    fun ipAddressFormatting() {
        assertEquals("::", IpAddress.parse("0:0:0:0:0:0:0:0").toString())
        assertEquals("1::", IpAddress.parse("1:0:0:0:0:0:0:0").toString())
        assertEquals("1:0:2::3:0:4", IpAddress.parse("1:0:2:0:0:3:0:4").toString())
        assertEquals("1:0:2:3:4:5:6:7", IpAddress.parse("1:0:2:3:4:5:6:7").toString()) // 1 グループだけの 0 は省略しない
        assertEquals("fd7a:115c:a1e0:1::1", IpAddress.parse("fd7a:115c:a1e0:0001:0000:0000:0000:0001").toString())
        assertEquals("1:0:0:2::3", IpAddress.parse("1:0:0:2:0:0:0:3").toString()) // 最も長い連続を省略
        assertEquals("1::2:0:0:3:0", IpAddress.parse("1:0:0:2:0:0:3:0").toString()) // 同じ長さなら先頭側を省略
        assertTrue(IpAddress.parse("::ffff:1.2.3.4")!!.isV4InV6)
        assertNull(IpAddress.parse("1.2.3"))
        assertNull(IpAddress.parse("1:2:3:4:5:6:7"))
        assertNull(IpAddress.parse("1:2:3:4:5:6:7:8:9"))
        assertNull(IpAddress.parse("1::2::3"))
        assertNull(IpAddress.parse("12345::"))
        assertNull(IpAddress.parse("::g"))
        assertNull(IpAddress.parse(":1"))
        assertNull(IpAddress.parse("1:"))
        assertNull(IpAddress.parse("fe80::1%eth0"))
    }
}
