package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func sampleReport(t *testing.T) domain.Report {
	t.Helper()
	// 未知のキー（future_field）を含む応答。ParseReport が捨てる。
	r, err := domain.ParseReport([]byte(`{
  "at": "2026-11-03T09:10:05Z",
  "future_field": {"x": 1},
  "layout": {"rows": 1, "cols": [2]},
  "panes": [
    {"pos": "0.0", "id": "%1", "agent": "claude", "state": "asking", "title": "t", "cwd": "/w",
     "elapsed": "1m", "activity": "a", "truncated": true, "changed_ago": "5s",
     "note": {"from": "x", "repo": "r", "items": ["i1", "i2"], "next": "n"}, "new_pane_key": 1},
    {"pos": "0.1", "id": "%2", "state": "brand-new-state"}
  ],
  "attention": [{"kind": "asking", "pane": "%1", "detail": "d"}],
  "warnings": [{"code": "w", "message": "m"}]
}`))
	if err != nil {
		t.Fatalf("ParseReport: %v", err)
	}
	return r
}

func TestSnapshotRoundTrip(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	if _, ok, err := s.Snapshot(ctx, "c1", "work"); err != nil || ok {
		t.Fatalf("Snapshot before save = ok %v, %v", ok, err)
	}

	report := sampleReport(t)
	if report.Panes[1].State != domain.StateUnknown {
		t.Fatalf("precondition: unknown state must normalise, got %q", report.Panes[1].State)
	}
	fetched := time.Date(2026, 11, 3, 9, 10, 7, 0, time.UTC)
	if err := s.SaveSnapshot(ctx, "c1", "work", domain.Snapshot{Report: report, FetchedAt: fetched, Stale: true}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Snapshot(ctx, "c1", "work")
	if err != nil || !ok {
		t.Fatalf("Snapshot = ok %v, %v", ok, err)
	}
	if !reflect.DeepEqual(got.Report, report) {
		t.Errorf("report mismatch:\n got %+v\nwant %+v", got.Report, report)
	}
	if !got.FetchedAt.Equal(fetched) || got.Stale {
		t.Errorf("FetchedAt = %v, Stale = %v", got.FetchedAt, got.Stale)
	}
	// 他のセッション・他の接続には見えない。
	if _, ok, _ := s.Snapshot(ctx, "c1", "other"); ok {
		t.Error("snapshot leaked across sessions")
	}
	if _, ok, _ := s.Snapshot(ctx, "c2", "work"); ok {
		t.Error("snapshot leaked across connections")
	}
}

func TestSaveSnapshotKeepsLatestOnly(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	for i, title := range []string{"first", "second"} {
		r := domain.Report{Panes: []domain.Pane{{ID: "%1", Title: title}}}
		sn := domain.Snapshot{Report: r, FetchedAt: clock.Now().Add(time.Duration(i) * time.Minute)}
		if err := s.SaveSnapshot(ctx, "c1", "work", sn); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, s, "SELECT count(*) FROM snapshots"); n != 1 {
		t.Fatalf("rows = %d; want exactly one per connection×session", n)
	}
	got, ok, err := s.Snapshot(ctx, "c1", "work")
	if err != nil || !ok || got.Report.Panes[0].Title != "second" {
		t.Errorf("latest = %+v, %v, %v", got, ok, err)
	}
}

func TestSaveSnapshotMissingConnection(t *testing.T) {
	s, _ := newTestStore(t)
	err := s.SaveSnapshot(context.Background(), "ghost", "work", domain.Snapshot{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}

func TestSnapshotTolerance(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	at := formatTime(clock.Now())
	put := func(session string, version int, raw string) {
		t.Helper()
		_, err := s.db.Exec(`INSERT INTO snapshots (connection_id, session_id, fetched_at, schema_version, report_json)
VALUES ('c1', ?, ?, ?, ?)`, session, at, version, raw)
		if err != nil {
			t.Fatalf("insert %s: %v", session, err)
		}
	}

	// 将来のアプリが足した未知のキー・未知の state を含む同じ版の JSON は読める。
	put("future-keys", SnapshotSchemaVersion,
		`{"at":"2026-11-03T09:10:05Z","new_top":{"a":1},"panes":[{"id":"%1","state":"zzz","new_key":true}]}`)
	got, ok, err := s.Snapshot(ctx, "c1", "future-keys")
	if err != nil || !ok || len(got.Report.Panes) != 1 || got.Report.Panes[0].State != domain.StateUnknown {
		t.Errorf("unknown keys: %+v, %v, %v", got, ok, err)
	}

	// 欠落した項目は零値。
	put("minimal", SnapshotSchemaVersion, `{}`)
	if got, ok, err := s.Snapshot(ctx, "c1", "minimal"); err != nil || !ok || len(got.Report.Panes) != 0 {
		t.Errorf("minimal: %+v, %v, %v", got, ok, err)
	}

	// 読めない版・形が違う JSON・壊れた時刻は「スナップショットなし」（エラーにしない）。
	put("newer-version", SnapshotSchemaVersion+1, `{}`)
	put("older-version", 0, `{}`)
	put("wrong-shape", SnapshotSchemaVersion, `[1,2,3]`)
	put("wrong-type", SnapshotSchemaVersion, `{"panes":"nope"}`)
	if _, err := s.db.Exec(`INSERT INTO snapshots VALUES ('c1', 'bad-time', 'yesterday', ?, '{}')`, SnapshotSchemaVersion); err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"newer-version", "older-version", "wrong-shape", "wrong-type", "bad-time"} {
		if got, ok, err := s.Snapshot(ctx, "c1", session); err != nil || ok {
			t.Errorf("%s: got %+v, ok %v, err %v; want no snapshot and no error", session, got, ok, err)
		}
	}
}

func TestSnapshotJSONCheck(t *testing.T) {
	s, clock := newTestStore(t)
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	_, err := s.db.Exec(`INSERT INTO snapshots VALUES ('c1', 's', 'x', 1, 'not json{')`)
	if err == nil {
		t.Fatal("json_valid CHECK must reject invalid JSON")
	}
}

func TestPruneSnapshots(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	mustSave(t, s, newConn("c2", urlB, clock.Now()))
	sessions := []string{"a", "b", "c"}
	for _, id := range []string{"c1", "c2"} {
		for _, sess := range sessions {
			if err := s.SaveSnapshot(ctx, id, sess, domain.Snapshot{FetchedAt: clock.Now()}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := s.PruneSnapshots(ctx, "c1", []string{"a", "c", "unknown"}); err != nil {
		t.Fatal(err)
	}
	for sess, want := range map[string]bool{"a": true, "b": false, "c": true} {
		if _, ok, _ := s.Snapshot(ctx, "c1", sess); ok != want {
			t.Errorf("c1/%s present = %v, want %v", sess, ok, want)
		}
	}
	if n := count(t, s, "SELECT count(*) FROM snapshots WHERE connection_id = 'c2'"); n != 3 {
		t.Errorf("other connection's snapshots = %d, want 3 (untouched)", n)
	}

	// keep が空なら（許可セッションが 0 件になった）接続の全スナップショットを消す。
	if err := s.PruneSnapshots(ctx, "c1", nil); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "SELECT count(*) FROM snapshots WHERE connection_id = 'c1'"); n != 0 {
		t.Errorf("c1 snapshots = %d, want 0", n)
	}
	// 何も無い接続でもエラーにしない。
	if err := s.PruneSnapshots(ctx, "ghost", []string{"a"}); err != nil {
		t.Errorf("prune on unknown connection: %v", err)
	}
}
