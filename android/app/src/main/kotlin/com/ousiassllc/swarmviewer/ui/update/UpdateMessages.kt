package com.ousiassllc.swarmviewer.ui.update

import com.ousiassllc.swarmviewer.domain.UpdateFailure

// 更新の利用者向けの固定文言（#109）。通信先の URL・応答本文・例外の本文は画面に出さない。

/** インストールの許可が無く、設定画面を開いたあとの案内。 */
const val INSTALL_PERMISSION_GUIDE = "「このアプリからのインストールを許可」をオンにしてから、もう一度「更新する」を押してください"

/** 失敗の短い文言。 */
fun updateFailureMessage(failure: UpdateFailure): String =
    when (failure) {
        UpdateFailure.Network, UpdateFailure.Rejected -> "更新を確認できませんでした。通信を確認してください"
        UpdateFailure.Truncated -> "ダウンロードが途中で止まりました。もう一度お試しください"
        UpdateFailure.AssetMissing -> "この版の更新ファイルが見つかりません"
        UpdateFailure.ChecksumInvalid, UpdateFailure.ChecksumMismatch -> "更新ファイルを確認できなかったため、インストールしませんでした"
        UpdateFailure.SignatureMismatch -> "更新ファイルの署名がこのアプリと違うため、インストールしませんでした"
        UpdateFailure.Storage -> "更新ファイルを保存できませんでした"
        UpdateFailure.Installer -> "インストーラを開けませんでした"
    }
