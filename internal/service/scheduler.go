package service

import (
	"context"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Run は通知用ポーリングのスケジューラ。ctx のキャンセルで返る。
//
// 起動直後に 1 回実行し、以降は Clock の ticker で回す。通知がオフの間は tick しても取得しない。
// 設定の変更（SettingsService.Subscribe）で ticker を Reset し、有効/無効に追随する。
func (s *NotificationService) Run(ctx context.Context) {
	sub := s.settings.Subscribe() // 先に購読して、初回の Get との間の更新を取りこぼさない
	cur, err := s.settings.Get(ctx)
	if err != nil {
		s.log.Warn("loading settings failed; using defaults", "err", err)
		cur = domain.DefaultSettings()
	}
	ticker := s.clock.NewTicker(notifyInterval(cur))
	defer ticker.Stop()

	s.round(ctx, cur)
	for {
		select {
		case <-ctx.Done():
			return
		case cur = <-sub:
			ticker.Reset(notifyInterval(cur))
		case <-ticker.C():
			s.round(ctx, cur)
		}
	}
}

func notifyInterval(st domain.Settings) time.Duration {
	return time.Duration(st.NotifyPollMinutes) * time.Minute
}

// round は全接続を順次処理する（並列にしない。NFR-PERF-6）。
func (s *NotificationService) round(ctx context.Context, st domain.Settings) {
	if s.afterRound != nil {
		defer s.afterRound()
	}
	if !st.NotificationsEnabled {
		return
	}
	conns, err := s.store.Connections(ctx)
	if err != nil {
		s.log.Warn("loading connections failed", "err", err)
		return
	}
	for _, conn := range conns {
		if ctx.Err() != nil {
			return
		}
		if s.conns.NeedsRepair(conn.ID) {
			continue
		}
		s.pollConnection(ctx, conn)
	}
}

// pollConnection は 1 接続の許可セッションを順に取得して Observe へ流す。
// 失敗したセッションは飛ばし（記録は変更しない。NFR-AVL-3）、全許可セッションが成功した一巡の最後に baselined_at を記録する。
// 401/403 は needs_repair を立ててその接続を打ち切る。
func (s *NotificationService) pollConnection(ctx context.Context, conn domain.Connection) {
	all, err := s.report.Sessions(ctx, conn)
	if err != nil {
		s.failed(conn, "", err)
		return
	}
	allowed := allowedSessions(all)
	ids := make([]string, len(allowed))
	for i, sess := range allowed {
		ids[i] = sess.ID
	}
	if err := s.store.PruneSnapshots(ctx, conn.ID, ids); err != nil {
		s.log.Warn("pruning snapshots failed", "connection", conn.Name, "err", err)
	}

	complete := true
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		r, err := s.report.Report(ctx, conn, id)
		if err != nil {
			complete = false
			if s.failed(conn, id, err) {
				return
			}
			continue
		}
		if err := s.store.SaveSnapshot(ctx, conn.ID, id, domain.Snapshot{Report: r, FetchedAt: s.clock.Now()}); err != nil {
			s.log.Warn("saving snapshot failed", "connection", conn.Name, "session", id, "err", err)
		}
		if err := s.Observe(ctx, conn, id, r); err != nil {
			complete = false
			s.log.Warn("observe failed", "connection", conn.Name, "session", id, "err", err)
		}
	}
	if complete && !conn.Baselined() {
		if err := s.store.MarkBaselined(ctx, conn.ID, s.clock.Now()); err != nil {
			s.log.Warn("marking baselined failed", "connection", conn.Name, "err", err)
		}
	}
}

// failed は取得失敗を分類してログに残し、401/403 なら needs_repair を立てて true を返す。
func (s *NotificationService) failed(conn domain.Connection, session string, err error) (repair bool) {
	fe := classify(err)
	if repair = needsRepair(fe); repair {
		s.conns.MarkNeedsRepair(conn.ID)
	}
	s.log.Warn("polling failed", fetchLogAttrs(conn, session, fe)...)
	return repair
}
