package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// fakeClock はテスト用の進められる時計。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func dbPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "swarm-viewer", "viewer.db")
}

// newTestStore は一時ファイルの DB を開く。
func newTestStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	s, err := open(dbPath(t), clock.Now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, clock
}

func newConn(id, url string, at time.Time) domain.Connection {
	return domain.Connection{ID: id, Name: "name-" + id, URL: url, CreatedAt: at, UpdatedAt: at}
}

func mustSave(t *testing.T, s *Store, c domain.Connection) {
	t.Helper()
	if err := s.SaveConnection(context.Background(), c); err != nil {
		t.Fatalf("SaveConnection(%s): %v", c.ID, err)
	}
}

func count(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}
