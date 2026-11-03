package service_test

import (
	"context"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

func TestConnectionDeleteReleasesInMemoryState(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, true)
	e.AddConn("c2", "other", st.URL2, true)
	ctx := context.Background()
	for _, id := range []string{"c1", "c2"} {
		e.Report.SetSessions(id, "0")
		e.Report.SetReport(id, "0", st.ReportOf(st.Pane("%1", "T1", domain.StateIdle)))
	}
	if _, err := e.Conns.Tabs(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	e.Snap.Get(ctx, "c1", "0")
	e.Snap.Get(ctx, "c2", "0")
	service.TouchObserveLock(e.Notif, "c1")
	e.Conns.MarkNeedsRepair("c1")

	if err := e.Conns.Delete(ctx, "c1"); err != nil {
		t.Fatal(err)
	}

	if e.Conns.NeedsRepair("c1") {
		t.Fatal("needs_repair not released")
	}
	if _, _, ok := e.Conns.CachedTabs("c1"); ok {
		t.Fatal("tab cache not released")
	}
	if service.HasSnapshotCache(e.Snap, "c1", "0") || !service.HasSnapshotCache(e.Snap, "c2", "0") {
		t.Fatal("snapshot cache must be released for the deleted connection only")
	}
	if service.HasObserveLock(e.Notif, "c1") {
		t.Fatal("notification mutex not released")
	}
}
