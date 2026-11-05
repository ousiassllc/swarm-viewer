package com.ousiassllc.swarmviewer.data.secret

import java.io.File
import java.io.IOException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.GeneralSecurityException

/** 接続ごとの token の保存先（ER §7.3）。キーは接続 ID。 */
interface TokenStore {
    /** token を保存する（同じ接続 ID があれば上書き）。失敗は [IOException]・[GeneralSecurityException]。 */
    fun put(
        connectionId: String,
        token: String,
    )

    /**
     * token を返す。保存されていない・復号できない（Keystore の鍵が消えた・暗号文が壊れた）ときは null。
     * どちらも「再ペアリングが必要」として扱う（401 と同じ導線）。ファイルを読めない I/O の失敗は [IOException]。
     */
    fun get(connectionId: String): String?

    /** token を削除する。保存されていなくても成功。失敗（[IOException]）のときは削除されていない。 */
    fun delete(connectionId: String)
}

private val CONNECTION_ID = Regex("[A-Za-z0-9_-]{1,64}")
private const val FILE_SUFFIX = ".token"
private const val TMP_SUFFIX = ".tmp"

/**
 * [TokenCipher] で暗号化した token を、[directory]（アプリの `noBackupFilesDir` 配下）に接続ごとのファイルとして保存する
 * （EncryptedSharedPreferences は使わない）。平文は DB・ファイル・ログに書かない。暗号文は接続 ID と結び付ける。
 * バックアップ・端末間転送の対象外にするため、[directory] は `noBackupFilesDir` を渡すこと。
 */
class FileTokenStore(
    private val directory: File,
    private val cipher: TokenCipher,
) : TokenStore {
    override fun put(
        connectionId: String,
        token: String,
    ) {
        val target = fileOf(connectionId)
        val sealed = cipher.encrypt(token.toByteArray(Charsets.UTF_8), associatedData(connectionId))
        if (!directory.isDirectory && !directory.mkdirs() && !directory.isDirectory) throw IOException("cannot create token directory")
        // 書き込み途中の状態を残さないよう、同じディレクトリの一時ファイルに書いてから置き換える。
        val tmp = File(directory, target.name + TMP_SUFFIX)
        try {
            tmp.writeBytes(sealed)
            Files.move(tmp.toPath(), target.toPath(), StandardCopyOption.ATOMIC_MOVE)
        } finally {
            tmp.delete()
        }
    }

    override fun get(connectionId: String): String? {
        val file = fileOf(connectionId)
        if (!file.isFile) return null
        val sealed = file.readBytes()
        return try {
            String(cipher.decrypt(sealed, associatedData(connectionId)), Charsets.UTF_8)
        } catch (e: GeneralSecurityException) {
            null // 例外の本文は捨てる（鍵・暗号文の情報を出さない）
        }
    }

    override fun delete(connectionId: String) {
        Files.deleteIfExists(fileOf(connectionId).toPath())
    }

    private fun fileOf(connectionId: String): File {
        require(CONNECTION_ID.matches(connectionId)) { "invalid connection id" } // パスの区切りを含む ID を通さない
        return File(directory, connectionId + FILE_SUFFIX)
    }

    private fun associatedData(connectionId: String): ByteArray = "connection:$connectionId".toByteArray(Charsets.UTF_8)
}
