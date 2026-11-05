package com.ousiassllc.swarmviewer.ui

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PairingError
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

/** 利用者向けの固定文言が、API 仕様 §2.2・機能要件 §10 のとおりであること。 */
class MessagesTest {
    @Test
    fun pairingFailuresUseTheSpecifiedWording() {
        assertEquals("接続文字列の形式が正しくありません", pairingErrorMessage(PairingError.Format))
        assertEquals("アプリを更新してください", pairingErrorMessage(PairingError.Version))
        assertEquals("接続先が許可された範囲外です（Tailscale の IP のみ）", pairingErrorMessage(PairingError.InvalidUrl))
        assertEquals("token が含まれていません", pairingErrorMessage(PairingError.Token))
    }

    @Test
    fun fetchFailuresUseTheSpecifiedWording() {
        assertEquals("ホストに届きません。Tailscale 接続を確認してください", fetchErrorMessage(ErrKind.Unreachable))
        assertEquals("認証できません。再ペアしてください", fetchErrorMessage(ErrKind.Unauthorized))
        assertEquals("このトークンでは接続できません（report 用が必要）", fetchErrorMessage(ErrKind.ForbiddenScope))
        assertEquals("セッションを選んでください", fetchErrorMessage(ErrKind.SessionRequired))
        assertEquals("このセッションは許可されていません", fetchErrorMessage(ErrKind.SessionForbidden))
        assertEquals("セッションが存在しません", fetchErrorMessage(ErrKind.SessionMissing))
        assertEquals("ホスト側でエラーが発生しました", fetchErrorMessage(ErrKind.HostError))
        assertEquals("接続先がこのアプリと互換ではありません", fetchErrorMessage(ErrKind.Protocol))
    }

    @Test
    fun theServerErrorCodeIsNeverShownInTheMessage() {
        for (kind in ErrKind.entries) {
            assertFalse(fetchErrorMessage(FetchException(kind, "secret_code")).contains("secret_code"))
        }
    }
}
