package paneservice_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

// fakeClient は paneservice.Client のフェイク。呼ばれたツール名を記録し、gate を設定した呼び出しはそこで待つ。
type fakeClient struct {
	mu      sync.Mutex
	calls   []string
	err     error
	gate    chan struct{} // 非 nil の間、SendText は閉じるまで待つ
	entered chan struct{}

	// 最初のタスクのテスト用。startRes が非 nil なら PaneStart の結果、sendRes・sendErr が非 nil なら SendText の結果。
	startRes *ops.StartResult
	sendRes  *ops.SendResult
	sendErr  error
	texts    []string // SendText で送られた本文
}

func (f *fakeClient) record(tool string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, tool)
	return f.err
}

func (f *fakeClient) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeClient) PaneTail(context.Context, domain.Connection, string, string, int) (ops.PaneTail, error) {
	return ops.PaneTail{Text: "t"}, f.record("pane_tail")
}

func (f *fakeClient) AskCard(context.Context, domain.Connection, string, string) (ops.AskCard, error) {
	return ops.AskCard{}, f.record("ask_card")
}

func (f *fakeClient) AskPick(context.Context, domain.Connection, string, string, ops.Pick) (ops.PickResult, error) {
	return ops.PickResult{OK: true}, f.record("ask_pick")
}

func (f *fakeClient) SendText(_ context.Context, _ domain.Connection, _, _, text string) (ops.SendResult, error) {
	if f.entered != nil {
		f.entered <- struct{}{}
	}
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	f.texts = append(f.texts, text)
	res, sendErr := ops.SendResult{OK: true, Accepted: true}, f.sendErr
	if f.sendRes != nil {
		res = *f.sendRes
	}
	f.mu.Unlock()
	if err := f.record("send_text"); err != nil {
		return res, err
	}
	return res, sendErr
}

func (f *fakeClient) SendEscape(context.Context, domain.Connection, string, string, bool) (ops.EscapeResult, error) {
	return ops.EscapeResult{OK: true}, f.record("send_escape")
}

func (f *fakeClient) PaneStart(context.Context, domain.Connection, string, string, ops.StartRequest) (ops.StartResult, error) {
	res := ops.StartResult{OK: true}
	if f.startRes != nil {
		res = *f.startRes
	}
	return res, f.record("pane_start")
}

func (f *fakeClient) SkillList(context.Context, domain.Connection, string) (ops.SkillList, error) {
	return ops.SkillList{Skills: []ops.Skill{{Name: "impl", Invoke: "/impl"}}}, f.record("skill_list")
}

func (f *fakeClient) PaneClose(context.Context, domain.Connection, string, string) (ops.CloseResult, error) {
	return ops.CloseResult{OK: true}, f.record("pane_close")
}

func (f *fakeClient) DirList(context.Context, domain.Connection, string) (ops.DirList, error) {
	return ops.DirList{}, f.record("dir_list")
}

type fixture struct {
	env *servicetest.Env
	cli *fakeClient
	svc *paneservice.PaneService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	env := servicetest.NewEnv(t)
	env.AddConn("view", "view", "http://100.64.0.5:48120/report", true)
	env.AddConn("ops", "ops", "http://100.64.0.5:48120/ops", true)
	cli := &fakeClient{}
	return &fixture{env: env, cli: cli, svc: paneservice.New(cli, env.Conns, slog.New(slog.DiscardHandler))}
}

func TestViewConnection_ReadsAreAllowedAndOperationsAreRefusedWithoutCommunicating(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	if _, err := f.svc.Tail(ctx, "view", "0", "T1", 30); err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if _, err := f.svc.Card(ctx, "view", "0", "T1"); err != nil {
		t.Fatalf("Card: %v", err)
	}
	refused := map[string]error{}
	_, refused["pick"] = f.svc.Pick(ctx, "view", "0", "T1", ops.Pick{N: 1})
	_, refused["send"] = f.svc.Send(ctx, "view", "0", "T1", "x")
	_, refused["escape"] = f.svc.Escape(ctx, "view", "0", "T1", true)
	_, refused["start"] = f.svc.Start(ctx, "view", "0", "T1", ops.StartRequest{Agent: "omp", Cwd: "/w", Title: "t"}, ops.FirstTask{Task: "x"})
	_, refused["close"] = f.svc.Close(ctx, "view", "0", "T1", true)
	_, refused["dirs"] = f.svc.Dirs(ctx, "view", "")
	_, refused["skills"] = f.svc.Skills(ctx, "view", "omp")
	_, refused["resend"] = f.svc.Resend(ctx, "view", "0", "T1", ops.FirstTask{Task: "x"})
	for name, err := range refused {
		if !errors.Is(err, paneservice.ErrOpsRequired) {
			t.Errorf("%s: err = %v; want ErrOpsRequired", name, err)
		}
	}
	if got := f.cli.Calls(); len(got) != 2 || got[0] != "pane_tail" || got[1] != "ask_card" {
		t.Errorf("client の呼び出し = %v; want 読み取りの 2 回だけ", got)
	}
}

func TestOpsConnection_OperationsReachTheClient(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	_, _ = f.svc.Pick(ctx, "ops", "0", "T1", ops.Pick{N: 1})
	_, _ = f.svc.Send(ctx, "ops", "0", "T1", "x")
	_, _ = f.svc.Escape(ctx, "ops", "0", "T1", true)
	_, _ = f.svc.Start(ctx, "ops", "0", "T1", ops.StartRequest{Agent: "omp", Cwd: "/w", Title: "t"}, ops.FirstTask{})
	_, _ = f.svc.Close(ctx, "ops", "0", "T1", true)
	_, _ = f.svc.Dirs(ctx, "ops", "")
	_, _ = f.svc.Skills(ctx, "ops", "omp")
	want := []string{"ask_pick", "send_text", "send_escape", "pane_start", "pane_close", "dir_list", "skill_list"}
	if got := f.cli.Calls(); len(got) != len(want) {
		t.Fatalf("calls = %v; want %v", got, want)
	}
}

func TestEscapeWithoutConfirmationNeverReachesTheClient(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Escape(t.Context(), "ops", "0", "T1", false)
	var fe *domain.FetchError
	if !errors.As(err, &fe) || fe.Kind != domain.ErrOperation || fe.Code != "confirm_required" {
		t.Fatalf("err = %v", err)
	}
	if got := f.cli.Calls(); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}

func TestCloseWithoutConfirmationNeverReachesTheClient(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Close(t.Context(), "ops", "0", "T1", false)
	var fe *domain.FetchError
	if !errors.As(err, &fe) || fe.Kind != domain.ErrOperation || fe.Code != "confirm_required" {
		t.Fatalf("err = %v", err)
	}
	if got := f.cli.Calls(); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}

func TestSamePaneOperationsAreExclusive_OtherPanesAndReadsAreNot(t *testing.T) {
	f := newFixture(t)
	f.cli.gate, f.cli.entered = make(chan struct{}), make(chan struct{}, 4)
	ctx := t.Context()

	done := make(chan error, 1)
	go func() { _, err := f.svc.Send(ctx, "ops", "0", "T1", "one"); done <- err }()
	servicetest.Recv(t, f.cli.entered, "1 つ目の send_text")

	// 同じペイン: どの操作も通信せずに ErrBusy。
	if _, err := f.svc.Send(ctx, "ops", "0", "T1", "two"); !errors.Is(err, paneservice.ErrBusy) {
		t.Errorf("同じペインの 2 つ目: err = %v; want ErrBusy", err)
	}
	if _, err := f.svc.Close(ctx, "ops", "0", "T1", true); !errors.Is(err, paneservice.ErrBusy) {
		t.Errorf("同じペインの別操作: err = %v; want ErrBusy", err)
	}
	// 別のペイン・別セッション・読み取りは止めない。
	if _, err := f.svc.Escape(ctx, "ops", "0", "T2", true); err != nil {
		t.Errorf("別ペイン: %v", err)
	}
	if _, err := f.svc.Escape(ctx, "ops", "1", "T1", true); err != nil {
		t.Errorf("別セッション: %v", err)
	}
	if _, err := f.svc.Tail(ctx, "ops", "0", "T1", 30); err != nil {
		t.Errorf("読み取り: %v", err)
	}
	for _, c := range f.cli.Calls() {
		if c == "pane_close" {
			t.Error("排他で断った pane_close が通信した")
		}
	}

	close(f.cli.gate)
	if err := servicetest.Recv(t, done, "1 つ目の完了"); err != nil {
		t.Fatalf("1 つ目: %v", err)
	}
	// 終わったあとは取り直せる（枠は解放される）。失敗しても解放される。
	if _, err := f.svc.Send(ctx, "ops", "0", "T1", "three"); err != nil {
		t.Errorf("完了後: %v", err)
	}
	f.cli.err = &domain.FetchError{Kind: domain.ErrOperation, Code: "invalid_state"}
	if _, err := f.svc.Send(ctx, "ops", "0", "T1", "four"); err == nil {
		t.Fatal("失敗が返らない")
	}
	f.cli.err = nil
	if _, err := f.svc.Send(ctx, "ops", "0", "T1", "five"); err != nil {
		t.Errorf("失敗後に枠が残っている: %v", err)
	}
}

func TestUnauthorizedMarksNeedsRepair_AndStopsFurtherCalls(t *testing.T) {
	for _, kind := range []domain.ErrKind{domain.ErrUnauthorized, domain.ErrForbiddenScope} {
		f := newFixture(t)
		f.cli.err = &domain.FetchError{Kind: kind, Code: "x"}
		if _, err := f.svc.Send(t.Context(), "ops", "0", "T1", "x"); err == nil {
			t.Fatal("失敗が返らない")
		}
		if !f.env.Conns.NeedsRepair("ops") {
			t.Fatalf("%v: needs_repair が立たない", kind)
		}
		before := len(f.cli.Calls())
		_, err := f.svc.Tail(t.Context(), "ops", "0", "T1", 30)
		var fe *domain.FetchError
		if !errors.As(err, &fe) || fe.Kind != domain.ErrUnauthorized || fe.Code != "needs_repair" {
			t.Errorf("%v: err = %v; want unauthorized/needs_repair", kind, err)
		}
		if len(f.cli.Calls()) != before {
			t.Errorf("%v: needs_repair のあとに通信した", kind)
		}
	}
}

func TestOperationErrorDoesNotMarkNeedsRepair(t *testing.T) {
	f := newFixture(t)
	f.cli.err = &domain.FetchError{Kind: domain.ErrOperation, Code: "invalid_state"}
	_, _ = f.svc.Send(t.Context(), "ops", "0", "T1", "x")
	if f.env.Conns.NeedsRepair("ops") {
		t.Error("操作の失敗で needs_repair が立った")
	}
}

func TestUnknownConnection_IsConnectionNotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Tail(t.Context(), "nope", "0", "T1", 30); !errors.Is(err, service.ErrConnectionNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestUnexpectedClientErrorsAreClassifiedWithoutLeakingText(t *testing.T) {
	f := newFixture(t)
	f.cli.err = errors.New("token=test-token-not-secret leaked")
	_, err := f.svc.Send(t.Context(), "ops", "0", "T1", "x")
	var fe *domain.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(fe.Error(), "test-token-not-secret") {
		t.Errorf("Error() = %q", fe.Error())
	}
}
