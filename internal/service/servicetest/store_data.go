package servicetest

import (
	"context"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// snapshots・notified_states と、テスト用の参照ヘルパー。

func (s *Store) Snapshot(_ context.Context, connID, session string) (domain.Snapshot, bool, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("Snapshot"); err != nil {
		return domain.Snapshot{}, false, err
	}
	sn, ok := s.Snaps[SnapKey{connID, session}]
	return sn, ok, nil
}

func (s *Store) SaveSnapshot(_ context.Context, connID, session string, sn domain.Snapshot) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("SaveSnapshot"); err != nil {
		return err
	}
	s.Snaps[SnapKey{connID, session}] = sn
	return nil
}

func (s *Store) PruneSnapshots(_ context.Context, connID string, keep []string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("PruneSnapshots"); err != nil {
		return err
	}
	s.Pruned = append(s.Pruned, Prune{Conn: connID, Keep: append([]string(nil), keep...)})
	return nil
}

func (s *Store) NotifiedKeys(_ context.Context, connID, session string) ([]domain.NotifiedKey, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("NotifiedKeys"); err != nil {
		return nil, err
	}
	var out []domain.NotifiedKey
	for k := range s.Notified[SnapKey{connID, session}] {
		out = append(out, k)
	}
	return out, nil
}

func (s *Store) ApplyNotified(_ context.Context, connID, session string, d domain.Diff) ([]domain.NotifiedKey, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("ApplyNotified"); err != nil {
		return nil, err
	}
	set := s.Notified[SnapKey{connID, session}]
	if set == nil {
		set = map[domain.NotifiedKey]struct{}{}
		s.Notified[SnapKey{connID, session}] = set
	}
	for _, k := range d.Delete {
		delete(set, k)
	}
	var inserted []domain.NotifiedKey
	for _, k := range d.Insert {
		if _, dup := set[k]; dup || s.IgnoreInsert[k] {
			set[k] = struct{}{}
			continue
		}
		set[k] = struct{}{}
		inserted = append(inserted, k)
	}
	return inserted, nil
}

// --- 参照ヘルパー ---

// Conn は保存済みの接続を返す。
func (s *Store) Conn(id string) (domain.Connection, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for _, c := range s.Conns {
		if c.ID == id {
			return c, true
		}
	}
	return domain.Connection{}, false
}

// ConnCount は保存済みの接続数。
func (s *Store) ConnCount() int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return len(s.Conns)
}

// NotifiedSet は通知済み記録の集合。
func (s *Store) NotifiedSet(connID, session string) map[domain.NotifiedKey]bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	out := map[domain.NotifiedKey]bool{}
	for k := range s.Notified[SnapKey{connID, session}] {
		out[k] = true
	}
	return out
}

// Snap は保存済みのスナップショット。
func (s *Store) Snap(connID, session string) (domain.Snapshot, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	sn, ok := s.Snaps[SnapKey{connID, session}]
	return sn, ok
}
