package paneservice_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

var startReq = ops.StartRequest{Agent: "omp", Cwd: "/w", Title: "t"}

func TestStart_FirstTaskIsSentAfterPaneStartWhenIdle(t *testing.T) {
	cases := []struct {
		name  string
		first ops.FirstTask
		want  string
	}{
		{"skill and task", ops.FirstTask{Skill: "/impl", Task: "#12 をやる"}, "/impl #12 をやる"},
		{"skill only", ops.FirstTask{Skill: "/impl"}, "/impl"},
		{"task only", ops.FirstTask{Task: "a\nb"}, "a\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
			out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, tc.first)
			if err != nil || out.Task != paneservice.TaskSent || !out.Send.Accepted {
				t.Fatalf("out = %+v, err = %v", out, err)
			}
			if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start", "send_text"}) {
				t.Errorf("calls = %v; want pane_start → send_text", got)
			}
			if !slices.Equal(f.cli.texts, []string{tc.want}) {
				t.Errorf("送った本文 = %q; want %q", f.cli.texts, tc.want)
			}
		})
	}
}

func TestStart_WithoutFirstTaskOnlyStarts(t *testing.T) {
	f := newFixture(t)
	f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
	for _, first := range []ops.FirstTask{{}, {Skill: " ", Task: "\n "}} {
		out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		if err != nil || out.Task != paneservice.TaskNone || !out.Start.OK {
			t.Fatalf("out = %+v, err = %v", out, err)
		}
	}
	if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start", "pane_start"}) {
		t.Errorf("calls = %v", got)
	}
}

func TestStart_FirstTaskIsNotSentWhenStartFailsOrIsNotIdle(t *testing.T) {
	first := ops.FirstTask{Task: "x"}
	t.Run("ok:false", func(t *testing.T) {
		f := newFixture(t)
		f.cli.startRes = &ops.StartResult{OK: false, State: "shell"}
		out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		if err != nil || out.Task != paneservice.TaskNone || out.Start.OK {
			t.Fatalf("out = %+v, err = %v", out, err)
		}
		if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("not idle", func(t *testing.T) {
		f := newFixture(t)
		f.cli.startRes = &ops.StartResult{OK: true, State: "working"}
		out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		if err != nil || out.Task != paneservice.TaskNotIdle || !out.Start.OK || out.Start.State != "working" {
			t.Fatalf("out = %+v, err = %v", out, err)
		}
		if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("pane_start error", func(t *testing.T) {
		f := newFixture(t)
		f.cli.err = &domain.FetchError{Kind: domain.ErrOperation, Code: "forbidden_path"}
		_, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		var fe *domain.FetchError
		if !errors.As(err, &fe) || fe.Code != "forbidden_path" {
			t.Fatalf("err = %v", err)
		}
		if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v", got)
		}
	})
}

func TestStart_InvalidFirstTaskIsRejectedBeforeAnyCall(t *testing.T) {
	for name, tc := range map[string]struct {
		first ops.FirstTask
		code  string
	}{
		"control char": {ops.FirstTask{Task: "a\x1bb"}, "forbidden_control_char"},
		"too long":     {ops.FirstTask{Skill: "/impl", Task: strings.Repeat("あ", ops.MaxSendChars)}, "text_too_long"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, tc.first)
			var fe *domain.FetchError
			if !errors.As(err, &fe) || fe.Kind != domain.ErrOperation || fe.Code != tc.code {
				t.Fatalf("err = %v; want %s", err, tc.code)
			}
			if got := f.cli.Calls(); len(got) != 0 {
				t.Errorf("calls = %v; want なし", got)
			}
		})
	}
}

func TestStart_SendFailureKeepsTheStartAndNeverRetriesOrCloses(t *testing.T) {
	first := ops.FirstTask{Task: "x"}
	t.Run("send_text error", func(t *testing.T) {
		f := newFixture(t)
		f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
		f.cli.sendErr = &domain.FetchError{Kind: domain.ErrOperation, Code: "invalid_state"}
		out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		var fe *domain.FetchError
		if err != nil || out.Task != paneservice.TaskFailed || !errors.As(out.Err, &fe) || fe.Code != "invalid_state" || !out.Start.OK {
			t.Fatalf("out = %+v, err = %v", out, err)
		}
		if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start", "send_text"}) {
			t.Errorf("calls = %v; 自動の再送・pane_close をしない", got)
		}
	})
	t.Run("accepted:false", func(t *testing.T) {
		f := newFixture(t)
		f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
		f.cli.sendRes = &ops.SendResult{OK: true, Accepted: false}
		out, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first)
		if err != nil || out.Task != paneservice.TaskFailed || out.Err != nil {
			t.Fatalf("out = %+v, err = %v", out, err)
		}
		if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start", "send_text"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("unauthorized marks needs_repair", func(t *testing.T) {
		f := newFixture(t)
		f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
		f.cli.sendErr = &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "x"}
		if _, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, first); err != nil {
			t.Fatal(err)
		}
		if !f.env.Conns.NeedsRepair("ops") {
			t.Error("needs_repair が立たない")
		}
	})
}

func TestStart_HoldsThePaneExclusivelyAcrossStartAndSend(t *testing.T) {
	f := newFixture(t)
	f.cli.startRes = &ops.StartResult{OK: true, State: "idle"}
	f.cli.gate, f.cli.entered = make(chan struct{}), make(chan struct{}, 4)
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.Start(t.Context(), "ops", "0", "T1", startReq, ops.FirstTask{Task: "x"})
		done <- err
	}()
	servicetest.Recv(t, f.cli.entered, "最初のタスクの send_text")

	// pane_start は済み、send_text の途中でも、同じペインは排他のまま（別の操作・再送は通信せずに ErrBusy）。
	if _, err := f.svc.Send(t.Context(), "ops", "0", "T1", "other"); !errors.Is(err, paneservice.ErrBusy) {
		t.Errorf("Send: err = %v; want ErrBusy", err)
	}
	if _, err := f.svc.Resend(t.Context(), "ops", "0", "T1", ops.FirstTask{Task: "x"}); !errors.Is(err, paneservice.ErrBusy) {
		t.Errorf("Resend: err = %v; want ErrBusy", err)
	}
	close(f.cli.gate)
	if err := servicetest.Recv(t, done, "Start の完了"); err != nil {
		t.Fatal(err)
	}
	if got := f.cli.Calls(); !slices.Equal(got, []string{"pane_start", "send_text"}) {
		t.Errorf("calls = %v", got)
	}
}

func TestResend_SendsOnlySendTextAndNeverPaneStart(t *testing.T) {
	f := newFixture(t)
	res, err := f.svc.Resend(t.Context(), "ops", "0", "T1", ops.FirstTask{Skill: "/impl", Task: "go"})
	if err != nil || !res.Accepted {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if got := f.cli.Calls(); !slices.Equal(got, []string{"send_text"}) || !slices.Equal(f.cli.texts, []string{"/impl go"}) {
		t.Errorf("calls = %v, texts = %q", got, f.cli.texts)
	}
}

func TestResend_EmptyOrInvalidTaskIsRejectedWithoutCommunicating(t *testing.T) {
	f := newFixture(t)
	for _, first := range []ops.FirstTask{{}, {Task: "a\x00"}} {
		var fe *domain.FetchError
		if _, err := f.svc.Resend(t.Context(), "ops", "0", "T1", first); !errors.As(err, &fe) || fe.Kind != domain.ErrOperation {
			t.Errorf("first = %+v: err = %v", first, err)
		}
	}
	if got := f.cli.Calls(); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}

func TestSkills_DoNotTakeThePaneLock(t *testing.T) {
	f := newFixture(t)
	f.cli.gate, f.cli.entered = make(chan struct{}), make(chan struct{}, 4)
	done := make(chan error, 1)
	go func() { _, err := f.svc.Send(t.Context(), "ops", "0", "T1", "x"); done <- err }()
	servicetest.Recv(t, f.cli.entered, "send_text")
	if _, err := f.svc.Skills(t.Context(), "ops", "omp"); err != nil {
		t.Errorf("Skills: %v", err)
	}
	close(f.cli.gate)
	if err := servicetest.Recv(t, done, "Send の完了"); err != nil {
		t.Fatal(err)
	}
}
