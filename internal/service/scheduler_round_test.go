package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

func TestRunMarksBaselinedOnlyAfterFullySuccessfulRound(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, false)
	e.Report.SetSessions("c1", "0", "1")
	e.Report.SetReport("c1", "0", st.ReportOf(st.Pane("%1", "T1", domain.StateAsking)))
	e.Report.SetReport("c1", "1", st.ReportOf(st.Pane("%2", "T2", domain.StateStalled)))
	e.Report.SetReportErr("c1", "1", st.ErrInjected) // 1 つ失敗する一巡

	rounds := startRun(t, e)
	st.Recv(t, rounds, "round 1 (partial failure)")

	if countOf(e.Rec.List(), "store.MarkBaselined") != 0 {
		t.Fatal("MarkBaselined called after a partially failed round")
	}
	if c, _ := e.Store.Conn("c1"); !c.BaselinedAt.IsZero() {
		t.Fatal("baselined_at set after a partially failed round")
	}
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times while baselining", n)
	}
	// 失敗したセッションの記録は変更しない。成功したセッションは記録済み。
	if got := e.Store.NotifiedSet("c1", "0"); !got[asking("%1")] {
		t.Fatalf("session 0 records = %v", got)
	}
	if got := e.Store.NotifiedSet("c1", "1"); len(got) != 0 {
		t.Fatalf("failed session records changed: %v", got)
	}

	// 次の周期で全許可セッションが成功 → まだ baselining 中なので通知せず、一巡の最後に MarkBaselined。
	e.Report.SetReportErr("c1", "1", nil)
	e.Clock.Ticker(0).Tick()
	st.Recv(t, rounds, "round 2 (success)")
	if countOf(e.Rec.List(), "store.MarkBaselined") != 1 {
		t.Fatalf("MarkBaselined calls = %d, want 1", countOf(e.Rec.List(), "store.MarkBaselined"))
	}
	if c, _ := e.Store.Conn("c1"); !c.BaselinedAt.Equal(e.Clock.Now()) {
		t.Fatalf("baselined_at = %v", c.BaselinedAt)
	}
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times during the baselining round", n)
	}

	// 以降は通常どおり通知する。
	e.Report.SetReport("c1", "1", st.ReportOf(st.Pane("%2", "T2", domain.StateStalled), st.Pane("%3", "T3", domain.StateAsking)))
	e.Clock.Ticker(0).Tick()
	st.Recv(t, rounds, "round 3")
	sent := e.Notifier.Sent()
	if len(sent) != 1 || sent[0].Pos != "T3" || sent[0].Session != "1" {
		t.Fatalf("notifications = %+v", sent)
	}
	if countOf(e.Rec.List(), "store.MarkBaselined") != 1 {
		t.Fatal("MarkBaselined must not be called again once baselined")
	}
}

func TestRunNoSessionsStillBaselines(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, false)
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	if countOf(e.Rec.List(), "store.MarkBaselined") != 1 {
		t.Fatal("a fully successful (empty) round should baseline")
	}
}

func TestRunSessionsFailureSkipsConnectionWithoutBaselining(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "a", st.URL1, false)
	e.AddConn("c2", "b", st.URL2, false)
	e.Report.SessionsErr["c1"] = st.ErrInjected
	e.Report.SetSessions("c2", "0")

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	calls := e.Rec.List()
	if indexOf(calls, "store.PruneSnapshots") < 0 || countOf(calls, "store.MarkBaselined") != 1 {
		t.Fatalf("calls = %v", calls)
	}
	if c, _ := e.Store.Conn("c1"); !c.BaselinedAt.IsZero() {
		t.Fatal("failed connection must not be baselined")
	}
	if c, _ := e.Store.Conn("c2"); c.BaselinedAt.IsZero() {
		t.Fatal("next connection should still be processed and baselined")
	}
	if e.Conns.NeedsRepair("c1") {
		t.Fatal("unreachable must not set needs_repair")
	}
}

func TestRunObserveFailureBlocksBaselining(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, false)
	e.Report.SetSessions("c1", "0")
	e.Store.SetErr("NotifiedKeys", st.ErrInjected)
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	if countOf(e.Rec.List(), "store.MarkBaselined") != 0 {
		t.Fatal("must not baseline when Observe failed")
	}
}

func TestRunBaselinedConnectionIsNotMarkedAgain(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	if countOf(e.Rec.List(), "store.MarkBaselined") != 0 {
		t.Fatal("already baselined")
	}
}

func TestRunAuthFailureSetsNeedsRepairAndMovesOn(t *testing.T) {
	cases := map[string]struct {
		setup func(e *st.Env)
	}{
		"report 401": {func(e *st.Env) {
			e.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: domain.ErrUnauthorized})
		}},
		"report 403": {func(e *st.Env) {
			e.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: domain.ErrForbiddenScope})
		}},
		"sessions 401": {func(e *st.Env) {
			e.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnauthorized}
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := st.NewEnv(t)
			e.AddConn("c1", "a", st.URL1, false)
			e.AddConn("c2", "b", st.URL2, true)
			e.Report.SetSessions("c1", "0", "1")
			e.Report.SetSessions("c2", "0")
			tc.setup(e)

			rounds := startRun(t, e)
			st.Recv(t, rounds, "first round")

			if !e.Conns.NeedsRepair("c1") || e.Conns.NeedsRepair("c2") {
				t.Fatalf("needs_repair c1=%v c2=%v", e.Conns.NeedsRepair("c1"), e.Conns.NeedsRepair("c2"))
			}
			calls := e.Rec.List()
			if countOf(calls, "report.Report:c1/1") != 0 {
				t.Fatalf("remaining sessions of a needs_repair connection were fetched: %v", calls)
			}
			if countOf(calls, "report.Report:c2/0") != 1 {
				t.Fatalf("next connection was not processed: %v", calls)
			}
			if countOf(calls, "store.MarkBaselined") != 0 {
				t.Fatal("must not baseline a failed connection")
			}
			// 次の周期でもスキップされる。
			before := len(calls)
			e.Clock.Ticker(0).Tick()
			st.Recv(t, rounds, "second round")
			for _, c := range e.Rec.List()[before:] {
				if strings.Contains(c, "c1") {
					t.Fatalf("needs_repair connection fetched on the next round: %s", c)
				}
			}
		})
	}
}

func TestRunFailedSessionDoesNotChangeRecordsAndOthersContinue(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0", "1")
	e.Store.Notified[st.SnapKey{Conn: "c1", Session: "0"}] = map[domain.NotifiedKey]struct{}{asking("%1"): {}}
	e.Report.SetReportErr("c1", "0", st.ErrInjected) // 0 は失敗（記録は残る）
	e.Report.SetReport("c1", "1", st.ReportOf(st.Pane("%9", "T1", domain.StateStalled)))

	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")

	if got := e.Store.NotifiedSet("c1", "0"); !got[asking("%1")] || len(got) != 1 {
		t.Fatalf("records of the failed session changed: %v", got)
	}
	if n := len(e.Notifier.Sent()); n != 1 {
		t.Fatalf("notifications = %d, want 1 (session 1)", n)
	}
	if e.Conns.NeedsRepair("c1") {
		t.Fatal("transient failure must not set needs_repair")
	}
}

func TestRunConnectionsLoadFailureIsSurvivable(t *testing.T) {
	e := st.NewEnv(t)
	e.Store.SetErr("Connections", st.ErrInjected)
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	e.Store.SetErr("Connections", nil)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetSessions("c1", "0")
	e.Clock.Ticker(0).Tick()
	st.Recv(t, rounds, "second round")
	if e.Report.ReportCount() != 1 {
		t.Fatalf("reports = %d", e.Report.ReportCount())
	}
}

func TestRunSettingsLoadFailureFallsBackToDefaults(t *testing.T) {
	e := st.NewEnv(t)
	e.Store.SetErr("Settings", st.ErrInjected)
	rounds := startRun(t, e)
	st.Recv(t, rounds, "first round")
	if got := e.Clock.Ticker(0).Interval(); got != 10*time.Minute {
		t.Fatalf("interval = %v", got)
	}
}

func asking(id string) domain.NotifiedKey {
	return domain.NotifiedKey{PaneID: id, State: domain.StateAsking}
}
