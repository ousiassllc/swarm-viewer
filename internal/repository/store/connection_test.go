package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

const (
	urlA = "http://100.64.0.1:48120/report"
	urlB = "http://100.64.0.2:48120/report"
)

func TestConnectionsEmptyAndOrder(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	if got, err := s.Connections(ctx); err != nil || len(got) != 0 {
		t.Fatalf("empty Connections = %v, %v", got, err)
	}
	t0 := clock.Now()
	// 作成時刻の降順で保存しても、昇順（古い順）で返る。
	mustSave(t, s, newConn("c-late", urlB, t0.Add(time.Hour)))
	mustSave(t, s, newConn("c-early", urlA, t0))
	got, err := s.Connections(ctx)
	if err != nil || len(got) != 2 || got[0].ID != "c-early" || got[1].ID != "c-late" {
		t.Fatalf("Connections = %+v, %v", got, err)
	}
}

func TestConnectionRoundTripNullHandling(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	t0 := clock.Now()

	mustSave(t, s, newConn("c1", urlA, t0))
	got, err := s.ConnectionByID(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSession != "" || !got.BaselinedAt.IsZero() {
		t.Errorf("NULL columns must read as zero values: %+v", got)
	}
	if !got.CreatedAt.Equal(t0) || !got.UpdatedAt.Equal(t0) || got.Name != "name-c1" || got.URL != urlA {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if n := count(t, s, "SELECT count(*) FROM connections WHERE last_session IS NULL AND baselined_at IS NULL"); n != 1 {
		t.Error("zero values must be stored as NULL")
	}

	full := newConn("c2", urlB, t0)
	full.LastSession = "work"
	full.BaselinedAt = t0.Add(time.Minute)
	mustSave(t, s, full)
	got, err = s.ConnectionByID(ctx, "c2")
	if err != nil || got.LastSession != "work" || !got.BaselinedAt.Equal(full.BaselinedAt) {
		t.Errorf("full round trip = %+v, %v", got, err)
	}
}

func TestConnectionByIDNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	_, err := s.ConnectionByID(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v; want ErrNotFound and sql.ErrNoRows", err)
	}
}

func TestConnectionByURL(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	got, ok, err := s.ConnectionByURL(ctx, urlA)
	if err != nil || !ok || got.ID != "c1" {
		t.Fatalf("ConnectionByURL = %+v, %v, %v", got, ok, err)
	}
	got, ok, err = s.ConnectionByURL(ctx, urlB)
	if err != nil || ok || got.ID != "" {
		t.Fatalf("missing url = %+v, %v, %v; want zero, false, nil", got, ok, err)
	}
}

func TestSaveConnectionUpsertByURLKeepsIdentity(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	t0 := clock.Now()

	orig := newConn("c1", urlA, t0)
	orig.LastSession = "work"
	orig.BaselinedAt = t0.Add(time.Minute)
	mustSave(t, s, orig)

	// 再ペアリング: 同じ url・新しい id・新しい名前・baselined_at なし。
	t1 := t0.Add(time.Hour)
	again := domain.Connection{ID: "c-new", Name: "renamed", URL: urlA, CreatedAt: t1, UpdatedAt: t1}
	mustSave(t, s, again)

	all, err := s.Connections(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("Connections = %+v, %v; want exactly 1 row", all, err)
	}
	got := all[0]
	if got.ID != "c1" {
		t.Errorf("id = %q; UPSERT must keep the existing id", got.ID)
	}
	if got.Name != "renamed" || !got.UpdatedAt.Equal(t1) {
		t.Errorf("name/updated_at not replaced: %+v", got)
	}
	if !got.CreatedAt.Equal(t0) {
		t.Errorf("created_at = %v; must be kept", got.CreatedAt)
	}
	if !got.BaselinedAt.Equal(orig.BaselinedAt) {
		t.Errorf("baselined_at = %v; must be kept (ER §3.1)", got.BaselinedAt)
	}
	if got.LastSession != "work" {
		t.Errorf("last_session = %q; must be kept", got.LastSession)
	}
}

func TestSaveConnectionConstraints(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	t0 := clock.Now()

	bad := newConn("c1", urlA, t0)
	bad.Name = ""
	if err := s.SaveConnection(ctx, bad); err == nil {
		t.Error("empty name must be rejected by CHECK")
	}
	long := make([]rune, 101)
	for i := range long {
		long[i] = 'あ'
	}
	bad.Name = string(long)
	if err := s.SaveConnection(ctx, bad); err == nil {
		t.Error("101-character name must be rejected by CHECK")
	}
	ok := newConn("c1", urlA, t0)
	ok.Name = string(long[:100])
	if err := s.SaveConnection(ctx, ok); err != nil {
		t.Errorf("100-character multibyte name must be accepted: %v", err)
	}
	// 同じ id・別 url は一意制約で失敗する（黙って上書きしない）。
	dup := newConn("c1", urlB, t0)
	if err := s.SaveConnection(ctx, dup); err == nil {
		t.Error("same id with a different url must fail")
	}
}

func TestSetLastSessionAndMarkBaselined(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	t0 := clock.Now()
	mustSave(t, s, newConn("c1", urlA, t0))

	clock.Advance(time.Hour)
	if err := s.SetLastSession(ctx, "c1", "work"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ConnectionByID(ctx, "c1")
	if got.LastSession != "work" || !got.UpdatedAt.Equal(clock.Now()) {
		t.Errorf("after SetLastSession: %+v", got)
	}
	if err := s.SetLastSession(ctx, "c1", ""); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "SELECT count(*) FROM connections WHERE last_session IS NULL"); n != 1 {
		t.Error("empty session must store NULL")
	}

	first := t0.Add(2 * time.Hour)
	if err := s.MarkBaselined(ctx, "c1", first); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkBaselined(ctx, "c1", first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ConnectionByID(ctx, "c1")
	if !got.Baselined() || !got.BaselinedAt.Equal(first) {
		t.Errorf("baselined_at = %v; the first time must be kept", got.BaselinedAt)
	}

	if err := s.SetLastSession(ctx, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetLastSession on missing = %v; want ErrNotFound", err)
	}
	if err := s.MarkBaselined(ctx, "nope", first); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkBaselined on missing = %v; want ErrNotFound", err)
	}
}

func TestDeleteConnectionCascades(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	t0 := clock.Now()
	mustSave(t, s, newConn("c1", urlA, t0))
	mustSave(t, s, newConn("c2", urlB, t0))

	for _, id := range []string{"c1", "c2"} {
		if err := s.SaveSnapshot(ctx, id, "work", domain.Snapshot{FetchedAt: t0}); err != nil {
			t.Fatal(err)
		}
		d := domain.Diff{Insert: []domain.NotifiedKey{{PaneID: "%1", State: domain.StateAsking}}}
		if _, err := s.ApplyNotified(ctx, id, "work", d); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetLastConnection(ctx, "c1"); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteConnection(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "SELECT count(*) FROM snapshots WHERE connection_id = 'c1'"); n != 0 {
		t.Errorf("snapshots of deleted connection = %d", n)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states WHERE connection_id = 'c1'"); n != 0 {
		t.Errorf("notified_states of deleted connection = %d", n)
	}
	if n := count(t, s, "SELECT count(*) FROM snapshots WHERE connection_id = 'c2'"); n != 1 {
		t.Error("other connection's snapshot must remain")
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states WHERE connection_id = 'c2'"); n != 1 {
		t.Error("other connection's notified_states must remain")
	}
	st, err := s.Settings(ctx)
	if err != nil || st.LastConnectionID != "" {
		t.Errorf("last_connection_id after delete = %q, %v; want NULL (empty)", st.LastConnectionID, err)
	}

	if err := s.DeleteConnection(ctx, "c1"); err != nil {
		t.Errorf("deleting a missing connection must not fail: %v", err)
	}
}
