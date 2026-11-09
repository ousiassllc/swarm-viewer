package vectors_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

func TestConnectionKind_IsDerivedFromTheURLPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  string
		want domain.ConnectionKind
	}{
		{"http://100.64.0.5:48120/ops", domain.KindOps},
		{"http://127.0.0.1:8080/ops", domain.KindOps},
		{"http://100.64.0.5:48120/report", domain.KindView},
		// 判定できない URL・別表記は見るだけ（操作を許す方向に倒さない）。
		{"", domain.KindView},
		{"::not a url", domain.KindView},
		{"http://100.64.0.5:48120/OPS", domain.KindView},
		{"http://100.64.0.5:48120/ops/", domain.KindView},
		{"http://100.64.0.5:48120/%6fps", domain.KindView},
		{"http://100.64.0.5:48120/ops/../report", domain.KindView},
	}
	for _, tc := range tests {
		if got := (domain.Connection{URL: tc.url}).Kind(); got != tc.want {
			t.Errorf("Kind(%q) = %q; want %q", tc.url, got, tc.want)
		}
	}
}

func TestValidateURL_KeepsTheOpsPathInTheNormalizedURL(t *testing.T) {
	t.Parallel()
	got, err := domain.ValidateURL("  HTTP://100.64.0.5:48120/ops ")
	if err != nil || got != "http://100.64.0.5:48120/ops" {
		t.Fatalf("ValidateURL = %q, %v", got, err)
	}
	if domain.KindOfURL(got) != domain.KindOps {
		t.Error("正規化後の URL でも操作の接続")
	}
}

// 操作ツールの invalid_args は、details.field が session のときだけ session_missing。ペイン・入力の不正は operation。
func TestOperationFailure_InvalidArgsIsOperationUnlessItNamesTheSession(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code, field string
		want        domain.ErrKind
	}{
		{"invalid_args", "session", domain.ErrSessionMissing},
		{"invalid_args", "pane", domain.ErrOperation},
		{"invalid_args", "", domain.ErrOperation},
		{"invalid_state", "", domain.ErrOperation},
		{"session_forbidden", "", domain.ErrSessionForbidden},
		{"tmux_error", "", domain.ErrHostError},
		{"something_new", "", domain.ErrProtocol},
	}
	for _, tc := range tests {
		fe := ops.Failure(tc.code, tc.field)
		if fe == nil || fe.Kind != tc.want || fe.Code != tc.code {
			t.Errorf("Failure(%q, %q) = %v; want %v/%s", tc.code, tc.field, fe, tc.want, tc.code)
		}
	}
}

func TestOperationError_IsAnOperationFetchErrorWithoutBody(t *testing.T) {
	t.Parallel()
	err := error(ops.Error("confirm_required"))
	var fe *domain.FetchError
	if !errors.As(err, &fe) || fe.Kind != domain.ErrOperation || fe.Code != "confirm_required" {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(err.Error(), "operation (confirm_required)") {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestPickCheck_RequiresExactlyOneOfNumberOrText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pick ops.Pick
		want string
	}{
		{"番号", ops.Pick{N: 2}, ""},
		{"その他", ops.Pick{Text: "x"}, ""},
		{"どちらも無い", ops.Pick{}, "invalid_args"},
		{"同時指定", ops.Pick{N: 1, Text: "x"}, "invalid_args"},
		{"0 以下の番号", ops.Pick{N: -1}, "invalid_args"},
		{"その他が長すぎる", ops.Pick{Text: strings.Repeat("a", 8001)}, "text_too_long"},
	}
	for _, tc := range tests {
		if got := tc.pick.Check(); got != tc.want {
			t.Errorf("%s: Check() = %q; want %q", tc.name, got, tc.want)
		}
	}
}

func TestStartRequestNormalize_RequiresKnownAgentCwdAndTitle(t *testing.T) {
	t.Parallel()
	for _, agent := range ops.StartAgents {
		if _, code := (ops.StartRequest{Agent: agent, Cwd: "/w", Title: "t"}).Normalize(); code != "" {
			t.Errorf("%s: code = %q", agent, code)
		}
	}
	bad := []ops.StartRequest{
		{Agent: "", Cwd: "/w", Title: "t"},
		{Agent: "vim", Cwd: "/w", Title: "t"},
		{Agent: "omp", Cwd: "", Title: "t"},
		{Agent: "omp", Cwd: "/w\n/x", Title: "t"},
		{Agent: "omp", Cwd: "/w", Title: ""},
	}
	for _, r := range bad {
		if _, code := r.Normalize(); code != "invalid_args" {
			t.Errorf("%+v: code = %q; want invalid_args", r, code)
		}
	}
}
