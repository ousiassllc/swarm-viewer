package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// SnapshotResult は Get の結果。
type SnapshotResult struct {
	Snapshot domain.Snapshot    // 取得成功なら最新、失敗なら直前（メモリ → snapshots。Stale: true）。無ければ零値
	Fresh    bool               // 今回の取得が成功したか
	Err      *domain.FetchError // 失敗の種別。Fresh のとき nil
}

// SnapshotService は選択中セッションの report 取得・直近値の保持・失敗時のフォールバック（FR-POLL、FR-ERR-1）。
type SnapshotService struct {
	report ReportClient
	store  Store
	notif  *NotificationService
	conns  *ConnectionService
	clock  Clock
	log    *slog.Logger

	group singleflight.Group

	mu    sync.RWMutex
	cache map[string]domain.Snapshot // snapshotKey → 直近の成功値

	afterJoin func() // テスト用。同じ呼び出しに合流した直後に呼ぶ
}

// NewSnapshotService は SnapshotService を作り、接続削除時にメモリ上の値を解放するよう登録する。
func NewSnapshotService(report ReportClient, store Store, notif *NotificationService, conns *ConnectionService, clock Clock, log *slog.Logger) *SnapshotService {
	s := &SnapshotService{
		report: report,
		store:  store,
		notif:  notif,
		conns:  conns,
		clock:  clock,
		log:    log,
		cache:  make(map[string]domain.Snapshot),
	}
	conns.OnDelete(s.forget)
	return s
}

func snapshotKey(connID, session string) string { return connID + "\x00" + session }

// Get は (接続, セッション) の report を取得する。同じキーの同時呼び出しは report 1 本にまとまる。
//
// 成功: メモリ更新 → SaveSnapshot → Observe（通知のオン/オフに関わらず）。
// 失敗: Err に分類済みエラー、Snapshot は直前値（Stale）。401/403 は needs_repair を立てる。
// needs_repair の接続は取得せず即座に ErrUnauthorized を返す（自動ポーリングの停止）。
func (s *SnapshotService) Get(ctx context.Context, connID, session string) SnapshotResult {
	if s.conns.NeedsRepair(connID) {
		return s.failed(ctx, connID, session, &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "needs_repair"})
	}
	// 共有する取得は呼び出し元の ctx から切り離す（先頭の呼び出し元がキャンセルしても他の待ち手を巻き込まない）。
	// 各呼び出し元は自分の ctx で待ちをやめられる。report 側に 10 秒のタイムアウトがある。
	shared := context.WithoutCancel(ctx)
	ch := s.group.DoChan(snapshotKey(connID, session), func() (any, error) {
		return s.fetch(shared, connID, session), nil
	})
	if s.afterJoin != nil {
		s.afterJoin()
	}
	select {
	case r := <-ch:
		return r.Val.(SnapshotResult)
	case <-ctx.Done():
		return s.failed(ctx, connID, session, &domain.FetchError{Kind: domain.ErrUnreachable, Code: "canceled"})
	}
}

func (s *SnapshotService) fetch(ctx context.Context, connID, session string) SnapshotResult {
	conn, err := s.conns.Get(ctx, connID)
	if err != nil {
		s.log.Warn("connection unavailable for snapshot", "connection_id", connID, "err", err)
		return s.failed(ctx, connID, session, &domain.FetchError{Kind: domain.ErrProtocol, Code: "connection_unavailable"})
	}
	r, err := s.report.Report(ctx, conn, session)
	if err != nil {
		fe := classify(err)
		if needsRepair(fe) {
			s.conns.MarkNeedsRepair(connID)
		}
		s.log.Warn("report failed", fetchLogAttrs(conn, session, fe)...)
		return s.failed(ctx, connID, session, fe)
	}

	sn := domain.Snapshot{Report: r, FetchedAt: s.clock.Now()}
	s.mu.Lock()
	s.cache[snapshotKey(connID, session)] = sn
	s.mu.Unlock()
	if err := s.store.SaveSnapshot(ctx, connID, session, sn); err != nil {
		s.log.Warn("saving snapshot failed", "connection", conn.Name, "session", session, "err", err)
	}
	if err := s.notif.Observe(ctx, conn, session, r); err != nil {
		s.log.Warn("observe failed", "connection", conn.Name, "session", session, "err", err)
	}
	return SnapshotResult{Snapshot: sn, Fresh: true}
}

// failed は失敗結果を作る。直前値はメモリと snapshots のうち新しいほう（Stale: true）。
func (s *SnapshotService) failed(ctx context.Context, connID, session string, fe *domain.FetchError) SnapshotResult {
	ctx = context.WithoutCancel(ctx) // 呼び出し元がキャンセル済みでも直前値は返す
	res := SnapshotResult{Err: fe}
	s.mu.RLock()
	mem, inMem := s.cache[snapshotKey(connID, session)]
	s.mu.RUnlock()

	prev, found, err := s.store.Snapshot(ctx, connID, session)
	if err != nil {
		s.log.Warn("loading snapshot failed", "connection_id", connID, "session", session, "err", err)
	}
	switch {
	case inMem && (!found || !prev.FetchedAt.After(mem.FetchedAt)):
		prev = mem
	case !found:
		return res
	}
	prev.Stale = true
	res.Snapshot = prev
	return res
}

func (s *SnapshotService) forget(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := connID + "\x00"
	for k := range s.cache {
		if strings.HasPrefix(k, prefix) {
			delete(s.cache, k)
		}
	}
}
