package servicetest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

func asking(id string) domain.NotifiedKey {
	return domain.NotifiedKey{PaneID: id, State: domain.StateAsking}
}

func stalled(id string) domain.NotifiedKey {
	return domain.NotifiedKey{PaneID: id, State: domain.StateStalled}
}

func TestObserveNotifiesOnceAndRecords(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	r := ReportOf(Pane("%1", "T1", domain.StateAsking), Pane("%2", "T2", domain.StateWorking))
	r.Attention = []domain.Attention{{Kind: "asking", Pane: "T1", Detail: "continue?"}, {Kind: "asking", Pane: "T9", Detail: "other"}}

	if err := e.Notif.Observe(ctx, conn, "0", r); err != nil {
		t.Fatal(err)
	}
	sent := e.Notifier.Sent()
	want := service.Notification{ConnectionID: "c1", ConnectionName: "devbox", Session: "0", Pos: "T1", State: domain.StateAsking, Detail: "continue?"}
	if len(sent) != 1 || sent[0] != want {
		t.Fatalf("sent = %+v, want [%+v]", sent, want)
	}
	if got := e.Store.NotifiedSet("c1", "0"); len(got) != 1 || !got[asking("%1")] {
		t.Fatalf("records = %v", got)
	}

	// 同じ状態の再観測では再通知しない。
	if err := e.Notif.Observe(ctx, conn, "0", r); err != nil {
		t.Fatal(err)
	}
	if n := len(e.Notifier.Sent()); n != 1 {
		t.Fatalf("re-notified: %d", n)
	}
}

func TestObserveStalledHasNoDetail(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	r := ReportOf(Pane("%1", "T1", domain.StateStalled))
	r.Attention = []domain.Attention{{Kind: "asking", Pane: "T1", Detail: "should not be used"}}
	if err := e.Notif.Observe(context.Background(), conn, "0", r); err != nil {
		t.Fatal(err)
	}
	sent := e.Notifier.Sent()
	if len(sent) != 1 || sent[0].State != domain.StateStalled || sent[0].Detail != "" {
		t.Fatalf("sent = %+v", sent)
	}
}

func TestObserveTransitions(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	observe := func(s domain.PaneState) {
		t.Helper()
		if err := e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", s))); err != nil {
			t.Fatal(err)
		}
	}
	steps := []struct {
		state   domain.PaneState
		total   int // この時点までの累計通知数
		records map[domain.NotifiedKey]bool
	}{
		{domain.StateAsking, 1, map[domain.NotifiedKey]bool{asking("%1"): true}},
		{domain.StateAsking, 1, map[domain.NotifiedKey]bool{asking("%1"): true}},
		{domain.StateStalled, 2, map[domain.NotifiedKey]bool{stalled("%1"): true}}, // asking → stalled は通知
		{domain.StateIdle, 2, map[domain.NotifiedKey]bool{}},                       // 離脱で記録を消す
		{domain.StateStalled, 3, map[domain.NotifiedKey]bool{stalled("%1"): true}}, // 再突入は再通知
	}
	for i, st := range steps {
		observe(st.state)
		if n := len(e.Notifier.Sent()); n != st.total {
			t.Fatalf("step %d: notifications = %d, want %d", i, n, st.total)
		}
		got := e.Store.NotifiedSet("c1", "0")
		if len(got) != len(st.records) {
			t.Fatalf("step %d: records = %v, want %v", i, got, st.records)
		}
		for k := range st.records {
			if !got[k] {
				t.Fatalf("step %d: records = %v, want %v", i, got, st.records)
			}
		}
	}
}

func TestObservePaneDisappearingClearsRecord(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf())
	if got := e.Store.NotifiedSet("c1", "0"); len(got) != 0 {
		t.Fatalf("records = %v", got)
	}
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))
	if n := len(e.Notifier.Sent()); n != 2 {
		t.Fatalf("notifications = %d, want 2", n)
	}
}

func TestObserveSessionsAreIndependent(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	r := ReportOf(Pane("%1", "T1", domain.StateAsking))
	_ = e.Notif.Observe(ctx, conn, "0", r)
	_ = e.Notif.Observe(ctx, conn, "1", r)
	if n := len(e.Notifier.Sent()); n != 2 {
		t.Fatalf("notifications = %d, want 2", n)
	}
}

func TestObserveDoubleNotificationPreventedByInsertedSet(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	// 別経路が Detect と ApplyNotified の間に同じキーを挿入した状況。
	e.Store.IgnoreInsert[asking("%1")] = true

	err := e.Notif.Observe(context.Background(), conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking), Pane("%2", "T2", domain.StateAsking)))
	if err != nil {
		t.Fatal(err)
	}

	sent := e.Notifier.Sent()
	if len(sent) != 1 || sent[0].Pos != "T2" {
		t.Fatalf("only the newly inserted key may notify, got %+v", sent)
	}
}

func TestObserveBaseliningRecordsWithoutNotifying(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, false)
	ctx := context.Background()

	err := e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking), Pane("%2", "T2", domain.StateStalled)))
	if err != nil {
		t.Fatal(err)
	}

	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times while baselining", n)
	}
	if got := e.Store.NotifiedSet("c1", "0"); !got[asking("%1")] || !got[stalled("%2")] {
		t.Fatalf("records = %v", got)
	}
	// baselined 後に同じ状態を観測しても通知されない（記録済み）。
	conn.BaselinedAt = e.Clock.Now()
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times after baselining for recorded state", n)
	}
}

func TestObserveNotificationsOffRecordsWithoutNotifying(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	e.Store.Cfg.NotificationsEnabled = false

	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))

	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times while off", n)
	}
	if got := e.Store.NotifiedSet("c1", "0"); !got[asking("%1")] {
		t.Fatalf("records = %v", got)
	}
	// オンに戻しても、オフの間に入った状態は一斉通知されない。
	e.Store.Mu.Lock()
	e.Store.Cfg.NotificationsEnabled = true
	e.Store.Mu.Unlock()
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times after turning on", n)
	}
	// 新しい遷移は通知される。
	_ = e.Notif.Observe(ctx, conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking), Pane("%2", "T2", domain.StateStalled)))
	if n := len(e.Notifier.Sent()); n != 1 {
		t.Fatalf("notifications = %d, want 1", n)
	}
}

func TestObserveNotifyFailureDoesNotRollBack(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	e.Notifier.Err = errors.New("os notification failed")
	r := ReportOf(Pane("%1", "T1", domain.StateAsking), Pane("%2", "T2", domain.StateStalled))

	if err := e.Notif.Observe(ctx, conn, "0", r); err != nil {
		t.Fatalf("notify failure must not fail Observe: %v", err)
	}

	if n := len(e.Notifier.Sent()); n != 2 {
		t.Fatalf("every event is attempted, got %d", n)
	}
	if got := e.Store.NotifiedSet("c1", "0"); len(got) != 2 {
		t.Fatalf("records must stay: %v", got)
	}
	if !strings.Contains(e.Logs.String(), "notify failed") {
		t.Fatal("failure should be logged as a warning")
	}
	// 再通知しない。
	_ = e.Notif.Observe(ctx, conn, "0", r)
	if n := len(e.Notifier.Sent()); n != 2 {
		t.Fatalf("re-notified after failure: %d", n)
	}
}

func TestObserveStoreFailures(t *testing.T) {
	for _, method := range []string{"Settings", "NotifiedKeys", "ApplyNotified"} {
		t.Run(method, func(t *testing.T) {
			e := NewEnv(t)
			conn := e.AddConn("c1", "devbox", URL1, true)
			e.Store.SetErr(method, ErrInjected)
			err := e.Notif.Observe(context.Background(), conn, "0", ReportOf(Pane("%1", "T1", domain.StateAsking)))
			if !errors.Is(err, ErrInjected) {
				t.Fatalf("err = %v", err)
			}
			if n := len(e.Notifier.Sent()); n != 0 {
				t.Fatalf("notified %d times despite failure", n)
			}
		})
	}
}

func TestObserveNoChangeSkipsApply(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	_ = e.Notif.Observe(context.Background(), conn, "0", ReportOf(Pane("%1", "T1", domain.StateWorking)))
	for _, c := range e.Rec.List() {
		if c == "store.ApplyNotified" {
			t.Fatal("ApplyNotified called without changes")
		}
	}
}

func TestObserveConcurrentCallersNotifyOnce(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	r := ReportOf(Pane("%1", "T1", domain.StateAsking))
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.Notif.Observe(context.Background(), conn, "0", r); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(e.Notifier.Sent()); n != 1 {
		t.Fatalf("notifications = %d, want 1", n)
	}
}

func TestObserveLogsDoNotContainBodiesOrURL(t *testing.T) {
	e := NewEnv(t)
	conn := e.AddConn("c1", "devbox", URL1, true)
	e.Notifier.Err = errors.New("os notification failed")
	p := Pane("%1", "T1", domain.StateAsking)
	p.Activity, p.Title = "SECRET-ACTIVITY-BODY", "SECRET-TITLE"
	p.Note = &domain.Note{Repo: "SECRET-REPO"}
	_ = e.Notif.Observe(context.Background(), conn, "0", ReportOf(p))
	logs := e.Logs.String()
	for _, s := range []string{"SECRET-ACTIVITY-BODY", "SECRET-TITLE", "SECRET-REPO", URL1} {
		if strings.Contains(logs, s) {
			t.Fatalf("log contains %q", s)
		}
	}
}
