package servicetest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Recorder は Store・SecretStore の呼び出し順を共有して記録する。
type Recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *Recorder) Add(s string) {
	r.mu.Lock()
	r.calls = append(r.calls, s)
	r.mu.Unlock()
}

func (r *Recorder) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// SnapKey は (接続, セッション) の組。
type SnapKey struct{ Conn, Session string }

// Store は service.Store のメモリ実装。実装の契約（UPSERT・ON DELETE CASCADE・OR IGNORE）を再現する。
type Store struct {
	rec *Recorder

	Mu       sync.Mutex // 以下のフィールドを直接参照・更新するテスト用
	Conns    []domain.Connection
	Cfg      domain.Settings
	Snaps    map[SnapKey]domain.Snapshot
	Notified map[SnapKey]map[domain.NotifiedKey]struct{}

	IgnoreInsert map[domain.NotifiedKey]bool // 別経路が先に挿入済みのキー（inserted に含めない）
	errs         map[string]error            // メソッド名 → 注入するエラー
	Pruned       []Prune
}

// Prune は PruneSnapshots の呼び出し記録。
type Prune struct {
	Conn string
	Keep []string
}

// NewStore は空の Store を作る。
func NewStore(rec *Recorder) *Store {
	return &Store{
		rec:          rec,
		Cfg:          domain.DefaultSettings(),
		Snaps:        map[SnapKey]domain.Snapshot{},
		Notified:     map[SnapKey]map[domain.NotifiedKey]struct{}{},
		IgnoreInsert: map[domain.NotifiedKey]bool{},
		errs:         map[string]error{},
	}
}

func (s *Store) fail(method string) error {
	s.rec.Add("store." + method)
	return s.errs[method]
}

// SetErr は method（Store のメソッド名）が返すエラーを設定する。nil で解除。
func (s *Store) SetErr(method string, err error) {
	s.Mu.Lock()
	s.errs[method] = err
	s.Mu.Unlock()
}

func (s *Store) Connections(context.Context) ([]domain.Connection, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("Connections"); err != nil {
		return nil, err
	}
	return append([]domain.Connection(nil), s.Conns...), nil
}

func (s *Store) ConnectionByID(_ context.Context, id string) (domain.Connection, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("ConnectionByID"); err != nil {
		return domain.Connection{}, err
	}
	for _, c := range s.Conns {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Connection{}, fmt.Errorf("store: not found: %w", sql.ErrNoRows)
}

func (s *Store) ConnectionByURL(_ context.Context, url string) (domain.Connection, bool, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("ConnectionByURL"); err != nil {
		return domain.Connection{}, false, err
	}
	for _, c := range s.Conns {
		if c.URL == url {
			return c, true, nil
		}
	}
	return domain.Connection{}, false, nil
}

func (s *Store) SaveConnection(_ context.Context, c domain.Connection) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("SaveConnection"); err != nil {
		return err
	}
	for i := range s.Conns {
		if s.Conns[i].URL == c.URL { // url で一意の UPSERT。id は既存を維持する
			c.ID = s.Conns[i].ID
			s.Conns[i] = c
			return nil
		}
	}
	s.Conns = append(s.Conns, c)
	return nil
}

func (s *Store) DeleteConnection(_ context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("DeleteConnection"); err != nil {
		return err
	}
	for i, c := range s.Conns {
		if c.ID == id {
			s.Conns = append(s.Conns[:i], s.Conns[i+1:]...)
			break
		}
	}
	for k := range s.Snaps {
		if k.Conn == id {
			delete(s.Snaps, k)
		}
	}
	for k := range s.Notified {
		if k.Conn == id {
			delete(s.Notified, k)
		}
	}
	return nil
}

func (s *Store) SetLastSession(_ context.Context, id, session string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("SetLastSession"); err != nil {
		return err
	}
	for i := range s.Conns {
		if s.Conns[i].ID == id {
			s.Conns[i].LastSession = session
		}
	}
	return nil
}

func (s *Store) MarkBaselined(_ context.Context, id string, at time.Time) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("MarkBaselined"); err != nil {
		return err
	}
	for i := range s.Conns {
		if s.Conns[i].ID == id && s.Conns[i].BaselinedAt.IsZero() {
			s.Conns[i].BaselinedAt = at
		}
	}
	return nil
}

func (s *Store) Settings(context.Context) (domain.Settings, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("Settings"); err != nil {
		return domain.Settings{}, err
	}
	return s.Cfg, nil
}

func (s *Store) SaveSettings(_ context.Context, st domain.Settings) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("SaveSettings"); err != nil {
		return err
	}
	st.LastConnectionID = s.Cfg.LastConnectionID // 実装は last_connection_id を書かない
	s.Cfg = st
	return nil
}

func (s *Store) SetLastConnection(_ context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if err := s.fail("SetLastConnection"); err != nil {
		return err
	}
	s.Cfg.LastConnectionID = id
	return nil
}

// ErrInjected は注入する失敗。
var ErrInjected = errors.New("injected failure")
