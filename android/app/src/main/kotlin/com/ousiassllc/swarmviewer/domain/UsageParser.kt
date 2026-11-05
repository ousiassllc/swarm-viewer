package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.JsonObject

// `report` の usage の読み取り。規則は ReportParser.kt と同じ（欠落・null は既定値、型違いは [InvalidResponseException]）。

internal fun JsonObject?.toUsage(): Usage =
    if (this == null) {
        Usage()
    } else {
        Usage(
            status = UsageStatus.normalize(get("status").string()),
            reason = get("reason").string(),
            fetchedAt = get("fetched_at").instantOrNull(),
            claude = get("claude").listOrEmpty().map { it.objectOrNull().toAgentAccount() },
            codex = get("codex").listOrEmpty().map { it.objectOrNull().toAgentAccount() },
            cursor = get("cursor").listOrEmpty().map { it.objectOrNull().toCursorAccount() },
        )
    }

private fun JsonObject?.toAgentAccount(): AgentAccount =
    if (this == null) {
        AgentAccount()
    } else {
        AgentAccount(
            account = get("account").string(),
            fiveHour = get("five_hour").objectOrNull().toSlot(),
            sevenDay = get("seven_day").objectOrNull().toSlot(),
            reason = get("reason").stringOrNull(),
        )
    }

private fun JsonObject?.toCursorAccount(): CursorAccount =
    if (this == null) {
        CursorAccount()
    } else {
        CursorAccount(
            account = get("account").string(),
            window = get("window").stringOrNull(),
            models = get("models").objectOrNull().toSlot(),
            reason = get("reason").stringOrNull(),
        )
    }

private fun JsonObject?.toSlot(): Slot =
    if (this == null) {
        Slot()
    } else {
        Slot(
            available = get("available").bool(),
            used = get("used").doubleOrNull(),
            unit = get("unit").string(),
            status = get("status").string(),
            resetsAt = get("resets_at").instantOrNull(),
        )
    }
