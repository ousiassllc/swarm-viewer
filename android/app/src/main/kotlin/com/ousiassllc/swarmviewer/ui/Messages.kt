package com.ousiassllc.swarmviewer.ui

import com.ousiassllc.swarmviewer.domain.ErrKind
import com.ousiassllc.swarmviewer.domain.FetchException
import com.ousiassllc.swarmviewer.domain.PairingError
import com.ousiassllc.swarmviewer.domain.PairingException

// 利用者向けの固定文言（機能要件 §10、API 仕様 §2.2）。swarm-console のエラー本文（message・details）と、
// 入力値（token を含みうる）・完全な URL は画面に出さない。デスクトップ版（internal/web/format/text.go）と同じ文言。

/** 接続文字列の検証失敗の文言（API 仕様 §2.2）。 */
fun pairingErrorMessage(error: PairingError): String =
    when (error) {
        PairingError.Version -> "アプリを更新してください"
        PairingError.InvalidUrl -> "接続先が許可された範囲外です（Tailscale の IP のみ）"
        PairingError.Token -> "token が含まれていません"
        PairingError.Format -> "接続文字列の形式が正しくありません"
    }

fun pairingErrorMessage(e: PairingException): String = pairingErrorMessage(e.error)

/** 取得（疎通確認を含む）失敗の文言（機能要件 §10、API 仕様 §1.5）。 */
fun fetchErrorMessage(kind: ErrKind): String =
    when (kind) {
        ErrKind.Unreachable -> "ホストに届きません。Tailscale 接続を確認してください"
        ErrKind.Unauthorized -> "認証できません。再ペアしてください"
        ErrKind.ForbiddenScope -> "このトークンでは接続できません（report 用が必要）"
        ErrKind.SessionRequired -> "セッションを選んでください"
        ErrKind.SessionForbidden -> "このセッションは許可されていません"
        ErrKind.SessionMissing -> "セッションが存在しません"
        ErrKind.HostError -> "ホスト側でエラーが発生しました"
        ErrKind.Protocol -> "接続先がこのアプリと互換ではありません"
    }

fun fetchErrorMessage(e: FetchException): String = fetchErrorMessage(e.kind)

const val DRAFT_EXPIRED_MESSAGE = "接続の確認の有効期限が切れました。もう一度、接続文字列を入力してください"
const val SAVE_FAILED_MESSAGE = "接続を保存できませんでした。もう一度お試しください"
const val UNEXPECTED_ERROR_MESSAGE = "処理に失敗しました。もう一度お試しください"

/** 再ペアリングが必要な接続に出す文言。 */
const val NEEDS_REPAIR_LABEL = "再ペアリングが必要です"
