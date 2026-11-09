package com.ousiassllc.swarmviewer.ui

import com.ousiassllc.swarmviewer.domain.CloseResult
import com.ousiassllc.swarmviewer.domain.EscapeResult
import com.ousiassllc.swarmviewer.domain.PaneState
import com.ousiassllc.swarmviewer.domain.PickResult
import com.ousiassllc.swarmviewer.domain.SendResult
import com.ousiassllc.swarmviewer.domain.StartResult
import com.ousiassllc.swarmviewer.service.OpsRequiredException
import com.ousiassllc.swarmviewer.service.PaneBusyException

// 操作モードの固定文言（機能要件 §10）。デスクトップ版（internal/web/format/text.go）と同じ文言だが、結果の `state` の表示だけは
// Kotlin が状態の日本語ラベル（working = 作業中 など。未知の値は「判定できない」）、Go が state の値そのものを出す。
// swarm-console の `message`・`details`・送った本文・ペインの本文は画面に出さない。

/** 操作ツールの失敗コードが未知・文言の無いときの文言。 */
const val OPERATION_FALLBACK_MESSAGE = "入力内容を受け付けられませんでした"

/** 見るだけの接続への操作要求（通信せずに拒否したとき）。 */
const val VIEW_ONLY_MESSAGE = "この接続では操作できません（見るだけの接続です）"

/** viewer 側の二重送信防止（同じペインの別の操作が実行中）。 */
const val PANE_BUSY_MESSAGE = "前の操作がまだ終わっていません"

/** 質問が閉じていて、見るだけの接続でできること（見るだけの質問カードの 1 行）。 */
const val ASK_VIEW_ONLY_HINT = "この質問には、操作できる接続から答えられます"

/** 中断の確認ダイアログ（`send_escape` の前に必ず出す）。 */
const val ESCAPE_CONFIRM_TITLE = "作業中のエージェントを止めます"
const val ESCAPE_CONFIRM_BODY = "ペインに Esc を送って、実行中の作業を中断します。"
const val ESCAPE_CONFIRM_ACTION = "中断する"

/** 終了の確認ダイアログ（`pane_close` の前に必ず出す）。 */
const val CLOSE_CONFIRM_TITLE = "このペインを終了します"
const val CLOSE_CONFIRM_BODY = "エージェントとペインを終了します。この操作は取り消せません。"
const val CLOSE_CONFIRM_ACTION = "終了する"

/** 確認ダイアログの取り消し。 */
const val CONFIRM_CANCEL_ACTION = "取り消し"

/** 操作ツールの失敗コードごとの固定文言（機能要件 §10）。未知のコードは [OPERATION_FALLBACK_MESSAGE]。 */
fun operationErrorMessage(code: String): String =
    when (code) {
        "invalid_state" -> "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください"
        "not_asking" -> "この質問はすでに閉じています"
        "option_out_of_range" -> "その選択肢はありません。最新の質問を読み込み直してください"
        "custom_not_allowed" -> "この質問は自由入力に対応していません"
        "cursor_mismatch" -> "選択肢を合わせられませんでした（何も送っていません）。もう一度お試しください"
        "confirm_required" -> "確認が必要です"
        "forbidden_control_char" -> "使えない制御文字が含まれています（改行とタブだけ使えます）"
        "text_too_long" -> "長すぎます（8000 字まで）"
        "forbidden_path", "path_forbidden" -> "このフォルダは選べません"
        "file_unreadable" -> "ファイルを読めませんでした"
        "unsupported_agent" -> "このペインは対応していないエージェントです"
        "pane_not_found" -> "ペインが見つかりません。画面を更新してください"
        "busy_timeout" -> "同じペインの別の操作がまだ終わっていません。しばらくしてからお試しください"
        "exit_timeout" -> "終了が間に合いませんでした（もう一度試せます）"
        "not_found" -> "フォルダが見つかりません"
        "unknown_tool" -> "この接続では使えない操作です（ホストの swarm-console が古い可能性があります）"
        else -> OPERATION_FALLBACK_MESSAGE // `invalid_args` を含む
    }

/** 操作の呼び出しで投げられた例外のうち、viewer 側の拒否の文言。該当しなければ null。 */
fun rejectionMessage(e: Exception): String? =
    when (e) {
        is OpsRequiredException -> VIEW_ONLY_MESSAGE
        is PaneBusyException -> PANE_BUSY_MESSAGE
        else -> null
    }

/** `ask_pick` の結果の通知（トースト）。 */
fun pickToast(result: PickResult): String = if (result.ok && result.cardClosed) "送りました" else "選択を確定できませんでした。もう一度お試しください"

/** `send_text` の結果の通知。 */
fun sendToast(result: SendResult): String = if (result.accepted) "送りました" else "受け付けられませんでした"

/** `send_escape` の結果の通知。 */
fun escapeToast(result: EscapeResult): String = if (result.ok && result.interrupted) "中断しました" else "状態が変わりませんでした"

/** `pane_start` の結果の通知。 */
fun startToast(result: StartResult): String = if (result.ok) "起動しました" else "起動を確認できませんでした（状態: ${stateLabel(result.state)}）"

/** `pane_close` の結果の通知。 */
fun closeToast(result: CloseResult): String = if (result.ok) "終了しました" else "片付けを確認できませんでした（状態: ${stateLabel(result.state)}）"

// 状態（report の値）は表示ラベルにする。未知の値はそのまま出さず「判定できない」にする（応答由来の文字列を画面に直接出さない）。
private fun stateLabel(state: String): String = PaneState.parse(state).label

/** ペアリング確認画面（操作の接続のとき）の警告文。 */
const val OPS_PAIRING_WARNING = "この接続はペインに送信・中断・起動・終了できます"
const val OPS_TOKEN_NOTE = "token が漏れると、この Tailscale 内からペインを操作されます。紛失したらホストで `swarm-console token revoke <名前>` を実行してください"
