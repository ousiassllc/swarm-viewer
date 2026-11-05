package com.ousiassllc.swarmviewer.data.secret

import java.security.GeneralSecurityException

/**
 * token の暗号化・復号。[associatedData]（接続 ID）を認証対象に含め、別の接続の暗号文として差し替えられても復号できないようにする。
 * 失敗は [GeneralSecurityException]（鍵の消失・暗号文の破損・改ざんを区別しない）。
 */
interface TokenCipher {
    /** 平文を暗号化する。戻り値は復号に必要な情報（IV）をすべて含む。 */
    fun encrypt(
        plain: ByteArray,
        associatedData: ByteArray,
    ): ByteArray

    fun decrypt(
        sealed: ByteArray,
        associatedData: ByteArray,
    ): ByteArray
}
