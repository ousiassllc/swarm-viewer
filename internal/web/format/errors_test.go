package format_test

import (
	"errors"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
)

func TestFetchErrorMessage_AllKindsAreFixedText(t *testing.T) {
	t.Parallel()
	kinds := []domain.ErrKind{
		domain.ErrUnreachable, domain.ErrUnauthorized, domain.ErrForbiddenScope, domain.ErrSessionRequired,
		domain.ErrSessionForbidden, domain.ErrSessionMissing, domain.ErrHostError, domain.ErrProtocol,
	}
	seen := map[string]domain.ErrKind{}
	for _, k := range kinds {
		msg := format.FetchErrorMessage(&domain.FetchError{Kind: k, Code: "secret-looking-code"})
		if msg == "" {
			t.Errorf("%v: 文言が空", k)
		}
		if msg != format.FetchErrorMessage(&domain.FetchError{Kind: k}) {
			t.Errorf("%v: Code が文言に影響している", k)
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("%v と %v が同じ文言", prev, k)
		}
		seen[msg] = k
	}
	if format.FetchErrorMessage(nil) != "" {
		t.Error("nil は空")
	}
	if format.FetchErrorMessage(&domain.FetchError{Kind: 99}) != format.FetchErrorMessage(&domain.FetchError{Kind: domain.ErrProtocol}) {
		t.Error("範囲外の Kind は protocol と同じ")
	}
}

func TestNeedsRepair(t *testing.T) {
	t.Parallel()
	if !format.NeedsRepair(&domain.FetchError{Kind: domain.ErrUnauthorized}) || !format.NeedsRepair(&domain.FetchError{Kind: domain.ErrForbiddenScope}) {
		t.Error("401・403 は再ペアリングが必要")
	}
	if format.NeedsRepair(nil) || format.NeedsRepair(&domain.FetchError{Kind: domain.ErrUnreachable}) {
		t.Error("それ以外は不要")
	}
}

func TestPairingErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{domain.ErrPairingVersion, "アプリを更新してください"},
		{domain.ErrInvalidURL, "接続先が許可された範囲外です（Tailscale の IP のみ）"},
		{domain.ErrPairingToken, "token が含まれていません"},
		{domain.ErrPairingFormat, "接続文字列の形式が正しくありません"},
		{errors.New("something else"), "接続文字列の形式が正しくありません"},
	}
	for _, tt := range tests {
		if got := format.PairingErrorMessage(tt.err); got != tt.want {
			t.Errorf("PairingErrorMessage(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestJoinNonEmpty(t *testing.T) {
	t.Parallel()
	if got := format.JoinNonEmpty(" / ", "", "a", "", "b"); got != "a / b" {
		t.Errorf("got %q", got)
	}
	if got := format.JoinNonEmpty(" / "); got != "" {
		t.Errorf("got %q", got)
	}
}
