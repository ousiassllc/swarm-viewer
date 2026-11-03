package service

import (
	"errors"
	"strings"
)

var (
	// ErrNoSessions は許可されたセッションが 0 件（ResolveSession）。
	ErrNoSessions = errors.New("service: no allowed sessions")
	// ErrDraftExpired はペアリング下書きが期限切れ・破棄済み・未知の ID（Draft・Confirm）。
	ErrDraftExpired = errors.New("service: pairing draft expired or unknown")
	// ErrConnectionNotFound は指定した接続がない。
	ErrConnectionNotFound = errors.New("service: connection not found")
)

// ValidationError は入力の検証失敗（Settings の Update・Pairing の Parse）。422 でフォームを再描画する。
// Fields は不正な項目名。Err は原因（errors.Is / errors.As で domain の種別を取り出せる）。
// Error() は固定文言で、入力値（token を含みうる）を含めない。
type ValidationError struct {
	Fields []string
	Err    error
}

// Error は項目名だけの固定文言を返す。
func (e *ValidationError) Error() string {
	return "service: validation failed: " + strings.Join(e.Fields, ",")
}

// Unwrap は原因を返す。
func (e *ValidationError) Unwrap() error { return e.Err }
