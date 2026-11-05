package com.ousiassllc.swarmviewer.data.secret

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.GeneralSecurityException
import java.security.KeyStore
import java.security.ProviderException
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val ANDROID_KEYSTORE = "AndroidKeyStore"
private const val TRANSFORMATION = "AES/GCM/NoPadding"
private const val KEY_SIZE_BITS = 256
private const val IV_BYTES = 12
private const val TAG_BITS = 128

/** token の暗号化鍵の別名（Keystore 内）。 */
const val TOKEN_KEY_ALIAS = "swarm-viewer-token-key"

/**
 * Android Keystore の AES-GCM 鍵（256 ビット、エクスポート不可）で token を暗号化する [TokenCipher]（NFR-SEC-2）。
 * 鍵は最初の暗号化のときに作る。出力は `IV（12 バイト）+ 暗号文 + 認証タグ`。
 *
 * 鍵が消えた（アプリのデータ削除・端末の生体情報の変更・Keystore のリセットなど）ときの復号は [GeneralSecurityException]。
 * Keystore の内部エラーが [ProviderException] で出ることがあるため、同じ失敗として [GeneralSecurityException] に揃える。
 */
class KeystoreTokenCipher(
    private val alias: String = TOKEN_KEY_ALIAS,
) : TokenCipher {
    override fun encrypt(
        plain: ByteArray,
        associatedData: ByteArray,
    ): ByteArray =
        guarded {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, key()) // IV は Keystore が毎回ランダムに作る（setRandomizedEncryptionRequired）
            cipher.updateAAD(associatedData)
            cipher.iv + cipher.doFinal(plain)
        }

    override fun decrypt(
        sealed: ByteArray,
        associatedData: ByteArray,
    ): ByteArray =
        guarded {
            if (sealed.size <= IV_BYTES) throw GeneralSecurityException("sealed token is too short")
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, existingKey(), GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES))
            cipher.updateAAD(associatedData)
            cipher.doFinal(sealed, IV_BYTES, sealed.size - IV_BYTES)
        }

    /** 鍵（無ければ作る）。 */
    private fun key(): SecretKey = existingKeyOrNull() ?: generateKey()

    private fun existingKey(): SecretKey = existingKeyOrNull() ?: throw GeneralSecurityException("token key is gone")

    private fun existingKeyOrNull(): SecretKey? = keyStore().getKey(alias, null) as? SecretKey

    private fun keyStore(): KeyStore = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }

    private fun generateKey(): SecretKey {
        val spec =
            KeyGenParameterSpec
                .Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(KEY_SIZE_BITS)
                .setRandomizedEncryptionRequired(true)
                .build()
        return KeyGenerator
            .getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
            .apply { init(spec) }
            .generateKey()
    }

    private inline fun <T> guarded(block: () -> T): T =
        try {
            block()
        } catch (e: ProviderException) {
            throw GeneralSecurityException("keystore failure", e)
        }
}
