package com.ousiassllc.swarmviewer.ui.main

import com.ousiassllc.swarmviewer.domain.Host
import com.ousiassllc.swarmviewer.domain.MemUsage

/**
 * ホスト負荷の 1 行（機能要件 §6）。[available] が false のときは「—」（バー・数値なし）。[noBar] のときは [detail] だけ出す（スワップ無し）。
 * [label] はマウント名を含む信頼できない入力として、常にテキストで描画する。
 */
data class HostRowModel(
    val label: String,
    val percent: Double,
    val available: Boolean,
    val detail: String = "",
    val noBar: Boolean = false,
)

/** CPU → メモリ → スワップ → ディスク（マウントごとに 1 行）の順。`load1`・`mem_free_gb` は使わない。 */
fun hostRows(host: Host): List<HostRowModel> =
    buildList {
        add(HostRowModel("CPU", host.cpu.usedPercent, host.cpu.available))
        add(memRow("メモリ", host.mem))
        add(swapRow(host.swap))
        host.disk.forEach { disk ->
            add(
                HostRowModel(
                    label = disk.mount,
                    percent = disk.usedPercent,
                    available = disk.available,
                    detail = if (disk.available) "${formatGb(disk.freeGb)} 空き" else "",
                ),
            )
        }
    }

private fun memRow(
    label: String,
    mem: MemUsage,
): HostRowModel =
    HostRowModel(
        label = label,
        percent = mem.usedPercent,
        available = mem.available,
        detail = if (mem.available) "${formatGb(mem.usedGb)} / ${formatGb(mem.totalGb)}" else "",
    )

private fun swapRow(swap: MemUsage): HostRowModel =
    if (swap.available && swap.totalGb == 0.0) {
        HostRowModel("スワップ", swap.usedPercent, available = true, detail = "なし", noBar = true)
    } else {
        memRow("スワップ", swap)
    }
