package com.ousiassllc.swarmviewer.domain

/**
 * IP リテラルの厳密な解析と整形。Go の net/netip（ParseAddr・Addr.String）と同じ規則にする。
 * java.net.InetAddress は名前解決・省略形（`127.1`・10 進の整数）を許すため使わない。
 * ゾーン（`%eth0`）付きは解析できない扱い（null）。
 */
internal class IpAddress private constructor(
    private val bytes: ByteArray,
) {
    val isV4: Boolean get() = bytes.size == V4_SIZE

    /** `::ffff:a.b.c.d`（IPv4 射影アドレス）。 */
    val isV4InV6: Boolean
        get() = !isV4 && (0 until V4_IN_V6_PREFIX).all { bytes[it] == ZERO } && bytes[10] == FF && bytes[11] == FF

    /** tailnet の範囲（100.64.0.0/10、fd7a:115c:a1e0::/48）。 */
    val isTailnet: Boolean
        get() =
            if (isV4) {
                bytes[0].toInt() == TAILNET_V4_FIRST && (bytes[1].toInt() and TAILNET_V4_MASK) == TAILNET_V4_SECOND
            } else {
                bytes.copyOfRange(0, TAILNET_V6_PREFIX.size).contentEquals(TAILNET_V6_PREFIX)
            }

    /** ループバック（127.0.0.0/8、::1）。 */
    val isLoopback: Boolean
        get() =
            if (isV4) {
                bytes[0].toInt() == LOOPBACK_V4_FIRST
            } else {
                (0 until V6_SIZE - 1).all { bytes[it] == ZERO } && bytes[V6_SIZE - 1].toInt() == 1
            }

    override fun toString(): String = if (isV4) formatV4() else formatV6()

    private fun formatV4(): String = bytes.joinToString(".") { (it.toInt() and BYTE_MASK).toString() }

    // 最も長い 0 の連続（2 グループ以上。同じ長さなら先頭側）を `::` にする（RFC 5952。Go の string6 と同じ）。
    private fun formatV6(): String {
        val groups = IntArray(V6_GROUPS) { (byteAt(it * 2) shl 8) or byteAt(it * 2 + 1) }
        var zeroStart = -1
        var zeroEnd = -1
        for (start in 0 until V6_GROUPS) {
            var end = start
            while (end < V6_GROUPS && groups[end] == 0) end++
            if (end - start >= 2 && end - start > zeroEnd - zeroStart) {
                zeroStart = start
                zeroEnd = end
            }
        }
        val out = StringBuilder()
        var i = 0
        while (i < V6_GROUPS) {
            if (i == zeroStart) {
                out.append("::")
                i = zeroEnd
                if (i >= V6_GROUPS) break
            } else if (i > 0) {
                out.append(':')
            }
            out.append(groups[i].toString(HEX_RADIX))
            i++
        }
        return out.toString()
    }

    private fun byteAt(index: Int): Int = bytes[index].toInt() and BYTE_MASK

    companion object {
        private const val V4_SIZE = 4
        private const val V6_SIZE = 16
        private const val V6_GROUPS = 8
        private const val V4_IN_V6_PREFIX = 10
        private const val BYTE_MASK = 0xFF
        private const val HEX_RADIX = 16
        private const val MAX_V4_OCTET = 255
        private const val MAX_V4_DIGITS = 3
        private const val MAX_GROUP_DIGITS = 4
        private const val V4_TAIL_START = 12
        private const val ZERO: Byte = 0
        private const val FF: Byte = -1
        private const val TAILNET_V4_FIRST = 100
        private const val TAILNET_V4_MASK = 0xC0
        private const val TAILNET_V4_SECOND = 0x40
        private const val LOOPBACK_V4_FIRST = 127
        private val TAILNET_V6_PREFIX = byteArrayOf(0xFD.toByte(), 0x7A, 0x11, 0x5C, 0xA1.toByte(), 0xE0.toByte())

        /** IPv4（ドットつき 10 進 4 つ。先頭ゼロ不可）または IPv6 のリテラル。解釈できなければ null。 */
        fun parse(text: String): IpAddress? =
            when {
                text.contains(':') -> parseV6(text)
                text.contains('.') -> parseV4(text)?.let(::IpAddress)
                else -> null
            }

        private fun parseV4(text: String): ByteArray? {
            val parts = text.split('.')
            if (parts.size != V4_SIZE) return null
            val out = ByteArray(V4_SIZE)
            for ((index, part) in parts.withIndex()) {
                val valid =
                    part.isNotEmpty() &&
                        part.length <= MAX_V4_DIGITS &&
                        part.all { it in '0'..'9' } &&
                        (part.length == 1 || part[0] != '0')
                if (!valid || part.toInt() > MAX_V4_OCTET) return null
                out[index] = part.toInt().toByte()
            }
            return out
        }

        private fun isAsciiHex(c: Char): Boolean = c in '0'..'9' || c in 'a'..'f' || c in 'A'..'F'

        private fun parseV6(text: String): IpAddress? {
            if (text.contains('%')) return null
            val out = ByteArray(V6_SIZE)
            var rest = text
            var written = 0
            var ellipsis = -1
            if (rest.startsWith("::")) {
                ellipsis = 0
                rest = rest.substring(2)
                if (rest.isEmpty()) return IpAddress(out)
            }
            while (written < V6_SIZE) {
                val digits = rest.takeWhile(::isAsciiHex)
                if (digits.isEmpty() || digits.length > MAX_GROUP_DIGITS) return null
                if (rest.getOrNull(digits.length) == '.') {
                    // 末尾の IPv4（`::1.2.3.4`）。残り全体が IPv4 でなければならない。
                    if ((ellipsis < 0 && written != V4_TAIL_START) || written + V4_SIZE > V6_SIZE) return null
                    parseV4(rest)?.copyInto(out, written) ?: return null
                    written += V4_SIZE
                    rest = ""
                    break
                }
                val value = digits.toInt(HEX_RADIX)
                out[written] = (value shr 8).toByte()
                out[written + 1] = value.toByte()
                written += 2
                rest = rest.substring(digits.length)
                if (rest.isEmpty()) break
                if (rest[0] != ':' || rest.length == 1) return null
                rest = rest.substring(1)
                if (rest[0] == ':') {
                    if (ellipsis >= 0) return null
                    ellipsis = written
                    rest = rest.substring(1)
                    if (rest.isEmpty()) break
                }
            }
            if (rest.isNotEmpty()) return null
            if (written < V6_SIZE) {
                if (ellipsis < 0) return null
                val tail = written - ellipsis
                out.copyInto(out, V6_SIZE - tail, ellipsis, written)
                out.fill(ZERO, ellipsis, V6_SIZE - tail)
            } else if (ellipsis >= 0) {
                return null
            }
            return IpAddress(out)
        }
    }
}
