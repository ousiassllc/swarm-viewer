package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	timeout := context.DeadlineExceeded
	dns := &net.DNSError{Err: "no such host", Name: "x", IsNotFound: true}
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		want   *FetchError // nil は nil を期待
	}{
		// 到達不能
		{"タイムアウト", timeout, 0, "", &FetchError{ErrUnreachable, ""}},
		{"DNS", dns, 0, "", &FetchError{ErrUnreachable, ""}},
		{"route なし", &net.OpError{Op: "dial", Err: errors.New("no route to host")}, 0, "", &FetchError{ErrUnreachable, ""}},
		{"ラップされたネットワークエラー", fmt.Errorf("do: %w", dns), 0, "", &FetchError{ErrUnreachable, ""}},
		// HTTP
		{"401", errors.New("status"), 401, "unauthorized", &FetchError{ErrUnauthorized, "unauthorized"}},
		{"401 コードなし", nil, 401, "", &FetchError{ErrUnauthorized, "unauthorized"}},
		{"403", nil, 403, "forbidden_scope", &FetchError{ErrForbiddenScope, "forbidden_scope"}},
		{"403 コードなし", nil, 403, "", &FetchError{ErrForbiddenScope, "forbidden_scope"}},
		{"404", nil, 404, "", &FetchError{ErrProtocol, "http_404"}},
		{"405", nil, 405, "", &FetchError{ErrProtocol, "http_405"}},
		{"413", nil, 413, "", &FetchError{ErrProtocol, "http_413"}},
		{"404 + コード", nil, 404, "not_found", &FetchError{ErrProtocol, "not_found"}},
		{"500", nil, 500, "", &FetchError{ErrProtocol, "http_500"}},
		{"429", nil, 429, "", &FetchError{ErrProtocol, "http_429"}},
		{"400 invalid_args はセッション不在", nil, 400, "invalid_args", &FetchError{ErrSessionMissing, "invalid_args"}},
		{"500 internal はホスト側エラー", nil, 500, "internal", &FetchError{ErrHostError, "internal"}},
		// ツールコード（HTTP 200）
		{"session_required", nil, 200, "session_required", &FetchError{ErrSessionRequired, "session_required"}},
		{"session_forbidden", nil, 200, "session_forbidden", &FetchError{ErrSessionForbidden, "session_forbidden"}},
		{"invalid_args", nil, 200, "invalid_args", &FetchError{ErrSessionMissing, "invalid_args"}},
		{"tmux_error", nil, 200, "tmux_error", &FetchError{ErrHostError, "tmux_error"}},
		{"internal", nil, 200, "internal", &FetchError{ErrHostError, "internal"}},
		{"status 0 のツールコード", nil, 0, "tmux_error", &FetchError{ErrHostError, "tmux_error"}},
		{"未知のコード", nil, 200, "some_new_code", &FetchError{ErrProtocol, "some_new_code"}},
		{"unauthorized ツールコード", nil, 200, "unauthorized", &FetchError{ErrUnauthorized, "unauthorized"}},
		// 形が想定外
		{"JSON が想定外", ErrInvalidResponse, 200, "", &FetchError{ErrProtocol, ""}},
		{"ラップされた ErrInvalidResponse", fmt.Errorf("x: %w", ErrInvalidResponse), 200, "", &FetchError{ErrProtocol, ""}},
		// 信頼できないコードは捨てる
		{"コードに本文らしき文字列", nil, 200, "bad code: token=test-token-not-secret", &FetchError{ErrProtocol, ""}},
		{"コードが長すぎる", nil, 200, strings.Repeat("a", 65), &FetchError{ErrProtocol, ""}},
		{"コード 64 文字は可", nil, 200, strings.Repeat("a", 64), &FetchError{ErrProtocol, strings.Repeat("a", 64)}},
		{"大文字コードは不可", nil, 200, "Tmux_Error", &FetchError{ErrProtocol, ""}},
		{"401 に不正コード", nil, 401, "<script>", &FetchError{ErrUnauthorized, "unauthorized"}},
		// 成功
		{"成功", nil, 200, "", nil},
		{"成功 (204)", nil, 204, "", nil},
		// FetchError の素通し
		{"既に FetchError", fmt.Errorf("w: %w", &FetchError{Kind: ErrSessionForbidden, Code: "session_forbidden"}), 0, "", &FetchError{ErrSessionForbidden, "session_forbidden"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Classify(tt.err, tt.status, tt.code)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v; want nil", got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("got %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestFetchErrorAs(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("fetch: %w", Classify(nil, 401, "unauthorized"))
	var fe *FetchError
	if !errors.As(err, &fe) || fe.Kind != ErrUnauthorized {
		t.Fatalf("errors.As 失敗: %v", err)
	}
}

func TestFetchErrorText(t *testing.T) {
	t.Parallel()
	for kind, name := range map[ErrKind]string{
		ErrUnreachable: "unreachable", ErrUnauthorized: "unauthorized", ErrForbiddenScope: "forbidden_scope",
		ErrSessionRequired: "session_required", ErrSessionForbidden: "session_forbidden",
		ErrSessionMissing: "session_missing", ErrHostError: "host_error", ErrProtocol: "protocol", ErrOperation: "operation",
	} {
		if kind.String() != name {
			t.Errorf("%d.String() = %q; want %q", kind, kind.String(), name)
		}
		msg := (&FetchError{Kind: kind}).Error()
		if !strings.Contains(msg, name) {
			t.Errorf("Error() = %q は種別名 %q を含む", msg, name)
		}
		withCode := (&FetchError{Kind: kind, Code: "c_1"}).Error()
		if !strings.Contains(withCode, name) || !strings.Contains(withCode, "c_1") {
			t.Errorf("Error() = %q は種別名と Code を含む", withCode)
		}
	}
	if ErrKind(99).String() != "unknown" || ErrKind(-1).String() != "unknown" {
		t.Error("範囲外は unknown")
	}
	// iota の並びは仕様どおり
	if ErrUnreachable != 0 || ErrProtocol != 7 || ErrOperation != 8 {
		t.Error("ErrKind の並びが仕様と違う")
	}
}
