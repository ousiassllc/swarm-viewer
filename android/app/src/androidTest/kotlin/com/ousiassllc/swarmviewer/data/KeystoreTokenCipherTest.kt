package com.ousiassllc.swarmviewer.data

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.ousiassllc.swarmviewer.data.secret.FileTokenStore
import com.ousiassllc.swarmviewer.data.secret.KeystoreTokenCipher
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.security.GeneralSecurityException
import java.security.KeyStore
import javax.crypto.SecretKey

private const val ALIAS = "swarm-viewer-test-token-key"
private const val TOKEN = "test-token-not-secret"
private const val CONNECTION_ID = "conn-1"

/** 実機・エミュレータの Android Keystore で、token の暗号化・復号を確かめる。 */
@RunWith(AndroidJUnit4::class)
class KeystoreTokenCipherTest {
    private val context: Context = ApplicationProvider.getApplicationContext()
    private val directory = File(context.noBackupFilesDir, "keystore-test-tokens")

    private fun keyStore() = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    @Before
    @After
    fun clean() {
        keyStore().deleteEntry(ALIAS)
        directory.deleteRecursively()
    }

    @Test
    fun encryptThenDecryptRoundTrips() {
        val cipher = KeystoreTokenCipher(ALIAS)
        val sealed = cipher.encrypt(TOKEN.toByteArray(), "a".toByteArray())

        assertFalse(String(sealed, Charsets.ISO_8859_1).contains(TOKEN))
        assertArrayEquals(TOKEN.toByteArray(), cipher.decrypt(sealed, "a".toByteArray()))
    }

    @Test
    fun theIvIsFreshForEveryEncryption() {
        val cipher = KeystoreTokenCipher(ALIAS)

        val first = cipher.encrypt(TOKEN.toByteArray(), "a".toByteArray())
        val second = cipher.encrypt(TOKEN.toByteArray(), "a".toByteArray())

        assertFalse(first.copyOfRange(0, 12).contentEquals(second.copyOfRange(0, 12)))
    }

    @Test
    fun theKeyIsNotExportable() {
        KeystoreTokenCipher(ALIAS).encrypt(TOKEN.toByteArray(), ByteArray(0))

        val key = keyStore().getKey(ALIAS, null) as SecretKey
        assertEquals("AES", key.algorithm)
        assertNull(key.encoded) // 鍵の中身は取り出せない
    }

    @Test
    fun aDifferentAssociatedDataOrATamperedCiphertextFailsToDecrypt() {
        val cipher = KeystoreTokenCipher(ALIAS)
        val sealed = cipher.encrypt(TOKEN.toByteArray(), "a".toByteArray())

        assertThrows(GeneralSecurityException::class.java) { cipher.decrypt(sealed, "b".toByteArray()) }
        val tampered = sealed.copyOf().also { it[it.size - 1] = (it[it.size - 1].toInt() xor 1).toByte() }
        assertThrows(GeneralSecurityException::class.java) { cipher.decrypt(tampered, "a".toByteArray()) }
        assertThrows(GeneralSecurityException::class.java) { cipher.decrypt(ByteArray(3), "a".toByteArray()) }
    }

    @Test
    fun aLostKeyMeansTheTokenIsGoneAndRePairingIsNeeded() {
        val store = FileTokenStore(directory, KeystoreTokenCipher(ALIAS))
        store.put(CONNECTION_ID, TOKEN)
        assertEquals(TOKEN, store.get(CONNECTION_ID))
        val onDisk =
            directory
                .listFiles()
                .orEmpty()
                .single()
                .readBytes()
        assertFalse(String(onDisk, Charsets.ISO_8859_1).contains(TOKEN))

        keyStore().deleteEntry(ALIAS) // 鍵の消失（アプリのデータ削除・Keystore のリセットに相当）

        assertNull(store.get(CONNECTION_ID))
        assertNotNull(directory.listFiles()) // 暗号文は残るが、読めない
        store.put(CONNECTION_ID, TOKEN) // 再ペアリングで新しい鍵に作り直される
        assertEquals(TOKEN, store.get(CONNECTION_ID))
    }

    @Test
    fun theTokenDirectoryIsOutsideBackups() {
        // noBackupFilesDir 配下はバックアップ・端末間転送の対象外（Context.getNoBackupFilesDir）。
        assertTrue(directory.absolutePath.startsWith(context.noBackupFilesDir.absolutePath))
    }
}
