package com.ousiassllc.swarmviewer.domain

/**
 * swarm-console の取得失敗の種別（API 仕様 §1.5）。[id] は固定の種別名（ログ・テストベクタ用）。
 * [Operation] は操作ツール（`ask_pick`・`send_text` など）の失敗（HTTP 200 の isError）で、`code` から画面の固定文言を引く。
 */
enum class ErrKind(
    val id: String,
) {
    Unreachable("unreachable"),
    Unauthorized("unauthorized"),
    ForbiddenScope("forbidden_scope"),
    SessionRequired("session_required"),
    SessionForbidden("session_forbidden"),
    SessionMissing("session_missing"),
    HostError("host_error"),
    Protocol("protocol"),
    Operation("operation"),
}

/**
 * swarm-console の失敗。[code] は swarm-console のエラーコード（無いとき空文字）。
 * message・details を持たない（NFR-LOG-3）。[message] は種別と code だけの固定文言で、応答本文を含めない。
 */
class FetchException(
    val kind: ErrKind,
    val code: String = "",
) : Exception(describe(kind, code)) {
    private companion object {
        fun describe(
            kind: ErrKind,
            code: String,
        ): String = if (code.isEmpty()) "swarm-console fetch error: ${kind.id}" else "swarm-console fetch error: ${kind.id} ($code)"
    }
}

private const val HTTP_UNAUTHORIZED = 401
private const val HTTP_FORBIDDEN = 403
private const val HTTP_NOT_FOUND = 404
private const val HTTP_METHOD_NOT_ALLOWED = 405
private const val HTTP_PAYLOAD_TOO_LARGE = 413
private const val HTTP_BAD_REQUEST = 400
private const val MAX_CODE_LENGTH = 64

/**
 * 失敗を [FetchException] に分類する（API 仕様 §1.5）。成功と判断できるとき（[error] が null で、
 * HTTP が 4xx/5xx でなく、[toolCode] も空）は null を返す。
 * [httpStatus] が 0 のときは HTTP の応答がない（タイムアウト・DNS・route なしなど）。
 *
 *  - [error] が [FetchException] ならそのまま返す
 *  - HTTP 401 → Unauthorized、403 → ForbiddenScope、404・405・413 → Protocol
 *  - ツールコード（HTTP 200 の isError、または HTTP エラー本文の code）:
 *    session_required / session_forbidden / invalid_args（session が sessions にない）/ tmux_error・internal → HostError。
 *    操作ツールの失敗（[OPERATION_CODES]）→ Operation。ほかは Protocol
 *  - HTTP が上記以外の 4xx/5xx で [toolCode] が写像に無ければ Protocol
 *  - HTTP の応答もツールコードも無い [error]（タイムアウト・DNS・route なし）→ Unreachable
 *  - [error] が [InvalidResponseException] → Protocol
 *
 * [toolCode] は応答本文由来の信頼できない値のため、`[a-z0-9_]{1,64}` に合わなければ code に入れない（種別は Protocol）。
 * HTTP エラーで code が空のときは `http_<status>` を入れる（ログにステータスコードを残すため）。
 * デスクトップ版の Classify と同じ結果になる（testdata/vectors/classify.json）。
 */
fun classify(
    error: Throwable?,
    httpStatus: Int = 0,
    toolCode: String = "",
): FetchException? {
    if (error is FetchException) return error
    val code = sanitizeCode(toolCode)
    val byStatus = classifyByStatus(httpStatus, code)
    if (byStatus != null) return byStatus
    return when {
        code.isNotEmpty() -> FetchException(toolKind(code) ?: ErrKind.Protocol, code)
        httpStatus >= HTTP_BAD_REQUEST -> FetchException(ErrKind.Protocol, "http_$httpStatus")
        toolCode.isNotEmpty() -> FetchException(ErrKind.Protocol) // 不正なコードは捨て、種別だけ残す
        error is InvalidResponseException -> FetchException(ErrKind.Protocol)
        error != null -> FetchException(ErrKind.Unreachable)
        else -> null
    }
}

private fun classifyByStatus(
    httpStatus: Int,
    code: String,
): FetchException? =
    when (httpStatus) {
        HTTP_UNAUTHORIZED -> FetchException(ErrKind.Unauthorized, code.ifEmpty { "unauthorized" })
        HTTP_FORBIDDEN -> FetchException(ErrKind.ForbiddenScope, code.ifEmpty { "forbidden_scope" })
        HTTP_NOT_FOUND, HTTP_METHOD_NOT_ALLOWED, HTTP_PAYLOAD_TOO_LARGE ->
            FetchException(ErrKind.Protocol, code.ifEmpty { "http_$httpStatus" })
        else -> null
    }

private fun toolKind(code: String): ErrKind? =
    when (code) {
        "unauthorized" -> ErrKind.Unauthorized
        "forbidden_scope" -> ErrKind.ForbiddenScope
        "session_required" -> ErrKind.SessionRequired
        "session_forbidden" -> ErrKind.SessionForbidden
        "invalid_args" -> ErrKind.SessionMissing
        "tmux_error", "internal" -> ErrKind.HostError
        in OPERATION_CODES -> ErrKind.Operation
        else -> null
    }

/**
 * ペインを引数に取るツール（`sessions_list`・`report` 以外）が isError で返した失敗を分類する。通常は [classify] と同じだが、
 * `invalid_args` は `details.field` が `session`（sessions に無い名前）のときだけ [ErrKind.SessionMissing]、
 * それ以外（ペイン・入力の不正、field なし）は [ErrKind.Operation]。[detailsField] は応答由来の信頼できない値で、
 * `[a-z_]{1,32}` に合うときだけ判定に使い、画面・ログには出さない。デスクトップ版の `ops.Failure` と同じ分類。
 */
fun classifyPaneToolFailure(
    toolCode: String,
    detailsField: String,
): FetchException? {
    if (toolCode == "invalid_args" && sanitizeField(detailsField) != "session") return FetchException(ErrKind.Operation, "invalid_args")
    return classify(null, HTTP_OK, toolCode)
}

private const val HTTP_OK = 200
private const val MAX_FIELD_LENGTH = 32

private fun sanitizeField(text: String): String =
    if (text.length in 1..MAX_FIELD_LENGTH && text.all { it in 'a'..'z' || it == '_' }) text else ""

/**
 * 操作ツールの失敗コード（allow-list）。これ以外の未知のコードは [ErrKind.Protocol]。`invalid_args` はここに含めない:
 * [classify] は従来どおり [ErrKind.SessionMissing] で、ペインを引数に取るツールでは [classifyPaneToolFailure] が
 * `details.field` を見て [ErrKind.Operation] に分ける。
 */
val OPERATION_CODES: Set<String> =
    setOf(
        "invalid_state",
        "not_asking",
        "option_out_of_range",
        "custom_not_allowed",
        "cursor_mismatch",
        "confirm_required",
        "forbidden_control_char",
        "text_too_long",
        "forbidden_path",
        "path_forbidden",
        "file_unreadable",
        "unsupported_agent",
        "pane_not_found",
        "busy_timeout",
        "exit_timeout",
        "not_found",
        "unknown_tool",
    )

// 応答由来のエラーコードを `[a-z0-9_]{1,64}` に限る。合わなければ空文字。
private fun sanitizeCode(text: String): String =
    if (text.length in 1..MAX_CODE_LENGTH && text.all { it in 'a'..'z' || it in '0'..'9' || it == '_' }) text else ""
