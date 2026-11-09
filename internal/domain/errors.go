package domain

import (
	"errors"
	"net/http"
	"strconv"
)

// ErrKind は swarm-console の取得失敗の種別（API 仕様 §1.5）。
type ErrKind int

// 取得失敗の種別。
const (
	ErrUnreachable ErrKind = iota
	ErrUnauthorized
	ErrForbiddenScope
	ErrSessionRequired
	ErrSessionForbidden
	ErrSessionMissing
	ErrHostError
	ErrProtocol
	// ErrOperation は操作ツール（`/ops`）の失敗（状態の不一致・入力の拒否など。HTTP 200 の isError）。Code で理由を区別する。
	ErrOperation
)

var kindNames = [...]string{
	ErrUnreachable:      "unreachable",
	ErrUnauthorized:     "unauthorized",
	ErrForbiddenScope:   "forbidden_scope",
	ErrSessionRequired:  "session_required",
	ErrSessionForbidden: "session_forbidden",
	ErrSessionMissing:   "session_missing",
	ErrHostError:        "host_error",
	ErrProtocol:         "protocol",
	ErrOperation:        "operation",
}

// String は種別名（ログ・固定文言用）。範囲外は "unknown"。
func (k ErrKind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return "unknown"
	}
	return kindNames[k]
}

// ErrInvalidResponse は応答の JSON が壊れている・形が想定外のときの ParseReport / ParseSessions の失敗。
// 応答本文は含まない。Classify は ErrProtocol に分類する。
var ErrInvalidResponse = errors.New("swarm-console: invalid response")

// FetchError は swarm-console の失敗。errors.As で判定する。
// Code は swarm-console のエラーコード。message・details は持たない（NFR-LOG-3）。
type FetchError struct {
	Kind ErrKind
	Code string
}

// Error は種別名と Code だけの固定文言を返す。応答本文は含めない。
func (e *FetchError) Error() string {
	if e.Code == "" {
		return "swarm-console fetch error: " + e.Kind.String()
	}
	return "swarm-console fetch error: " + e.Kind.String() + " (" + e.Code + ")"
}

// Classify は失敗を FetchError に分類する（API 仕様 §1.5）。成功と判断できるとき（err が nil で、
// HTTP が 4xx/5xx でなく、toolCode も空）は nil を返す。
//
//   - err が FetchError を含む場合はそれをそのまま返す
//   - HTTP 401 → ErrUnauthorized、403 → ErrForbiddenScope、404・405・413 → ErrProtocol
//   - ツールコード（HTTP 200 の isError、または HTTP エラー本文の code）:
//     session_required / session_forbidden / invalid_args（session が sessions にない）/
//     tmux_error・internal → ErrHostError。操作ツールの失敗（operationCodes。invalid_state・not_asking など）→ ErrOperation。ほかは ErrProtocol
//   - HTTP が上記以外の 4xx/5xx で toolCode が写像に無ければ ErrProtocol
//   - HTTP の応答もツールコードも無い err（タイムアウト・DNS・route なし）→ ErrUnreachable
//   - err が ErrInvalidResponse → ErrProtocol
//
// toolCode は応答本文由来の信頼できない値のため、`[a-z0-9_]{1,64}` に合わなければ Code に入れない（種別は ErrProtocol）。
// HTTP エラーで Code が空のときは `http_<status>` を入れる（ログにステータスコードを残すため）。
func Classify(err error, httpStatus int, toolCode string) *FetchError {
	var existing *FetchError
	if errors.As(err, &existing) && existing != nil {
		return existing
	}
	code := sanitizeCode(toolCode)
	switch httpStatus {
	case http.StatusUnauthorized:
		return &FetchError{Kind: ErrUnauthorized, Code: codeOr(code, "unauthorized")}
	case http.StatusForbidden:
		return &FetchError{Kind: ErrForbiddenScope, Code: codeOr(code, "forbidden_scope")}
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge:
		return &FetchError{Kind: ErrProtocol, Code: codeOr(code, httpCode(httpStatus))}
	}
	if code != "" {
		if kind, ok := toolKind(code); ok {
			return &FetchError{Kind: kind, Code: code}
		}
		return &FetchError{Kind: ErrProtocol, Code: code}
	}
	if httpStatus >= http.StatusBadRequest {
		return &FetchError{Kind: ErrProtocol, Code: httpCode(httpStatus)}
	}
	if toolCode != "" {
		return &FetchError{Kind: ErrProtocol} // 不正なコードは捨て、種別だけ残す
	}
	if errors.Is(err, ErrInvalidResponse) {
		return &FetchError{Kind: ErrProtocol}
	}
	if err != nil {
		return &FetchError{Kind: ErrUnreachable}
	}
	return nil
}

func toolKind(code string) (ErrKind, bool) {
	switch code {
	case "unauthorized":
		return ErrUnauthorized, true
	case "forbidden_scope":
		return ErrForbiddenScope, true
	case "session_required":
		return ErrSessionRequired, true
	case "session_forbidden":
		return ErrSessionForbidden, true
	case "invalid_args":
		return ErrSessionMissing, true
	case "tmux_error", "internal":
		return ErrHostError, true
	default:
		if _, ok := operationCodes[code]; ok {
			return ErrOperation, true
		}
		return 0, false
	}
}

// operationCodes は ErrOperation に分類する操作ツールのエラーコード（allow-list。契約 §2）。
// invalid_args は含めない（Classify は session が sessions にない意味の session_missing に写す。
// 操作ツールの invalid_args は ops.Failure が ErrOperation にする）。
var operationCodes = map[string]struct{}{
	"invalid_state": {}, "not_asking": {}, "option_out_of_range": {}, "custom_not_allowed": {},
	"cursor_mismatch": {}, "confirm_required": {}, "forbidden_control_char": {}, "text_too_long": {},
	"forbidden_path": {}, "path_forbidden": {}, "file_unreadable": {}, "unsupported_agent": {},
	"pane_not_found": {}, "busy_timeout": {}, "exit_timeout": {}, "not_found": {}, "unknown_tool": {},
}

func httpCode(status int) string { return "http_" + strconv.Itoa(status) }

func codeOr(code, fallback string) string {
	if code != "" {
		return code
	}
	return fallback
}

// sanitizeCode は応答由来のエラーコードを `[a-z0-9_]{1,64}` に限る。合わなければ空文字。
func sanitizeCode(s string) string {
	if len(s) == 0 || len(s) > 64 {
		return ""
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return ""
		}
	}
	return s
}
