package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter

private val timeFormat = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'").withZone(ZoneOffset.UTC)

private fun obj(vararg pairs: Pair<String, JsonElement>) = JsonObject(mapOf(*pairs))

private fun text(value: String?): JsonElement = if (value == null) JsonNull else JsonPrimitive(value)

private fun time(value: Instant?): JsonElement = if (value == null) JsonNull else JsonPrimitive(timeFormat.format(value))

private fun <T> list(
    values: List<T>,
    transform: (T) -> JsonElement,
) = JsonArray(values.map(transform))

/** Report を testdata/vectors/README.md の「正規化した Report」にする（テスト用）。 */
internal fun normalizedReport(report: Report): JsonElement =
    obj(
        "at" to time(report.at),
        "layout" to obj("rows" to JsonPrimitive(report.layout.rows), "cols" to list(report.layout.cols) { JsonPrimitive(it) }),
        "panes" to list(report.panes, ::pane),
        "attention" to
            list(report.attention) {
                obj("kind" to text(it.kind), "pane" to text(it.pane), "detail" to text(it.detail))
            },
        "warnings" to list(report.warnings) { obj("code" to text(it.code), "message" to text(it.message)) },
        "host" to host(report.host),
        "usage" to usage(report.usage),
        "auth" to
            obj(
                "agents" to
                    list(report.auth.agents) {
                        obj("agent" to text(it.agent), "status" to text(it.status), "expires_at" to time(it.expiresAt))
                    },
            ),
    )

private fun pane(p: Pane): JsonElement =
    obj(
        "pos" to text(p.pos),
        "id" to text(p.id),
        "agent" to text(p.agent),
        "state" to text(p.state.id),
        "title" to text(p.title),
        "cwd" to text(p.cwd),
        "elapsed" to text(p.elapsed),
        "activity" to text(p.activity),
        "truncated" to JsonPrimitive(p.activityTruncated),
        "changed_ago" to text(p.changedAgo),
        "note" to
            (
                p.note?.let {
                    obj(
                        "from" to text(it.from),
                        "repo" to text(it.repo),
                        "items" to list(it.items, ::text),
                        "next" to text(it.next),
                    )
                } ?: JsonNull
            ),
    )

private fun mem(m: MemUsage): JsonElement =
    obj(
        "available" to JsonPrimitive(m.available),
        "total_gb" to JsonPrimitive(m.totalGb),
        "used_gb" to JsonPrimitive(m.usedGb),
        "used_percent" to JsonPrimitive(m.usedPercent),
    )

private fun host(h: Host): JsonElement =
    obj(
        "cpu" to obj("available" to JsonPrimitive(h.cpu.available), "used_percent" to JsonPrimitive(h.cpu.usedPercent)),
        "mem" to mem(h.mem),
        "swap" to mem(h.swap),
        "disk" to
            list(h.disk) {
                obj(
                    "mount" to text(it.mount),
                    "available" to JsonPrimitive(it.available),
                    "free_gb" to JsonPrimitive(it.freeGb),
                    "total_gb" to JsonPrimitive(it.totalGb),
                    "used_percent" to JsonPrimitive(it.usedPercent),
                )
            },
    )

private fun slot(s: Slot): JsonElement =
    obj(
        "available" to JsonPrimitive(s.available),
        "used" to JsonPrimitive(s.used),
        "unit" to text(s.unit),
        "status" to text(s.status),
        "resets_at" to time(s.resetsAt),
    )

private fun usage(u: Usage): JsonElement =
    obj(
        "status" to text(u.status),
        "reason" to text(u.reason),
        "fetched_at" to time(u.fetchedAt),
        "claude" to list(u.claude, ::agentAccount),
        "codex" to list(u.codex, ::agentAccount),
        "cursor" to
            list(u.cursor) {
                obj(
                    "account" to text(it.account),
                    "window" to text(it.window),
                    "models" to slot(it.models),
                    "reason" to text(it.reason),
                )
            },
    )

private fun agentAccount(a: AgentAccount): JsonElement =
    obj(
        "account" to text(a.account),
        "five_hour" to slot(a.fiveHour),
        "seven_day" to slot(a.sevenDay),
        "reason" to text(a.reason),
    )
