package com.ousiassllc.swarmviewer.data.store

import com.ousiassllc.swarmviewer.domain.AgentAccount
import com.ousiassllc.swarmviewer.domain.Auth
import com.ousiassllc.swarmviewer.domain.CursorAccount
import com.ousiassllc.swarmviewer.domain.Disk
import com.ousiassllc.swarmviewer.domain.Host
import com.ousiassllc.swarmviewer.domain.MemUsage
import com.ousiassllc.swarmviewer.domain.Note
import com.ousiassllc.swarmviewer.domain.Pane
import com.ousiassllc.swarmviewer.domain.Report
import com.ousiassllc.swarmviewer.domain.Slot
import com.ousiassllc.swarmviewer.domain.Usage
import com.ousiassllc.swarmviewer.domain.parseReport
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonObjectBuilder
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonObject

/** `snapshots.report_json` の形式版。読めない版は「スナップショットなし」として扱う（ER §3.3）。 */
const val SNAPSHOT_SCHEMA_VERSION = 1

/**
 * domain の [Report] を `report_json`（swarm-console `report` と同じキーの JSON）にする。viewer が使う項目だけを持ち、
 * 未知のキーは保存しない。読み戻しは domain の [parseReport] なので、読み方の規則（欠落は既定値など）が 1 か所に保たれる。
 */
internal fun encodeReport(report: Report): String =
    buildJsonObject {
        put("at", report.at?.toString())
        putJsonObject("layout") {
            put("rows", report.layout.rows)
            put("cols", JsonArray(report.layout.cols.map { JsonPrimitive(it) }))
        }
        put("panes", JsonArray(report.panes.map { it.toJson() }))
        put(
            "attention",
            JsonArray(
                report.attention.map {
                    buildJsonObject {
                        put("kind", it.kind)
                        put("pane", it.pane)
                        put("detail", it.detail)
                    }
                },
            ),
        )
        put("usage", report.usage.toJson())
        put("host", report.host.toJson())
        put(
            "warnings",
            JsonArray(
                report.warnings.map {
                    buildJsonObject {
                        put("code", it.code)
                        put("message", it.message)
                    }
                },
            ),
        )
        put("auth", report.auth.toJson())
    }.toString()

/** 保存した `report_json` を読み戻す。壊れている（[com.ousiassllc.swarmviewer.domain.InvalidResponseException]）ときは例外。 */
internal fun decodeReport(json: String): Report = parseReport(json)

private fun Pane.toJson(): JsonObject =
    buildJsonObject {
        put("pos", pos)
        put("id", id)
        put("agent", agent)
        put("state", state.id)
        put("title", title)
        put("cwd", cwd)
        put("elapsed", elapsed)
        put("activity", activity)
        put("truncated", activityTruncated)
        put("changed_ago", changedAgo)
        put("note", note?.toJson())
    }

private fun JsonObjectBuilder.put(
    key: String,
    value: JsonObject?,
) = put(key, value ?: kotlinx.serialization.json.JsonNull)

private fun Note.toJson(): JsonObject =
    buildJsonObject {
        put("from", from)
        put("repo", repo)
        put("items", JsonArray(items.map { JsonPrimitive(it) }))
        put("next", next)
    }

private fun Slot.toJson(): JsonObject =
    buildJsonObject {
        put("available", available)
        put("used", used)
        put("unit", unit)
        put("status", status)
        put("resets_at", resetsAt?.toString())
    }

private fun AgentAccount.toJson(): JsonObject =
    buildJsonObject {
        put("account", account)
        put("five_hour", fiveHour.toJson())
        put("seven_day", sevenDay.toJson())
        put("reason", reason)
    }

private fun CursorAccount.toJson(): JsonObject =
    buildJsonObject {
        put("account", account)
        put("window", window)
        put("models", models.toJson())
        put("reason", reason)
    }

private fun Usage.toJson(): JsonObject =
    buildJsonObject {
        put("status", status)
        put("reason", reason)
        put("fetched_at", fetchedAt?.toString())
        put("claude", buildJsonArray { claude.forEach { add(it.toJson()) } })
        put("codex", buildJsonArray { codex.forEach { add(it.toJson()) } })
        put("cursor", buildJsonArray { cursor.forEach { add(it.toJson()) } })
    }

private fun MemUsage.toJson(): JsonObject =
    buildJsonObject {
        put("available", available)
        put("total_gb", totalGb)
        put("used_gb", usedGb)
        put("used_percent", usedPercent)
    }

private fun Disk.toJson(): JsonObject =
    buildJsonObject {
        put("mount", mount)
        put("available", available)
        put("free_gb", freeGb)
        put("total_gb", totalGb)
        put("used_percent", usedPercent)
    }

private fun Host.toJson(): JsonObject =
    buildJsonObject {
        put(
            "cpu",
            buildJsonObject {
                put("available", cpu.available)
                put("used_percent", cpu.usedPercent)
            },
        )
        put("mem", mem.toJson())
        put("swap", swap.toJson())
        put("disk", buildJsonArray { disk.forEach { add(it.toJson()) } })
    }

private fun Auth.toJson(): JsonObject =
    buildJsonObject {
        put(
            "agents",
            buildJsonArray {
                agents.forEach {
                    add(
                        buildJsonObject {
                            put("agent", it.agent)
                            put("status", it.status)
                            put("expires_at", it.expiresAt?.toString())
                        },
                    )
                }
            },
        )
    }
