package com.ousiassllc.swarmviewer.domain

// Go の strings.TrimSpace・unicode.IsSpace・unicode.IsControl と同じ判定（デスクトップ版との挙動の一致のため）。
// Kotlin 標準の isWhitespace は NEL（U+0085）を含まず、NBSP（U+00A0）の扱いも Go と異なる。

private val spaceCodePoints =
    intArrayOf(
        0x09,
        0x0A,
        0x0B,
        0x0C,
        0x0D,
        0x20,
        0x85,
        0xA0,
        0x1680,
        0x2028,
        0x2029,
        0x202F,
        0x205F,
        0x3000,
    )

internal fun isGoSpace(c: Char): Boolean = c.code in spaceCodePoints || c.code in 0x2000..0x200A

/** Unicode の制御文字（カテゴリ Cc: U+0000〜U+001F、U+007F〜U+009F）。 */
internal fun isGoControl(c: Char): Boolean = Character.getType(c) == Character.CONTROL.toInt()

internal fun String.trimGoSpace(): String = trim(::isGoSpace)

internal fun String.hasControl(): Boolean = any(::isGoControl)

/** UTF-8 にしたときのバイト数。対になっていないサロゲートを含む文字列は null（Go の utf8.ValidString が偽）。 */
internal fun String.utf8LengthOrNull(): Int? {
    var length = 0
    var i = 0
    while (i < this.length) {
        val c = this[i]
        when {
            c.code < 0x80 -> length += 1
            c.code < 0x800 -> length += 2
            Character.isHighSurrogate(c) -> {
                if (i + 1 >= this.length || !Character.isLowSurrogate(this[i + 1])) return null
                length += 4
                i++
            }
            Character.isLowSurrogate(c) -> return null
            else -> length += 3
        }
        i++
    }
    return length
}
