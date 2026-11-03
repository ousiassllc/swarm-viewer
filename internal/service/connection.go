package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

type tabsEntry struct {
	sessions  []domain.Session
	fetchedAt time.Time
}

// ConnectionService は接続の一覧・削除・セッションタブ・needs_repair を扱う（FR-PAIR-3・4、FR-SESS-1〜3）。
type ConnectionService struct {
	report  ReportClient
	store   Store
	secrets SecretStore
	clock   Clock
	log     *slog.Logger

	mu       sync.RWMutex
	repair   map[string]bool
	tabs     map[string]tabsEntry
	onDelete []func(id string)
}

// NewConnectionService は ConnectionService を作る。
func NewConnectionService(report ReportClient, store Store, secrets SecretStore, clock Clock, log *slog.Logger) *ConnectionService {
	return &ConnectionService{
		report:  report,
		store:   store,
		secrets: secrets,
		clock:   clock,
		log:     log,
		repair:  make(map[string]bool),
		tabs:    make(map[string]tabsEntry),
	}
}

// OnDelete は接続削除時に呼ぶ解放処理を登録する（Snapshot・Notification のメモリ上の状態用）。
// 呼び出しは配線時（goroutine を起こす前）に行う。
func (s *ConnectionService) OnDelete(fn func(id string)) {
	s.mu.Lock()
	s.onDelete = append(s.onDelete, fn)
	s.mu.Unlock()
}

// List は保存済みの接続を返す。
func (s *ConnectionService) List(ctx context.Context) ([]domain.Connection, error) {
	return s.store.Connections(ctx)
}

// Get は接続を 1 件返す。無ければ ErrConnectionNotFound。
func (s *ConnectionService) Get(ctx context.Context, id string) (domain.Connection, error) {
	conn, err := s.store.ConnectionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Connection{}, ErrConnectionNotFound
		}
		return domain.Connection{}, err
	}
	return conn, nil
}

// Delete は接続を削除する。先に Store.DeleteConnection、次に秘密領域（取りこぼしを減らす順序。ER §4）。
// 秘密領域の削除失敗は警告ログのみ。メモリ上の状態（needs_repair・タブ・通知用の mutex など）も解放する。
func (s *ConnectionService) Delete(ctx context.Context, id string) error {
	if err := s.store.DeleteConnection(ctx, id); err != nil {
		return err
	}
	if err := s.secrets.Delete(id); err != nil {
		s.log.Warn("secret entry was not deleted", "connection_id", id, "err", err)
	}

	s.mu.Lock()
	delete(s.repair, id)
	delete(s.tabs, id)
	hooks := append([]func(string){}, s.onDelete...)
	s.mu.Unlock()
	for _, fn := range hooks {
		fn(id)
	}
	return nil
}

// Tabs は sessions_list を呼び、許可セッションを返す。呼び出しごとに取得する（通知用ポーリングとは共有しない）。
// 失敗は *domain.FetchError。401/403 なら needs_repair を立てる。
func (s *ConnectionService) Tabs(ctx context.Context, id string) ([]domain.Session, error) {
	conn, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	all, err := s.report.Sessions(ctx, conn)
	if err != nil {
		fe := classify(err)
		s.noteFetchError(id, fe)
		s.log.Warn("sessions_list failed", fetchLogAttrs(conn, "", fe)...)
		return nil, fe
	}
	allowed := allowedSessions(all)
	s.mu.Lock()
	s.tabs[id] = tabsEntry{sessions: allowed, fetchedAt: s.clock.Now()}
	s.mu.Unlock()
	return append([]domain.Session(nil), allowed...), nil
}

// CachedTabs は直近の Tabs の結果（取得失敗時の表示用）。無ければ ok=false。
func (s *ConnectionService) CachedTabs(id string) (sessions []domain.Session, fetchedAt time.Time, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.tabs[id]
	if !ok {
		return nil, time.Time{}, false
	}
	return append([]domain.Session(nil), e.sessions...), e.fetchedAt, true
}

// ResolveSession は表示するセッションを決める。requested が許可セッションならそれ、許可外・消滅・空なら先頭の許可セッション。
// 許可セッションが 0 件なら ErrNoSessions。
func (s *ConnectionService) ResolveSession(ctx context.Context, id, requested string) (string, error) {
	tabs, err := s.Tabs(ctx, id)
	if err != nil {
		return "", err
	}
	if len(tabs) == 0 {
		return "", ErrNoSessions
	}
	for _, t := range tabs {
		if t.ID == requested {
			return requested, nil
		}
	}
	return tabs[0].ID, nil
}

// RememberSession は接続の last_session を記録する（FR-SESS-3）。
func (s *ConnectionService) RememberSession(ctx context.Context, id, session string) error {
	return s.store.SetLastSession(ctx, id, session)
}

// SetLastConnection は settings.last_connection_id を記録する。空文字は NULL に戻す。
func (s *ConnectionService) SetLastConnection(ctx context.Context, id string) error {
	return s.store.SetLastConnection(ctx, id)
}

// NeedsRepair は再ペアリングが必要か（401/403 を受けた）を返す。true の間は自動取得を止める。
func (s *ConnectionService) NeedsRepair(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repair[id]
}

// MarkNeedsRepair は再ペアリングが必要な状態にする。
func (s *ConnectionService) MarkNeedsRepair(id string) {
	s.mu.Lock()
	s.repair[id] = true
	s.mu.Unlock()
}

// MarkRepaired は再ペアリング済みにする。
func (s *ConnectionService) MarkRepaired(id string) {
	s.mu.Lock()
	delete(s.repair, id)
	s.mu.Unlock()
}

// noteFetchError は認証・スコープのエラー（401/403）のとき needs_repair を立てる。
func (s *ConnectionService) noteFetchError(id string, fe *domain.FetchError) {
	if needsRepair(fe) {
		s.MarkNeedsRepair(id)
	}
}

// classify は取得失敗を FetchError に分類する。分類できないときも nil を返さない。
func classify(err error) *domain.FetchError {
	if fe := domain.Classify(err, 0, ""); fe != nil {
		return fe
	}
	return &domain.FetchError{Kind: domain.ErrProtocol}
}

func needsRepair(fe *domain.FetchError) bool {
	return fe != nil && (fe.Kind == domain.ErrUnauthorized || fe.Kind == domain.ErrForbiddenScope)
}

// allowedSessions は許可されたセッションだけに絞る（順序は保つ）。
func allowedSessions(in []domain.Session) []domain.Session {
	out := make([]domain.Session, 0, len(in))
	for _, s := range in {
		if s.Allowed && s.ID != "" {
			out = append(out, s)
		}
	}
	return out
}

// fetchLogAttrs は取得失敗のログ属性（種別とコードのみ）。
func fetchLogAttrs(conn domain.Connection, session string, fe *domain.FetchError) []any {
	attrs := []any{"connection", conn.Name, "host", conn.Host()}
	if session != "" {
		attrs = append(attrs, "session", session)
	}
	if fe != nil {
		attrs = append(attrs, "kind", fe.Kind.String(), "code", fe.Code)
	}
	return attrs
}
