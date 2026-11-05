package com.ousiassllc.swarmviewer.data.secret

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.security.GeneralSecurityException
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val TOKEN = "test-token-not-secret"
private const val IV_BYTES = 12
private const val TAG_BITS = 128

/** JVM 用の AES-GCM。本番（[KeystoreTokenCipher]）と同じ形式（IV + 暗号文 + タグ）で、鍵だけメモリ上にある。 */
private class InMemoryCipher(
    private val key: SecretKey = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey(),
) : TokenCipher {
    override fun encrypt(
        plain: ByteArray,
        associatedData: ByteArray,
    ): ByteArray {
        val iv = ByteArray(IV_BYTES).also { SecureRandom().nextBytes(it) }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key, GCMParameterSpec(TAG_BITS, iv)) }
        cipher.updateAAD(associatedData)
        return iv + cipher.doFinal(plain)
    }

    override fun decrypt(
        sealed: ByteArray,
        associatedData: ByteArray,
    ): ByteArray {
        if (sealed.size <= IV_BYTES) throw GeneralSecurityException("too short")
        val cipher =
            Cipher.getInstance("AES/GCM/NoPadding").apply {
                init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES))
            }
        cipher.updateAAD(associatedData)
        return cipher.doFinal(sealed, IV_BYTES, sealed.size - IV_BYTES)
    }
}

class FileTokenStoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private fun store(cipher: TokenCipher = InMemoryCipher()) = FileTokenStore(tmp.root.resolve("tokens"), cipher)

    @Test
    fun putThenGetReturnsTheToken() {
        val store = store()
        store.put("conn-1", TOKEN)

        assertEquals(TOKEN, store.get("conn-1"))
    }

    @Test
    fun putOverwritesAndKeepsConnectionsSeparate() {
        val store = store()
        store.put("conn-1", "old-token")
        store.put("conn-2", "second-token")
        store.put("conn-1", TOKEN)

        assertEquals(TOKEN, store.get("conn-1"))
        assertEquals("second-token", store.get("conn-2"))
    }

    @Test
    fun getForAnUnknownConnectionIsNull() {
        assertNull(store().get("never-saved"))
    }

    @Test
    fun deleteRemovesTheTokenAndIsIdempotent() {
        val store = store()
        store.put("conn-1", TOKEN)

        store.delete("conn-1")
        store.delete("conn-1")

        assertNull(store.get("conn-1"))
        assertEquals(
            emptyList<String>(),
            tmp.root
                .resolve("tokens")
                .list()
                .orEmpty()
                .toList(),
        )
    }

    @Test
    fun theFileNeverContainsThePlainToken() {
        val store = store()
        store.put("conn-1", TOKEN)

        val bytes =
            tmp.root
                .resolve("tokens")
                .listFiles()
                .orEmpty()
                .single()
                .readBytes()
        assertFalse(String(bytes, Charsets.ISO_8859_1).contains(TOKEN))
        // 同じ token でも暗号文は毎回変わる（IV がランダム）
        store.put("conn-1", TOKEN)
        val again =
            tmp.root
                .resolve("tokens")
                .listFiles()
                .orEmpty()
                .single()
                .readBytes()
        assertFalse(bytes.contentEquals(again))
    }

    @Test
    fun noTemporaryFileIsLeftBehind() {
        val store = store()
        store.put("conn-1", TOKEN)

        assertEquals(
            listOf("conn-1.token"),
            tmp.root
                .resolve("tokens")
                .list()
                .orEmpty()
                .toList(),
        )
    }

    @Test
    fun aLostKeyMeansRePairingNotAnError() {
        store(InMemoryCipher()).put("conn-1", TOKEN)

        // 鍵が変わった（Keystore の鍵の消失に相当）。復号できない token は null＝再ペアリングが必要。
        assertNull(store(InMemoryCipher()).get("conn-1"))
    }

    @Test
    fun aCorruptedFileMeansRePairing() {
        val store = store()
        store.put("conn-1", TOKEN)
        val file = tmp.root.resolve("tokens/conn-1.token")
        file.writeBytes(file.readBytes().also { it[it.size - 1] = (it[it.size - 1].toInt() xor 1).toByte() })

        assertNull(store.get("conn-1"))
        file.writeBytes(byteArrayOf(1, 2, 3)) // 短すぎる
        assertNull(store.get("conn-1"))
    }

    @Test
    fun aCiphertextCannotBeMovedToAnotherConnection() {
        val store = store(InMemoryCipher())
        store.put("conn-1", TOKEN)
        val dir = tmp.root.resolve("tokens")
        dir.resolve("conn-1.token").copyTo(dir.resolve("conn-2.token"))

        assertNull(store.get("conn-2"))
        assertEquals(TOKEN, store.get("conn-1"))
    }

    @Test
    fun anEncryptionFailureLeavesNoFileAndIsReported() {
        val failing =
            object : TokenCipher {
                override fun encrypt(
                    plain: ByteArray,
                    associatedData: ByteArray,
                ): ByteArray = throw GeneralSecurityException("boom")

                override fun decrypt(
                    sealed: ByteArray,
                    associatedData: ByteArray,
                ): ByteArray = throw GeneralSecurityException("boom")
            }
        val store = store(failing)

        assertThrows(GeneralSecurityException::class.java) { store.put("conn-1", TOKEN) }
        assertTrue(
            tmp.root
                .resolve("tokens")
                .list()
                .orEmpty()
                .isEmpty(),
        )
    }

    @Test
    fun idsThatCouldEscapeTheDirectoryAreRefused() {
        val store = store()
        for (id in listOf("", "../evil", "a/b", "a\\b", ".", "..", "a".repeat(65))) {
            assertThrows(id, IllegalArgumentException::class.java) { store.put(id, TOKEN) }
            assertThrows(id, IllegalArgumentException::class.java) { store.get(id) }
            assertThrows(id, IllegalArgumentException::class.java) { store.delete(id) }
        }
    }

    @Test
    fun inMemoryCipherRoundTripsBytes() {
        val cipher = InMemoryCipher()
        val sealed = cipher.encrypt("x".toByteArray(), "a".toByteArray())

        assertArrayEquals("x".toByteArray(), cipher.decrypt(sealed, "a".toByteArray()))
        assertThrows(GeneralSecurityException::class.java) { cipher.decrypt(sealed, "b".toByteArray()) }
    }
}
