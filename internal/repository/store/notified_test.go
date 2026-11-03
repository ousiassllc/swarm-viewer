package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func key(pane string, st domain.PaneState) domain.NotifiedKey {
	return domain.NotifiedKey{PaneID: pane, State: st}
}

func insertDiff(keys ...domain.NotifiedKey) domain.Diff { return domain.Diff{Insert: keys} }

func TestApplyNotifiedInsertDeleteAndRead(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	if got, err := s.NotifiedKeys(ctx, "c1", "work"); err != nil || len(got) != 0 {
		t.Fatalf("empty NotifiedKeys = %v, %v", got, err)
	}
	ins, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(key("%2", domain.StateStalled), key("%1", domain.StateAsking)))
	if err != nil || len(ins) != 2 || ins[0].PaneID != "%2" || ins[1].PaneID != "%1" {
		t.Fatalf("ApplyNotified = %v, %v; want both keys in input order", ins, err)
	}
	got, err := s.NotifiedKeys(ctx, "c1", "work")
	want := []domain.NotifiedKey{key("%1", domain.StateAsking), key("%2", domain.StateStalled)}
	if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("NotifiedKeys = %v, %v; want %v", got, err, want)
	}
	// 別セッション・別接続からは見えない。
	if got, _ := s.NotifiedKeys(ctx, "c1", "other"); len(got) != 0 {
		t.Errorf("leaked across sessions: %v", got)
	}

	// asking → stalled: 古い行を消して新しい行を入れる。
	d := domain.Diff{
		Delete: []domain.NotifiedKey{key("%1", domain.StateAsking)},
		Insert: []domain.NotifiedKey{key("%1", domain.StateStalled)},
	}
	ins, err = s.ApplyNotified(ctx, "c1", "work", d)
	if err != nil || len(ins) != 1 || ins[0] != key("%1", domain.StateStalled) {
		t.Fatalf("transition = %v, %v", ins, err)
	}
	got, _ = s.NotifiedKeys(ctx, "c1", "work")
	want = []domain.NotifiedKey{key("%1", domain.StateStalled), key("%2", domain.StateStalled)}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("after transition = %v, want %v", got, want)
	}

	// 空の Diff は何も変えない。
	if ins, err := s.ApplyNotified(ctx, "c1", "work", domain.Diff{}); err != nil || len(ins) != 0 {
		t.Errorf("empty diff = %v, %v", ins, err)
	}
	// 存在しない行の削除はエラーにしない。
	if _, err := s.ApplyNotified(ctx, "c1", "work", domain.Diff{Delete: []domain.NotifiedKey{key("%9", domain.StateAsking)}}); err != nil {
		t.Errorf("deleting a missing key: %v", err)
	}
}

func TestApplyNotifiedDoubleInsertReturnsOnce(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	d := insertDiff(key("%1", domain.StateAsking))

	first, err := s.ApplyNotified(ctx, "c1", "work", d)
	if err != nil || len(first) != 1 {
		t.Fatalf("first = %v, %v", first, err)
	}
	second, err := s.ApplyNotified(ctx, "c1", "work", d)
	if err != nil || len(second) != 0 {
		t.Fatalf("second = %v, %v; want nothing re-inserted", second, err)
	}
	// 同じ Diff 内の重複・既存と新規の混在でも、実際に入ったものだけを返す。
	mixed, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(
		key("%1", domain.StateAsking), key("%2", domain.StateAsking), key("%2", domain.StateAsking)))
	if err != nil || len(mixed) != 1 || mixed[0].PaneID != "%2" {
		t.Fatalf("mixed = %v, %v; want only %%2 once", mixed, err)
	}
	// 同じペインでも状態が違えば別のキー。
	other, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(key("%1", domain.StateStalled)))
	if err != nil || len(other) != 1 {
		t.Fatalf("other state = %v, %v", other, err)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states"); n != 3 {
		t.Errorf("rows = %d, want 3", n)
	}
}

func TestApplyNotifiedConcurrentSingleWinner(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	const workers = 16
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		total    int
		firstErr error
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ins, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(key("%1", domain.StateAsking)))
			mu.Lock()
			defer mu.Unlock()
			total += len(ins)
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}()
	}
	close(start)
	wg.Wait()

	if firstErr != nil {
		t.Fatalf("concurrent ApplyNotified failed: %v", firstErr)
	}
	if total != 1 {
		t.Fatalf("inserted reported %d times across %d callers; want exactly once", total, workers)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states"); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestApplyNotifiedCapEvictsOldest(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	mustSave(t, s, newConn("c2", urlB, clock.Now()))

	paneKeys := func(prefix string, n int) []domain.NotifiedKey {
		keys := make([]domain.NotifiedKey, n)
		for i := range keys {
			keys[i] = key(fmt.Sprintf("%%%s%04d", prefix, i), domain.StateAsking)
		}
		return keys
	}

	// ちょうど上限: 何も消えない。
	if ins, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(paneKeys("old", MaxNotifiedPerConnection)...)); err != nil || len(ins) != MaxNotifiedPerConnection {
		t.Fatalf("fill = %d, %v", len(ins), err)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states WHERE connection_id = 'c1'"); n != MaxNotifiedPerConnection {
		t.Fatalf("rows at cap = %d", n)
	}
	// 別の接続は数えない。
	if _, err := s.ApplyNotified(ctx, "c2", "work", insertDiff(paneKeys("other", 3)...)); err != nil {
		t.Fatal(err)
	}

	// 上限を 5 件超える: 古い（先に入れた）5 件が消える。別セッションの行も同じ接続として数える。
	clock.Advance(time.Minute)
	if ins, err := s.ApplyNotified(ctx, "c1", "second-session", insertDiff(paneKeys("new", 5)...)); err != nil || len(ins) != 5 {
		t.Fatalf("overflow = %d, %v", len(ins), err)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states WHERE connection_id = 'c1'"); n != MaxNotifiedPerConnection {
		t.Fatalf("rows after overflow = %d, want %d", n, MaxNotifiedPerConnection)
	}
	oldKeys, _ := s.NotifiedKeys(ctx, "c1", "work")
	have := map[string]bool{}
	for _, k := range oldKeys {
		have[k.PaneID] = true
	}
	for i := range 5 {
		if have[fmt.Sprintf("%%old%04d", i)] {
			t.Errorf("oldest key %d must be evicted", i)
		}
	}
	for _, i := range []int{5, 6, MaxNotifiedPerConnection - 1} {
		if !have[fmt.Sprintf("%%old%04d", i)] {
			t.Errorf("key %d must survive", i)
		}
	}
	newKeys, _ := s.NotifiedKeys(ctx, "c1", "second-session")
	if len(newKeys) != 5 {
		t.Errorf("newest keys = %d, want all 5 kept", len(newKeys))
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states WHERE connection_id = 'c2'"); n != 3 {
		t.Errorf("other connection rows = %d, want 3 (cap is per connection)", n)
	}
}

func TestApplyNotifiedMissingConnection(t *testing.T) {
	s, _ := newTestStore(t)
	ins, err := s.ApplyNotified(context.Background(), "ghost", "work", insertDiff(key("%1", domain.StateAsking)))
	if !errors.Is(err, ErrNotFound) || len(ins) != 0 {
		t.Fatalf("= %v, %v; want ErrNotFound", ins, err)
	}
}

func TestApplyNotifiedIgnoresNonNotifiableState(t *testing.T) {
	s, clock := newTestStore(t)
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	ins, err := s.ApplyNotified(context.Background(), "c1", "work",
		insertDiff(key("%1", domain.StateWorking), key("%2", domain.StateAsking)))
	if err != nil || len(ins) != 1 || ins[0].PaneID != "%2" {
		t.Fatalf("= %v, %v; want only the notifiable key", ins, err)
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states"); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestApplyNotifiedCanceledContextRollsBack(t *testing.T) {
	s, clock := newTestStore(t)
	mustSave(t, s, newConn("c1", urlA, clock.Now()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ApplyNotified(ctx, "c1", "work", insertDiff(key("%1", domain.StateAsking))); err == nil {
		t.Fatal("canceled context must fail")
	}
	if n := count(t, s, "SELECT count(*) FROM notified_states"); n != 0 {
		t.Errorf("rows = %d after failed apply", n)
	}
}
