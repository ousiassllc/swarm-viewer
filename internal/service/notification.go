package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// NotificationService は状態遷移の検知・OS 通知の送出・通知用ポーリングのスケジューラ（FR-NTF-1〜3）。
type NotificationService struct {
	report   ReportClient
	store    Store
	notifier Notifier
	clock    Clock
	settings *SettingsService
	conns    *ConnectionService
	log      *slog.Logger

	mu      sync.Mutex
	locks   map[string]*sync.Mutex // 接続 ID → Observe の直列化用
	wakeIDs map[string]struct{}    // Wake で依頼された、直ちに通知用ポーリングを回す接続
	wake    chan struct{}          // wakeIDs に依頼が入ったことを Run へ知らせる（容量 1）

	afterRound func() // テスト用。通知用ポーリングの一巡（スキップを含む）が終わるたびに呼ぶ
}

// NewNotificationService は NotificationService を作り、接続削除時に mutex を解放するよう登録する。
func NewNotificationService(report ReportClient, store Store, notifier Notifier, clock Clock, settings *SettingsService, conns *ConnectionService, log *slog.Logger) *NotificationService {
	s := &NotificationService{
		report:   report,
		store:    store,
		notifier: notifier,
		clock:    clock,
		settings: settings,
		conns:    conns,
		log:      log,
		locks:    make(map[string]*sync.Mutex),
		wakeIDs:  make(map[string]struct{}),
		wake:     make(chan struct{}, 1),
	}
	conns.OnDelete(s.forget)
	return s
}

// Observe は取得に成功した report を通知判定へ流す。画面更新・通知用ポーリングの両方が呼ぶ。
//
// 通知済み記録は常に更新する。OS 通知を出すのは、baselining（conn.BaselinedAt が未設定）でなく、
// 通知がオンで、ApplyNotified が実際に挿入できたキーに対応するペインだけ。通知の失敗は警告ログのみで記録は戻さない。
func (s *NotificationService) Observe(ctx context.Context, conn domain.Connection, session string, r domain.Report) error {
	mu := s.lockFor(conn.ID)
	mu.Lock()
	defer mu.Unlock()

	st, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	known, err := s.store.NotifiedKeys(ctx, conn.ID, session)
	if err != nil {
		return err
	}
	baselining := !conn.Baselined()
	diff := domain.Detect(known, r.Panes, baselining)
	if len(diff.Insert) == 0 && len(diff.Delete) == 0 {
		return nil
	}
	inserted, err := s.store.ApplyNotified(ctx, conn.ID, session, diff)
	if err != nil {
		return err
	}
	if baselining || !st.NotificationsEnabled {
		return nil
	}

	insertedSet := make(map[domain.NotifiedKey]struct{}, len(inserted))
	for _, k := range inserted {
		insertedSet[k] = struct{}{}
	}
	for _, ev := range diff.Notify {
		if _, ok := insertedSet[domain.NotifiedKey{PaneID: ev.Pane.ID, State: ev.State}]; !ok {
			continue // 別経路が先に記録した。二重通知を防ぐ
		}
		n := Notification{
			ConnectionID:   conn.ID,
			ConnectionName: conn.Name,
			Session:        session,
			Pos:            ev.Pane.Pos,
			State:          ev.State,
		}
		if ev.State == domain.StateAsking {
			n.Detail = askingDetail(r.Attention, ev.Pane)
		}
		if err := s.notifier.Notify(ctx, n); err != nil {
			s.log.Warn("notify failed", "connection", conn.Name, "session", session, "pos", ev.Pane.Pos, "state", string(ev.State), "err", err)
		}
	}
	return nil
}

// askingDetail は asking のペインに対応する attention の detail（質問文の先頭）を返す。無ければ空。
func askingDetail(att []domain.Attention, p domain.Pane) string {
	for _, a := range att {
		if a.Kind == string(domain.StateAsking) && (a.Pane == p.Pos || a.Pane == p.ID) {
			return a.Detail
		}
	}
	return ""
}

func (s *NotificationService) lockFor(connID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	mu, ok := s.locks[connID]
	if !ok {
		mu = &sync.Mutex{}
		s.locks[connID] = mu
	}
	return mu
}

func (s *NotificationService) forget(connID string) {
	s.mu.Lock()
	delete(s.locks, connID)
	delete(s.wakeIDs, connID)
	s.mu.Unlock()
}
