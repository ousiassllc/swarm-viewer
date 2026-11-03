package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

// startRun は Run をバックグラウンドで起動する。rounds は一巡（スキップを含む）が終わるたびに届く。
func startRun(t *testing.T, e *st.Env) (rounds <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 32)
	service.SetAfterRound(e.Notif, func() { ch <- struct{}{} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.Notif.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(st.WaitDeadline):
			t.Error("Run did not stop after cancel")
		}
	})
	return ch
}

func indexOf(calls []string, want string) int {
	for i, c := range calls {
		if c == want {
			return i
		}
	}
	return -1
}

func countOf(calls []string, want string) int {
	n := 0
	for _, c := range calls {
		if c == want {
			n++
		}
	}
	return n
}

func TestRunFirstRoundImmediatelyWithoutTick(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	e.Report.SetReport("c1", "0", st.ReportOf(st.Pane("%1", "T1", domain.StateAsking)))

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	if e.Report.ReportCount() != 1 {
		t.Fatalf("reports = %d, want 1 before any tick", e.Report.ReportCount())
	}
	if n := len(e.Notifier.Sent()); n != 1 {
		t.Fatalf("notifications = %d", n)
	}
	if _, ok := e.Store.Snap("c1", "0"); !ok {
		t.Fatal("snapshot not saved by the polling round")
	}
	tk := e.Clock.Ticker(0)
	if got := tk.Interval(); got != 10*time.Minute {
		t.Fatalf("interval = %v, want 10m (default)", got)
	}
}

func TestRunUsesConfiguredInterval(t *testing.T) {
	e := st.NewEnv(t)
	e.Store.Cfg.NotifyPollMinutes = 30
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	if got := e.Clock.Ticker(0).Interval(); got != 30*time.Minute {
		t.Fatalf("interval = %v", got)
	}
}

func TestRunTickTriggersRound(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	e.Clock.Ticker(0).Tick()
	st.Recv(t, rounds, "second round")

	if n := e.Report.ReportCount(); n != 2 {
		t.Fatalf("reports = %d, want 2", n)
	}
}

func TestRunDisabledSettingSkipsFetching(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	e.Store.Cfg.NotificationsEnabled = false

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	e.Clock.Ticker(0).Tick()
	st.Recv(t, rounds, "tick round")

	if calls := e.Rec.List(); len(calls) != 1 || calls[0] != "store.Settings" {
		t.Fatalf("calls = %v; a disabled round must not touch connections or the network", calls)
	}
}

func TestRunSettingsChangeResetsTickerAndTogglesPolling(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	tk := e.Clock.Ticker(0)
	ctx := context.Background()

	// 間隔変更 → Reset。
	if _, err := e.Settings.Update(ctx, service.SettingsInput{NotificationsEnabled: true, ScreenPollSeconds: 10, NotifyPollMinutes: 60}); err != nil {
		t.Fatal(err)
	}
	if got := st.Recv(t, tk.Resets, "ticker reset"); got != time.Hour {
		t.Fatalf("reset to %v, want 1h", got)
	}
	// 無効化 → tick しても取得しない。
	if _, err := e.Settings.Update(ctx, service.SettingsInput{NotificationsEnabled: false, ScreenPollSeconds: 10, NotifyPollMinutes: 60}); err != nil {
		t.Fatal(err)
	}
	st.Recv(t, tk.Resets, "ticker reset (disable)")
	before := e.Report.ReportCount()
	tk.Tick()
	st.Recv(t, rounds, "round while disabled")
	if e.Report.ReportCount() != before {
		t.Fatal("fetched while notifications are disabled")
	}
	// 再有効化 → 次の tick から取得する。
	if _, err := e.Settings.Update(ctx, service.SettingsInput{NotificationsEnabled: true, ScreenPollSeconds: 10, NotifyPollMinutes: 60}); err != nil {
		t.Fatal(err)
	}
	st.Recv(t, tk.Resets, "ticker reset (enable)")
	tk.Tick()
	st.Recv(t, rounds, "round after enabling")
	if e.Report.ReportCount() != before+1 {
		t.Fatalf("reports = %d, want %d", e.Report.ReportCount(), before+1)
	}
}

func TestRunStopsOnCancelAndStopsTicker(t *testing.T) {
	e := st.NewEnv(t)
	ch := make(chan struct{}, 8)
	service.SetAfterRound(e.Notif, func() { ch <- struct{}{} })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); e.Notif.Run(ctx) }()
	st.Recv(t, ch, "first round")

	cancel()
	st.Recv(t, done, "Run to return")

	if !e.Clock.Ticker(0).Stopped() {
		t.Fatal("ticker not stopped")
	}
}

func TestRunProcessesConnectionsSequentiallyInOrder(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "a", st.URL1, true)
	e.AddConn("c2", "b", st.URL2, true)
	e.Report.SetSessions("c1", "0", "1")
	e.Report.SetSessions("c2", "0")

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	if m := e.Report.MaxInflight.Load(); m != 1 {
		t.Fatalf("max concurrent report calls = %d, want 1 (NFR-PERF-6)", m)
	}
	var net []string
	for _, c := range e.Rec.List() {
		if strings.HasPrefix(c, "report.") {
			net = append(net, c)
		}
	}
	want := "report.Sessions:c1,report.Report:c1/0,report.Report:c1/1,report.Sessions:c2,report.Report:c2/0"
	if strings.Join(net, ",") != want {
		t.Fatalf("order = %v", net)
	}
}

func TestRunSkipsNeedsRepairConnections(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "a", st.URL1, true)
	e.AddConn("c2", "b", st.URL2, true)
	e.Report.SetSessions("c1", "0")
	e.Report.SetSessions("c2", "0")
	e.Conns.MarkNeedsRepair("c1")

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	calls := e.Rec.List()
	for _, c := range calls {
		if strings.HasSuffix(c, ":c1") || strings.Contains(c, ":c1/") {
			t.Fatalf("needs_repair connection was fetched: %v", calls)
		}
	}
	if countOf(calls, "report.Report:c2/0") != 1 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestRunPrunesSnapshotsAfterSessionsBeforeReports(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "a", st.URL1, true)
	e.Report.Mu.Lock()
	e.Report.SessionList["c1"] = []domain.Session{{ID: "0", Allowed: true}, {ID: "stub", Allowed: false}, {ID: "1", Allowed: true}}
	e.Report.Mu.Unlock()

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	calls := e.Rec.List()
	s, p, r := indexOf(calls, "report.Sessions:c1"), indexOf(calls, "store.PruneSnapshots"), indexOf(calls, "report.Report:c1/0")
	if s < 0 || p < s || r < p {
		t.Fatalf("want Sessions < PruneSnapshots < Report, got %v", calls)
	}
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()
	if len(e.Store.Pruned) != 1 || e.Store.Pruned[0].Conn != "c1" || strings.Join(e.Store.Pruned[0].Keep, ",") != "0,1" {
		t.Fatalf("pruned = %+v, want keep [0 1]", e.Store.Pruned)
	}
}
