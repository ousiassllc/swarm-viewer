package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Cpu
import com.ousiassllc.swarmviewer.domain.Disk
import com.ousiassllc.swarmviewer.domain.Host
import com.ousiassllc.swarmviewer.domain.MemUsage
import com.ousiassllc.swarmviewer.domain.Note
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.Tone
import com.ousiassllc.swarmviewer.domain.percentTone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant
import java.time.ZoneOffset

class FormatTest {
    @Test
    fun percentIsRoundedToOneDecimalWithoutTrailingZero() {
        assertEquals("80%", formatPercent(80.0))
        assertEquals("79.9%", formatPercent(79.9))
        assertEquals("12.3%", formatPercent(12.34))
        assertEquals("0%", formatPercent(0.0))
        assertEquals(DASH, formatPercent(Double.NaN))
        assertEquals(DASH, formatPercent(Double.POSITIVE_INFINITY))
    }

    @Test
    fun usdAndGbAreFormatted() {
        assertEquals("$12.50", formatUsd(12.5))
        assertEquals(DASH, formatUsd(Double.NaN))
        assertEquals("12.3 GB", formatGb(12.34))
        assertEquals("128 GB", formatGb(128.0))
        assertEquals(DASH, formatGb(Double.NaN))
    }

    @Test
    fun barFractionClampsToZeroToOne() {
        assertEquals(0f, barFraction(Double.NaN), 0f)
        assertEquals(0f, barFraction(-5.0), 0f)
        assertEquals(0.5f, barFraction(50.0), 0f)
        assertEquals(1f, barFraction(250.0), 0f)
    }

    @Test
    fun timesAreFormattedInTheGivenZone() {
        val at = Instant.parse("2026-11-05T01:02:00Z")
        val zone = ZoneOffset.ofHours(9)
        assertEquals("10:02", formatClock(at, zone))
        assertEquals("11/05 10:02", formatDateTime(at, zone))
        assertEquals("2026/11/05 10:02", formatDateTimeWithYear(at, zone))
    }

    @Test
    fun untilTruncatesAndSaysSoonForLessThanAMinuteOrThePast() {
        val now = Instant.parse("2026-11-05T00:00:00Z")
        assertEquals("まもなく", formatUntil(now.minusSeconds(60), now))
        assertEquals("まもなく", formatUntil(now.plusSeconds(59), now))
        assertEquals("あと 1 分", formatUntil(now.plusSeconds(119), now))
        assertEquals("あと 59 分", formatUntil(now.plusSeconds(59 * 60 + 59), now))
        assertEquals("あと 1 時間", formatUntil(now.plusSeconds(3600 + 3599), now))
        assertEquals("あと 23 時間", formatUntil(now.plusSeconds(24 * 3600 - 1), now))
        assertEquals("あと 3 日", formatUntil(now.plusSeconds(3 * 24 * 3600 + 5), now))
    }

    @Test
    fun attentionLabelsFollowTheSpecAndUnknownKindsAreShownAsIs() {
        assertEquals("答え待ち", attentionLabel("asking"))
        assertEquals("止まってるかも", attentionLabel("stalled"))
        assertEquals("報告待ち", attentionLabel("report_waiting"))
        assertEquals("ログイン期限切れ", attentionLabel("auth_expired"))
        assertEquals("ログイン期限間近", attentionLabel("auth_expiring"))
        assertEquals("brand_new", attentionLabel("brand_new"))
        assertEquals(DisplayTone.Muted, attentionTone("brand_new"))
        assertEquals("", attentionDetail("report_waiting", "/path/on/server")) // サーバー上のパスは出さない
        assertEquals("q", attentionDetail("asking", "q"))
    }

    @Test
    fun usageStatusDecidesTheTextAndWhetherNumbersAreShown() {
        assertEquals("", usageStatusLabel("ok"))
        assertEquals("古いデータ", usageStatusLabel("stale"))
        assertEquals("取得中", usageStatusLabel("pending"))
        assertEquals("利用上限の取得は無効です", usageStatusLabel("disabled"))
        assertEquals("取得できません", usageStatusLabel("unavailable"))
        assertEquals("取得できません", usageStatusLabel("something-new")) // 未知は unavailable
        assertTrue(usageShowsNumbers("ok") && usageShowsNumbers("stale"))
        assertFalse(
            usageShowsNumbers("pending") || usageShowsNumbers("unavailable") || usageShowsNumbers("disabled") || usageShowsNumbers(""),
        )
    }

    @Test
    fun usageReasonsAreHedgedAndUnknownOnesAreIgnored() {
        assertEquals("ログイン切れの可能性", usageReasonLabel("without_usage"))
        assertEquals("アカウントを特定できません（ログイン切れの可能性）", usageReasonLabel("no_email"))
        assertEquals("", usageReasonLabel(null))
        assertEquals("", usageReasonLabel("whatever"))
        assertEquals("月間", cursorWindowLabel("monthly"))
        assertEquals("週間", cursorWindowLabel("weekly"))
        assertEquals("", cursorWindowLabel(null))
        assertEquals("daily", cursorWindowLabel("daily"))
    }

    @Test
    fun noteSummaryKeepsTheFirstThreeItemsAndCountsTheRest() {
        val summary = noteSummary(Note(repo = "r", items = listOf("1", "2", "3", "4", "5")))
        assertEquals(listOf("1", "2", "3"), summary.items)
        assertEquals(2, summary.more)
        assertTrue(noteSummary(null).isEmpty)
        assertTrue(noteSummary(Note(from = "x", next = "y")).isEmpty) // repo と items だけ表示する
    }

    @Test
    fun stateToneMatchesTheSpec() {
        assertEquals(DisplayTone.Accent, stateTone(PaneState.Working))
        assertEquals(DisplayTone.Warning, stateTone(PaneState.Asking))
        assertEquals(DisplayTone.Success, stateTone(PaneState.Idle))
        assertEquals(DisplayTone.Error, stateTone(PaneState.Stalled))
        assertEquals(DisplayTone.Muted, stateTone(PaneState.Shell))
        assertEquals(DisplayTone.Muted, stateTone(PaneState.Unknown))
    }

    @Test
    fun thresholdColorsComeFromTheDomainDecision() {
        assertEquals(DisplayTone.Accent, displayTone(percentTone(79.9)))
        assertEquals(DisplayTone.Warning, displayTone(percentTone(80.0)))
        assertEquals(DisplayTone.Warning, displayTone(percentTone(94.9)))
        assertEquals(DisplayTone.Error, displayTone(percentTone(95.0)))
        assertEquals(DisplayTone.Error, displayTone(Tone.Error))
    }

    @Test
    fun hostRowsAreCpuMemorySwapThenOneRowPerDisk() {
        val rows =
            hostRows(
                Host(
                    cpu = Cpu(available = true, usedPercent = 42.0),
                    mem = MemUsage(available = true, totalGb = 128.0, usedGb = 41.3, usedPercent = 32.3),
                    swap = MemUsage(available = true, totalGb = 0.0),
                    disk =
                        listOf(
                            Disk(mount = "/", available = true, freeGb = 108.5, totalGb = 500.0, usedPercent = 78.3),
                            Disk(mount = "/mnt", available = false),
                        ),
                ),
            )

        assertEquals(listOf("CPU", "メモリ", "スワップ", "/", "/mnt"), rows.map { it.label })
        assertEquals("41.3 GB / 128 GB", rows[1].detail)
        assertTrue(rows[2].noBar) // スワップ無しは「なし」
        assertEquals("なし", rows[2].detail)
        assertEquals("108.5 GB 空き", rows[3].detail)
        assertFalse(rows[4].available) // 取れないマウントは「—」
    }

    @Test
    fun hostRowsShowDashesForUnavailableValuesWithoutHidingTheRows() {
        val rows = hostRows(Host()) // host.mem を返さない古い swarm-console
        assertEquals(listOf("CPU", "メモリ", "スワップ"), rows.map { it.label })
        assertTrue(rows.none { it.available })
    }
}
