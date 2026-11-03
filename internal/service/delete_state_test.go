package service_test

import (
	"context"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

// 削除した接続の情報は、取得に失敗したときの「直前値」としても、タブ・needs_repair としても残らない。
func TestConnectionDeleteLeavesNoStaleStateBehind(t *testing.T) {
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
	if got := e.Snap.Get(ctx, "c1", "0"); got.Err == nil || len(got.Snapshot.Report.Panes) != 0 {
		t.Fatalf("deleted connection served %+v; want an error and no stale data", got)
	}

	// 別の接続は、取得に失敗したとき直前値を返し続ける。
	e.Report.SetReportErr("c2", "0", st.ErrInjected)
	if got := e.Snap.Get(ctx, "c2", "0"); got.Err == nil || !got.Snapshot.Stale || len(got.Snapshot.Report.Panes) != 1 {
		t.Fatalf("other connection lost its last value: %+v", got)
	}
}
