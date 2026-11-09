package format_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
)

func TestOperationMessage_AllTargetCodesHaveFixedTextAndUnknownFallsBack(t *testing.T) {
	t.Parallel()
	codes := []string{
		"invalid_state", "not_asking", "option_out_of_range", "custom_not_allowed", "cursor_mismatch", "confirm_required",
		"forbidden_control_char", "text_too_long", "forbidden_path", "path_forbidden", "file_unreadable", "unsupported_agent",
		"pane_not_found", "busy_timeout", "exit_timeout", "not_found", "unknown_tool", "invalid_args",
	}
	for _, code := range codes {
		msg := format.OperationMessage(code)
		if msg == "" || strings.Contains(msg, code) {
			t.Errorf("%s: 文言 = %q", code, msg)
		}
		fe := &domain.FetchError{Kind: domain.ErrOperation, Code: code}
		if format.FetchErrorMessage(fe) != msg || format.OperationFailureMessage(fe) != msg {
			t.Errorf("%s: FetchErrorMessage と OperationFailureMessage が OperationMessage と違う", code)
		}
	}
	// 未知のコード・本文らしき文字列は、画面に出さず invalid_args と同じ文言。
	secret := "secret token=test-token-not-secret"
	if got := format.OperationMessage(secret); got != format.OperationMessage("invalid_args") || strings.Contains(got, "secret") {
		t.Errorf("未知のコード: %q", got)
	}
	// operation 以外は従来の文言。
	if got := format.OperationFailureMessage(&domain.FetchError{Kind: domain.ErrUnreachable}); got != format.FetchErrorMessage(&domain.FetchError{Kind: domain.ErrUnreachable}) {
		t.Errorf("unreachable = %q", got)
	}
}

func TestToasts_FollowOkAcceptedCardClosedAndInterrupted(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, got, want string }{
		{"pick 確定", format.PickToast(ops.PickResult{OK: true, CardClosed: true}), "送りました"},
		{"pick ok だが閉じない", format.PickToast(ops.PickResult{OK: true}), "選択を確定できませんでした。もう一度お試しください"},
		{"pick ok:false", format.PickToast(ops.PickResult{}), "選択を確定できませんでした。もう一度お試しください"},
		{"send 受理", format.SendToast(ops.SendResult{OK: true, Accepted: true}), "送りました"},
		{"send 受理されない", format.SendToast(ops.SendResult{OK: false}), "受け付けられませんでした"},
		{"escape 中断", format.EscapeToast(ops.EscapeResult{OK: true, Interrupted: true}), "中断しました"},
		{"escape 変わらない", format.EscapeToast(ops.EscapeResult{OK: false}), "状態が変わりませんでした"},
		{"start ok", format.StartToast(ops.StartResult{OK: true}), "起動しました"},
		{"start ok:false", format.StartToast(ops.StartResult{State: "shell"}), "起動を確認できませんでした（状態: shell）"},
		{"close ok", format.CloseToast(ops.CloseResult{OK: true}), "終了しました"},
		{"close ok:false", format.CloseToast(ops.CloseResult{State: "idle"}), "片付けを確認できませんでした（状態: idle）"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: %q; want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestPaneStateGates_SendEscapeClose(t *testing.T) {
	t.Parallel()
	states := []domain.PaneState{domain.StateWorking, domain.StateAsking, domain.StateIdle, domain.StateStalled, domain.StateShell, domain.StateUnknown}
	send := map[domain.PaneState]bool{domain.StateIdle: true, domain.StateStalled: true}
	escape := map[domain.PaneState]bool{domain.StateWorking: true, domain.StateStalled: true, domain.StateAsking: true, domain.StateIdle: true}
	closeOK := map[domain.PaneState]bool{domain.StateIdle: true, domain.StateShell: true}
	reasons := map[domain.PaneState]string{
		domain.StateWorking: "作業中のため送れません。終わるまで待つか、中断してください",
		domain.StateAsking:  "質問に答えてから送れます",
		domain.StateShell:   "エージェントが動いていません。「始める」から起動してください",
		domain.StateUnknown: "このペインには送れません",
	}
	for _, s := range states {
		if format.CanSend(s) != send[s] || format.CanEscape(s) != escape[s] || format.CanClose(s) != closeOK[s] {
			t.Errorf("%s: send=%v escape=%v close=%v", s, format.CanSend(s), format.CanEscape(s), format.CanClose(s))
		}
		if got := format.SendBlockedReason(s); got != reasons[s] {
			t.Errorf("%s: 理由 = %q; want %q", s, got, reasons[s])
		}
	}
}

func TestKindLabel_UsesTextNotOnlyColor(t *testing.T) {
	t.Parallel()
	if format.KindLabel(domain.KindOps) != "操作" || format.KindLabel(domain.KindView) != "見るだけ" || format.KindLabel("") != "見るだけ" {
		t.Error("種類のバッジの文言")
	}
	if format.KindTone(domain.KindOps) == format.KindTone(domain.KindView) {
		t.Error("種類で色も変える")
	}
}

func TestSkillListFailureMessage_UnknownToolHasNoReloadOthersDo(t *testing.T) {
	t.Parallel()
	msg, reload := format.SkillListFailureMessage(&domain.FetchError{Kind: domain.ErrOperation, Code: "unknown_tool"})
	if msg != "スキルの一覧を取得できません（ホストの swarm-console が古い可能性があります）" || reload {
		t.Errorf("unknown_tool: %q, reload=%v", msg, reload)
	}
	for _, err := range []error{
		&domain.FetchError{Kind: domain.ErrOperation, Code: "invalid_args"},
		&domain.FetchError{Kind: domain.ErrUnreachable},
		&domain.FetchError{Kind: domain.ErrProtocol},
		domain.ErrInvalidResponse,
	} {
		msg, reload := format.SkillListFailureMessage(err)
		if msg != "スキルの一覧を読み込めませんでした" || !reload {
			t.Errorf("%v: %q, reload=%v", err, msg, reload)
		}
	}
}

func TestFirstTaskMessages(t *testing.T) {
	t.Parallel()
	if got := format.FirstTaskNotIdleMessage("working"); got != "起動しました。最初のタスクはまだ送っていません（状態: working）。ペイン詳細から送れます" {
		t.Errorf("not idle = %q", got)
	}
	if format.FirstTaskFailedMessage != "起動しました。最初のタスクを送れませんでした" {
		t.Errorf("failed = %q", format.FirstTaskFailedMessage)
	}
}
