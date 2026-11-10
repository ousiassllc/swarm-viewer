package com.ousiassllc.swarmviewer.data.update

import java.io.File
import java.security.MessageDigest

private const val BUFFER_BYTES = 64 * 1024

/** ファイルの SHA-256（小文字の 16 進）。読み取りの失敗は [java.io.IOException]。 */
fun sha256Hex(file: File): String {
    val digest = MessageDigest.getInstance("SHA-256")
    val buffer = ByteArray(BUFFER_BYTES)
    file.inputStream().use { input ->
        while (true) {
            val n = input.read(buffer)
            if (n < 0) break
            digest.update(buffer, 0, n)
        }
    }
    return digest.digest().joinToString("") { "%02x".format(it) }
}
