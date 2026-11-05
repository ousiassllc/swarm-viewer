package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.JsonObject

/**
 * `report` 応答（JSON）を [Report] に変換する（API 仕様 §1.4）。
 * 未知のキーは無視し、欠落・null は既定値、未知の state は [PaneState.Unknown]、未知の usage.status は unavailable にする。
 * JSON として壊れている・オブジェクトでない・型が想定外のときは [InvalidResponseException]。
 * デスクトップ版の ParseReport と同じ結果になる（testdata/vectors/report.json）。
 */
fun parseReport(data: String): Report {
    val root = parseJsonObject(data)
    return Report(
        at = root["at"].instantOrNull(),
        layout = root["layout"].objectOrNull().toLayout(),
        panes = root["panes"].listOrEmpty().map { it.objectOrNull().toPane() },
        attention = root["attention"].listOrEmpty().map { it.objectOrNull().toAttention() },
        usage = root["usage"].objectOrNull().toUsage(),
        host = root["host"].objectOrNull().toHost(),
        warnings = root["warnings"].listOrEmpty().map { it.objectOrNull().toWarning() },
        auth = root["auth"].objectOrNull().toAuth(),
    )
}

/**
 * `sessions_list` 応答を許可されたセッション（`allowed: true`）だけに絞る。順序は応答のまま。
 * id が空・重複のものは捨てる。0 件のときは空リスト。JSON が壊れている・`sessions` が配列でないときは [InvalidResponseException]。
 */
fun parseSessions(data: String): List<Session> {
    val root = parseJsonObject(data)
    val sessions = root["sessions"]
    if (sessions == null || sessions is kotlinx.serialization.json.JsonNull) invalid()
    val seen = HashSet<String>()
    return sessions
        .listOrEmpty()
        .map { it.objectOrNull() }
        .map { Session(id = it?.get("id").string(), allowed = it?.get("allowed").bool()) }
        .filter { it.allowed && it.id.isNotEmpty() && seen.add(it.id) }
}

private fun JsonObject?.toLayout(): Layout =
    if (this == null) Layout() else Layout(rows = get("rows").int(), cols = get("cols").listOrEmpty().map { it.int() })

private fun JsonObject?.toNote(): Note? =
    this?.let {
        Note(
            from = it["from"].string(),
            repo = it["repo"].string(),
            items = it["items"].listOrEmpty().map { item -> item.string() },
            next = it["next"].string(),
        )
    }

private fun JsonObject?.toPane(): Pane {
    if (this == null) return Pane()
    return Pane(
        pos = get("pos").string(),
        id = get("id").string(),
        agent = get("agent").stringOrNull(),
        // 未知の値・文字列以外・null はすべて unknown（エラーにしない。NFR-CMP-1・2）。
        state = PaneState.parse((get("state") as? kotlinx.serialization.json.JsonPrimitive)?.takeIf { it.isString }?.content),
        title = get("title").string(),
        cwd = get("cwd").string(),
        elapsed = get("elapsed").string(),
        activity = get("activity").string(),
        activityTruncated = get("truncated").bool(),
        changedAgo = get("changed_ago").string(),
        note = get("note").objectOrNull().toNote(),
    )
}

private fun JsonObject?.toAttention(): Attention =
    if (this == null) Attention() else Attention(kind = get("kind").string(), pane = get("pane").string(), detail = get("detail").string())

private fun JsonObject?.toWarning(): Warning =
    if (this == null) Warning() else Warning(code = get("code").string(), message = get("message").string())

private fun JsonObject?.toCpu(): Cpu =
    if (this == null) Cpu() else Cpu(available = get("available").bool(), usedPercent = get("used_percent").double())

private fun JsonObject?.toMemUsage(): MemUsage =
    if (this == null) {
        MemUsage()
    } else {
        MemUsage(
            available = get("available").bool(),
            totalGb = get("total_gb").double(),
            usedGb = get("used_gb").double(),
            usedPercent = get("used_percent").double(),
        )
    }

private fun JsonObject?.toDisk(): Disk =
    if (this == null) {
        Disk()
    } else {
        Disk(
            mount = get("mount").string(),
            available = get("available").bool(),
            freeGb = get("free_gb").double(),
            totalGb = get("total_gb").double(),
            usedPercent = get("used_percent").double(),
        )
    }

private fun JsonObject?.toHost(): Host =
    if (this == null) {
        Host()
    } else {
        Host(
            cpu = get("cpu").objectOrNull().toCpu(),
            mem = get("mem").objectOrNull().toMemUsage(),
            swap = get("swap").objectOrNull().toMemUsage(),
            disk = get("disk").listOrEmpty().map { it.objectOrNull().toDisk() },
        )
    }

private fun JsonObject?.toAuth(): Auth =
    if (this == null) {
        Auth()
    } else {
        Auth(
            agents =
                get("agents").listOrEmpty().map { element ->
                    val agent = element.objectOrNull()
                    AuthAgent(
                        agent = agent?.get("agent").string(),
                        status = agent?.get("status").string(),
                        expiresAt = agent?.get("expires_at").instantOrNull(),
                    )
                },
        )
    }
