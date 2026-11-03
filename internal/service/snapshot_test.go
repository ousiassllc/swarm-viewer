package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

func TestSnapshotGetSuccessSavesAndObserves(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	r := st.ReportOf(st.Pane("%1", "T1", domain.StateAsking))
	r.Attention = []domain.Attention{{Kind: "asking", Pane: "T1", Detail: "proceed?"}}
	e.Report.SetReport("c1", "0", r)

	res := e.Snap.Get(context.Background(), "c1", "0")

	if !res.Fresh || res.Err != nil || res.Snapshot.Stale {
		t.Fatalf("res = %+v", res)
	}
	if !res.Snapshot.FetchedAt.Equal(e.Clock.Now()) || len(res.Snapshot.Report.Panes) != 1 {
		t.Fatalf("snapshot = %+v", res.Snapshot)
	}
	if saved, ok := e.Store.Snap("c1", "0"); !ok || saved.FetchedAt != res.Snapshot.FetchedAt {
		t.Fatalf("saved = %+v ok=%v", saved, ok)
	}
	// Observe が呼ばれ、通知が出る。
	sent := e.Notifier.Sent()
	if len(sent) != 1 || sent[0].Pos != "T1" || sent[0].Detail != "proceed?" || sent[0].ConnectionName != "devbox" {
		t.Fatalf("notifications = %+v", sent)
	}
	// 順序: Report → SaveSnapshot → Observe（NotifiedKeys）。
	var order []string
	for _, c := range e.Rec.List() {
		switch c {
		case "report.Report:c1/0", "store.SaveSnapshot", "store.NotifiedKeys":
			order = append(order, c)
		}
	}
	if strings.Join(order, ",") != "report.Report:c1/0,store.SaveSnapshot,store.NotifiedKeys" {
		t.Fatalf("order = %v", order)
	}
}

func TestSnapshotGetObservesEvenWhenNotificationsOff(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Store.Cfg.NotificationsEnabled = false
	e.Report.SetReport("c1", "0", st.ReportOf(st.Pane("%1", "T1", domain.StateAsking)))

	res := e.Snap.Get(context.Background(), "c1", "0")

	if !res.Fresh {
		t.Fatalf("res = %+v", res)
	}
	if got := e.Store.NotifiedSet("c1", "0"); !got[domain.NotifiedKey{PaneID: "%1", State: domain.StateAsking}] {
		t.Fatalf("Observe must record even when notifications are off: %v", got)
	}
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times while off", n)
	}
	if _, ok := e.Store.Snap("c1", "0"); !ok {
		t.Fatal("snapshot not saved while notifications are off")
	}
}

func TestSnapshotGetSingleFlightCollapsesConcurrentCalls(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetReport("c1", "0", st.ReportOf(st.Pane("%1", "T1", domain.StateIdle)))
	e.Report.Gate = make(chan struct{})
	const callers = 8
	joined := make(chan struct{}, callers)
	service.SetAfterJoin(e.Snap, func() { joined <- struct{}{} })

	results := make(chan service.SnapshotResult, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- e.Snap.Get(context.Background(), "c1", "0")
		}()
	}
	// 全員が合流してから report を解放する。
	for range callers {
		st.Recv(t, joined, "caller to join")
	}
	st.Recv(t, e.Report.Entered, "report call")
	close(e.Report.Gate)
	wg.Wait()
	close(results)

	if n := e.Report.ReportCount(); n != 1 {
		t.Fatalf("report called %d times, want 1", n)
	}
	for res := range results {
		if !res.Fresh || res.Err != nil {
			t.Fatalf("res = %+v", res)
		}
	}
}

func TestSnapshotGetDifferentKeysAreNotCollapsed(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.AddConn("c2", "other", st.URL2, true)
	ctx := context.Background()
	e.Snap.Get(ctx, "c1", "0")
	e.Snap.Get(ctx, "c1", "1")
	e.Snap.Get(ctx, "c2", "0")
	if n := e.Report.ReportCount(); n != 3 {
		t.Fatalf("report called %d times, want 3", n)
	}
}

func TestSnapshotGetCallerCancelDoesNotAbortSharedFetch(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.Report.SetReport("c1", "0", st.ReportOf())
	e.Report.Gate = make(chan struct{})
	joined := make(chan struct{}, 2)
	service.SetAfterJoin(e.Snap, func() { joined <- struct{}{} })

	cancelCtx, cancel := context.WithCancel(context.Background())
	first := make(chan service.SnapshotResult, 1)
	second := make(chan service.SnapshotResult, 1)
	go func() { first <- e.Snap.Get(cancelCtx, "c1", "0") }()
	st.Recv(t, joined, "first caller")
	go func() { second <- e.Snap.Get(context.Background(), "c1", "0") }()
	st.Recv(t, joined, "second caller")
	st.Recv(t, e.Report.Entered, "report call")

	cancel()
	if res := st.Recv(t, first, "canceled caller"); res.Fresh || res.Err == nil {
		t.Fatalf("canceled caller res = %+v", res)
	}
	close(e.Report.Gate)
	if res := st.Recv(t, second, "second caller result"); !res.Fresh {
		t.Fatalf("second caller res = %+v", res)
	}
	if n := e.Report.ReportCount(); n != 1 {
		t.Fatalf("report called %d times", n)
	}
}
