package com.ousiassllc.swarmviewer.ui

import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.OPERATION_CODES
import com.ousiassllc.swarmviewer.domain.PairingError
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.StartResult
import com.ousiassllc.swarmviewer.service.OpsRequiredException
import com.ousiassllc.swarmviewer.service.PaneBusyException
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

    @Test
    fun operationFailuresUseTheFixedWordingOfEveryCode() {
        val expected =
            mapOf(
                "invalid_state" to "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください",
                "not_asking" to "この質問はすでに閉じています",
                "option_out_of_range" to "その選択肢はありません。最新の質問を読み込み直してください",
                "custom_not_allowed" to "この質問は自由入力に対応していません",
                "cursor_mismatch" to "選択肢を合わせられませんでした（何も送っていません）。もう一度お試しください",
                "confirm_required" to "確認が必要です",
                "forbidden_control_char" to "使えない制御文字が含まれています（改行とタブだけ使えます）",
                "text_too_long" to "長すぎます（8000 字まで）",
                "forbidden_path" to "このフォルダは選べません",
                "path_forbidden" to "このフォルダは選べません",
                "file_unreadable" to "ファイルを読めませんでした",
                "unsupported_agent" to "このペインは対応していないエージェントです",
                "pane_not_found" to "ペインが見つかりません。画面を更新してください",
                "busy_timeout" to "同じペインの別の操作がまだ終わっていません。しばらくしてからお試しください",
                "exit_timeout" to "終了が間に合いませんでした（もう一度試せます）",
                "not_found" to "フォルダが見つかりません",
                "unknown_tool" to "この接続では使えない操作です（ホストの swarm-console が古い可能性があります）",
                "invalid_args" to "入力内容を受け付けられませんでした",
            )
        assertEquals(OPERATION_CODES + "invalid_args", expected.keys)
        for ((code, message) in expected) {
            assertEquals(code, message, operationErrorMessage(code))
            assertEquals(code, message, fetchErrorMessage(FetchException(ErrKind.Operation, code)))
        }
    }

    @Test
    fun viewOnlyAndBusyRejectionsUseTheFixedWording() {
        assertEquals("この接続では操作できません（見るだけの接続です）", fetchErrorMessage(FetchException(ErrKind.Protocol, "tool_not_allowed")))
        assertEquals("この接続では操作できません（見るだけの接続です）", rejectionMessage(OpsRequiredException()))
        assertEquals("前の操作がまだ終わっていません", rejectionMessage(PaneBusyException()))
        assertEquals(null, rejectionMessage(IllegalStateException()))
    }

    @Test
    fun resultToastsFollowTheOkAcceptedAndCardClosedFlags() {
        assertEquals("送りました", pickToast(PickResult(ok = true, cardClosed = true)))
        assertEquals("選択を確定できませんでした。もう一度お試しください", pickToast(PickResult(ok = false, cardClosed = false)))
        assertEquals("選択を確定できませんでした。もう一度お試しください", pickToast(PickResult(ok = true, cardClosed = false)))
        assertEquals("送りました", sendToast(SendResult(ok = true, accepted = true)))
        assertEquals("受け付けられませんでした", sendToast(SendResult(ok = false, accepted = false)))
        assertEquals("中断しました", escapeToast(EscapeResult(ok = true, interrupted = true)))
        assertEquals("状態が変わりませんでした", escapeToast(EscapeResult(ok = false, interrupted = false)))
        assertEquals("起動しました", startToast(StartResult(ok = true, state = "idle")))
        assertEquals("起動を確認できませんでした（状態: 空き）", startToast(StartResult(ok = false, state = "shell")))
        assertEquals("終了しました", closeToast(CloseResult(ok = true, state = "shell")))
        assertEquals("片付けを確認できませんでした（状態: 指示待ち）", closeToast(CloseResult(ok = false, state = "idle")))
    }

    @Test
    fun anUnknownResultStateIsNeverEchoedIntoTheMessage() {
        val message = startToast(StartResult(ok = false, state = "<script>secret</script>"))

        assertFalse(message, message.contains("secret"))
        assertEquals("起動を確認できませんでした（状態: 判定できない）", message)
    }

    @Test
    fun theServerErrorCodeOfAnOperationFailureIsNeverShown() {
        assertFalse(fetchErrorMessage(FetchException(ErrKind.Operation, "secret_code")).contains("secret_code"))
        assertEquals(OPERATION_FALLBACK_MESSAGE, fetchErrorMessage(FetchException(ErrKind.Operation, "secret_code")))
    }
}
