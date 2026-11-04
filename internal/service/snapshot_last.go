package service

import (
	"context"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Last は取得せずに、直近の値（メモリと snapshots のうち新しいほう。Stale: true）を返す。無ければ ok=false。
// メイン画面の初回描画で前回の状態をグレーアウトして先に見せる（FR-POLL-4）ために使う。
func (s *SnapshotService) Last(ctx context.Context, connID, session string) (sn domain.Snapshot, ok bool) {
	res := s.failed(ctx, connID, session, nil)
	return res.Snapshot, !res.Snapshot.FetchedAt.IsZero()
}
