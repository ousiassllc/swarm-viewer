package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import java.time.DateTimeException
import java.time.Instant
import java.time.OffsetDateTime

// swarm-console の応答を読む型付きの読み取り。デスクトップ版（Go の encoding/json で struct に読む）と同じ規則:
//  - キーの欠落・null は既定値（NFR-CMP-2）。未知のキーは無視する（NFR-CMP-1）
//  - 型が違う値（文字列の項目に数値、配列の項目にオブジェクトなど）は [InvalidResponseException]
// エラーに応答の本文は含めない（NFR-LOG-3）。

/** 応答の JSON が壊れている・形が想定外のとき。[classify] は [ErrKind.Protocol] に分類する。メッセージは固定で、応答本文を含まない。 */
class InvalidResponseException : Exception("swarm-console: invalid response")

internal fun invalid(): Nothing = throw InvalidResponseException()

/** 本文全体が JSON のオブジェクトであることを確かめて読む。 */
internal fun parseJsonObject(data: String): JsonObject {
    if (!data.trimGoSpace().startsWith('{')) invalid()
    val element =
        try {
            Json.parseToJsonElement(data)
        } catch (e: IllegalArgumentException) {
            // SerializationException（壊れた JSON）も IllegalArgumentException の一種。例外の本文（入力の一部）は捨てる。
            invalid()
        }
    return element as? JsonObject ?: invalid()
}

internal fun JsonElement?.objectOrNull(): JsonObject? =
    when (this) {
        null, JsonNull -> null
        is JsonObject -> this
        else -> invalid()
    }

internal fun JsonElement?.listOrEmpty(): List<JsonElement> =
    when (this) {
        null, JsonNull -> emptyList()
        is JsonArray -> this
        else -> invalid()
    }

internal fun JsonElement?.stringOrNull(): String? =
    when (this) {
        null, JsonNull -> null
        is JsonPrimitive -> if (isString) content else invalid()
        else -> invalid()
    }

internal fun JsonElement?.string(): String = stringOrNull().orEmpty()

internal fun JsonElement?.bool(): Boolean =
    when (this) {
        null, JsonNull -> false
        is JsonPrimitive -> if (!isString && (content == "true" || content == "false")) content == "true" else invalid()
        else -> invalid()
    }

internal fun JsonElement?.doubleOrNull(): Double? =
    when (this) {
        null, JsonNull -> null
        is JsonPrimitive -> if (isString) invalid() else content.toDoubleOrNull()?.takeIf { it.isFinite() } ?: invalid()
        else -> invalid()
    }

internal fun JsonElement?.double(): Double = doubleOrNull() ?: 0.0

/** 整数の項目。`2.5`・`1e2` のような整数でない表記は不正（Go の int と同じ）。 */
internal fun JsonElement?.int(): Int =
    when (this) {
        null, JsonNull -> 0
        is JsonPrimitive -> if (isString) invalid() else content.toIntOrNull() ?: invalid()
        else -> invalid()
    }

/** RFC 3339 の時刻。null・欠落は null。 */
internal fun JsonElement?.instantOrNull(): Instant? {
    val text = stringOrNull() ?: return null
    return try {
        OffsetDateTime.parse(text).toInstant()
    } catch (e: DateTimeException) {
        invalid()
    }
}
