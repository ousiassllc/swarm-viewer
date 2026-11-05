package com.ousiassllc.swarmviewer.domain

import java.time.Instant

/** `usage.status` の値。これ以外は [UNAVAILABLE] として扱う。 */
object UsageStatus {
    const val OK = "ok"
    const val STALE = "stale"
    const val UNAVAILABLE = "unavailable"
    const val PENDING = "pending"
    const val DISABLED = "disabled"

    /** 未知・空の `usage.status` を [UNAVAILABLE] にする。 */
    fun normalize(status: String): String =
        when (status) {
            OK, STALE, UNAVAILABLE, PENDING, DISABLED -> status
            else -> UNAVAILABLE
        }
}

/** 枠の単位（[Slot.unit]）と状態（[Slot.status]）。 */
object SlotValues {
    const val UNIT_PERCENT = "percent"
    const val UNIT_USD = "usd"
    const val STATUS_EXHAUSTED = "exhausted"
}

/** 利用上限のしきい値（%）。固定値で設定項目にしない（機能要件 §5.3）。 */
const val WARN_PERCENT = 80.0
const val CRITICAL_PERCENT = 95.0

/** 利用上限の 1 枠。[available] が false のとき、ほかの項目は既定値（表示は「—」）。 */
data class Slot(
    val available: Boolean = false,
    val used: Double? = null,
    val unit: String = "",
    val status: String = "",
    val resetsAt: Instant? = null,
)

/** Claude / Codex のアカウント 1 件。[reason] は枠を取れなかった理由のコード（`without_usage`・`no_email` など）。無ければ null。 */
data class AgentAccount(
    val account: String = "",
    val fiveHour: Slot = Slot(),
    val sevenDay: Slot = Slot(),
    val reason: String? = null,
)

/** Cursor のアカウント 1 件。[window] は `monthly`・`weekly` など。枠が無ければ null。 */
data class CursorAccount(
    val account: String = "",
    val window: String? = null,
    val models: Slot = Slot(),
    val reason: String? = null,
)

/** 利用上限パネルの値（ホスト全体）。 */
data class Usage(
    val status: String = UsageStatus.UNAVAILABLE,
    val reason: String = "",
    val fetchedAt: Instant? = null,
    val claude: List<AgentAccount> = emptyList(),
    val codex: List<AgentAccount> = emptyList(),
    val cursor: List<CursorAccount> = emptyList(),
)

/** 使用率の色の意味。実際の色は ui 層が決める。 */
enum class Tone(
    val id: String,
) {
    Normal("normal"),
    Warning("warning"),
    Error("error"),
}

/**
 * 使用率（%）のしきい値判定。80 未満は通常、80 以上 95 未満は警告、95 以上はエラー。
 * NaN は通常として扱う。ホスト負荷（CPU・スワップ・ディスク）にも同じ規則を使う。
 */
fun percentTone(percent: Double): Tone =
    when {
        percent >= CRITICAL_PERCENT -> Tone.Error
        percent >= WARN_PERCENT -> Tone.Warning
        else -> Tone.Normal
    }

/**
 * 枠の色を返す。
 *  - 取得できない枠（[Slot.available] が false）は通常
 *  - [Slot.status] が exhausted なら used・unit に関わらずエラー
 *  - unit が percent で used があるときだけしきい値で判定する。usd などはしきい値なし
 */
fun usageTone(slot: Slot): Tone {
    val used = slot.used
    return when {
        !slot.available -> Tone.Normal
        slot.status == SlotValues.STATUS_EXHAUSTED -> Tone.Error
        slot.unit != SlotValues.UNIT_PERCENT || used == null -> Tone.Normal
        else -> percentTone(used)
    }
}
