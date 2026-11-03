package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

func TestSnapshotGetFailureReturnsPreviousStaleSnapshot(t *testing.T) {
	cases := map[string]struct {
		err      error
		wantKind domain.ErrKind
		wantCode string
	}{
		"unreachable":     {st.ErrInjected, domain.ErrUnreachable, ""},
		"session missing": {&domain.FetchError{Kind: domain.ErrSessionMissing, Code: "invalid_args"}, domain.ErrSessionMissing, "invalid_args"},
		"host error":      {&domain.FetchError{Kind: domain.ErrHostError, Code: "tmux_error"}, domain.ErrHostError, "tmux_error"},
		"protocol":        {domain.ErrInvalidResponse, domain.ErrProtocol, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := st.NewEnv(t)
			e.AddConn("c1", "devbox", st.URL1, true)
			ctx := context.Background()
			e.Report.SetReport("c1", "0", st.ReportOf(st.Pane("%1", "T1", domain.StateWorking)))
			ok := e.Snap.Get(ctx, "c1", "0")
			e.Clock.Advance(time.Minute)
			e.Report.SetReportErr("c1", "0", tc.err)

			res := e.Snap.Get(ctx, "c1", "0")

			if res.Fresh || res.Err == nil || res.Err.Kind != tc.wantKind || res.Err.Code != tc.wantCode {
				t.Fatalf("res = %+v err=%v", res, res.Err)
			}
			if !res.Snapshot.Stale || !res.Snapshot.FetchedAt.Equal(ok.Snapshot.FetchedAt) || len(res.Snapshot.Report.Panes) != 1 {
				t.Fatalf("previous snapshot not returned stale: %+v", res.Snapshot)
			}
			if saved, _ := e.Store.Snap("c1", "0"); saved.Stale {
				t.Fatal("stale flag must not be persisted")
			}
			if e.Conns.NeedsRepair("c1") {
				t.Fatal("non-auth failure must not set needs_repair")
			}
		})
	}
}

func TestSnapshotGetFailureFallsBackToStoredSnapshot(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	at := e.Clock.Now().Add(-time.Hour)
	e.Store.Snaps[st.SnapKey{Conn: "c1", Session: "0"}] = domain.Snapshot{Report: st.ReportOf(st.Pane("%1", "T1", domain.StateIdle)), FetchedAt: at}
	e.Report.SetReportErr("c1", "0", st.ErrInjected)

	res := e.Snap.Get(context.Background(), "c1", "0")

	if res.Fresh || res.Err == nil || !res.Snapshot.Stale || !res.Snapshot.FetchedAt.Equal(at) {
		t.Fatalf("res = %+v", res)
	}
}

func TestSnapshotGetFailureWithoutPreviousIsZero(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetReportErr("c1", "0", st.ErrInjected)

	res := e.Snap.Get(context.Background(), "c1", "0")

	if res.Fresh || res.Err == nil || !res.Snapshot.FetchedAt.IsZero() || res.Snapshot.Stale {
		t.Fatalf("res = %+v", res)
	}
}

func TestSnapshotGetFailureDoesNotTouchNotifiedRecords(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Store.Notified[st.SnapKey{Conn: "c1", Session: "0"}] = map[domain.NotifiedKey]struct{}{{PaneID: "%1", State: domain.StateAsking}: {}}
	e.Report.SetReportErr("c1", "0", st.ErrInjected)

	e.Snap.Get(context.Background(), "c1", "0")

	if got := e.Store.NotifiedSet("c1", "0"); len(got) != 1 {
		t.Fatalf("records changed on failure: %v", got)
	}
	for _, c := range e.Rec.List() {
		if c == "store.ApplyNotified" {
			t.Fatal("ApplyNotified called on failure")
		}
	}
}

func TestSnapshotGetAuthErrorsSetNeedsRepairAndStopFetching(t *testing.T) {
	for name, kind := range map[string]domain.ErrKind{"401": domain.ErrUnauthorized, "403": domain.ErrForbiddenScope} {
		t.Run(name, func(t *testing.T) {
			e := st.NewEnv(t)
			e.AddConn("c1", "devbox", st.URL1, true)
			ctx := context.Background()
			e.Report.SetReport("c1", "0", st.ReportOf())
			e.Snap.Get(ctx, "c1", "0") // 直前値を作る
			e.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: kind, Code: "x"})

			res := e.Snap.Get(ctx, "c1", "0")

			if res.Err == nil || res.Err.Kind != kind || res.Fresh {
				t.Fatalf("res = %+v", res)
			}
			if !e.Conns.NeedsRepair("c1") {
				t.Fatal("needs_repair not set")
			}
			if !res.Snapshot.Stale {
				t.Fatal("previous snapshot should still be returned")
			}
			// 以降は取得しない。
			before := e.Report.ReportCount()
			again := e.Snap.Get(ctx, "c1", "0")
			if e.Report.ReportCount() != before {
				t.Fatal("fetched while needs_repair")
			}
			if again.Err == nil || again.Err.Kind != domain.ErrUnauthorized || again.Fresh || !again.Snapshot.Stale {
				t.Fatalf("again = %+v", again)
			}
			// 再ペアリング後は取得を再開する。
			e.Conns.MarkRepaired("c1")
			e.Report.SetReportErr("c1", "0", nil)
			if res := e.Snap.Get(ctx, "c1", "0"); !res.Fresh {
				t.Fatalf("after repair: %+v", res)
			}
		})
	}
}

func TestSnapshotGetUnknownConnection(t *testing.T) {
	e := st.NewEnv(t)
	res := e.Snap.Get(context.Background(), "missing", "0")
	if res.Fresh || res.Err == nil {
		t.Fatalf("res = %+v", res)
	}
	if e.Report.ReportCount() != 0 {
		t.Fatal("report called for an unknown connection")
	}
}

func TestSnapshotGetSaveFailureStillReturnsFresh(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Store.SetErr("SaveSnapshot", st.ErrInjected)
	if res := e.Snap.Get(context.Background(), "c1", "0"); !res.Fresh {
		t.Fatalf("res = %+v", res)
	}
}

func TestSnapshotGetObserveFailureStillReturnsFresh(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Store.SetErr("NotifiedKeys", st.ErrInjected)
	if res := e.Snap.Get(context.Background(), "c1", "0"); !res.Fresh || res.Err != nil {
		t.Fatalf("res = %+v", res)
	}
	if !strings.Contains(e.Logs.String(), "observe failed") {
		t.Fatal("observe failure should be logged")
	}
}

func TestSnapshotLogsDoNotContainBodies(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	p := st.Pane("%1", "T1", domain.StateAsking)
	p.Activity = "SECRET-ACTIVITY-BODY"
	e.Report.SetReport("c1", "0", st.ReportOf(p))
	e.Notifier.Err = errors.New("os notification failed")
	e.Snap.Get(context.Background(), "c1", "0")
	if strings.Contains(e.Logs.String(), "SECRET-ACTIVITY-BODY") {
		t.Fatal("log contains pane body")
	}
}
