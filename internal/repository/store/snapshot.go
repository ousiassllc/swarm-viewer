package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// SnapshotSchemaVersion は snapshots.report_json の形式版（domain.Report の JSON）。
// 読めない版・壊れた JSON は「スナップショットなし」として扱う（ER §3.3）。
const SnapshotSchemaVersion = 1

// Snapshot は接続×セッションの最後に保存した report を返す。
// 無い・版が合わない・JSON が壊れているときは ok=false（エラーではない。起動を止めない）。
// 返す Snapshot の Stale は常に false（再表示かどうかは呼び出し側が決める）。
func (s *Store) Snapshot(ctx context.Context, connID, session string) (domain.Snapshot, bool, error) {
	var (
		fetchedAt string
		version   int
		raw       string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT fetched_at, schema_version, report_json FROM snapshots
WHERE connection_id = ? AND session_id = ?`, connID, session).Scan(&fetchedAt, &version, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Snapshot{}, false, nil
	}
	if err != nil {
		return domain.Snapshot{}, false, err
	}
	if version != SnapshotSchemaVersion {
		return domain.Snapshot{}, false, nil
	}
	at, err := parseTime(fetchedAt)
	if err != nil {
		return domain.Snapshot{}, false, nil //nolint:nilerr // 壊れた行は「スナップショットなし」として扱う（ER §3.3）
	}
	var report domain.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return domain.Snapshot{}, false, nil //nolint:nilerr // 壊れた行は「スナップショットなし」として扱う（ER §3.3）
	}
	return domain.Snapshot{Report: report, FetchedAt: at}, true, nil
}

// SaveSnapshot は接続×セッションの最新 1 件を UPSERT する（履歴は持たない）。
// 保存するのは domain.Report の項目だけ（未知のキーは保存しない）。接続が無ければ ErrNotFound。
func (s *Store) SaveSnapshot(ctx context.Context, connID, session string, sn domain.Snapshot) error {
	raw, err := json.Marshal(sn.Report)
	if err != nil {
		return fmt.Errorf("store: encode snapshot: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO snapshots (connection_id, session_id, fetched_at, schema_version, report_json)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (connection_id, session_id) DO UPDATE SET
    fetched_at = excluded.fetched_at,
    schema_version = excluded.schema_version,
    report_json = excluded.report_json`,
		connID, session, formatTime(sn.FetchedAt), SnapshotSchemaVersion, string(raw))
	return mapFK(err)
}

// PruneSnapshots は keep に無いセッションのスナップショットを接続から削除する
// （sessions_list から消えたセッション分。keep が空なら全削除）。
func (s *Store) PruneSnapshots(ctx context.Context, connID string, keep []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	keepSet := make(map[string]struct{}, len(keep))
	for _, id := range keep {
		keepSet[id] = struct{}{}
	}
	rows, err := tx.QueryContext(ctx, "SELECT session_id FROM snapshots WHERE connection_id = ?", connID)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		if _, ok := keepSet[id]; !ok {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM snapshots WHERE connection_id = ? AND session_id = ?", connID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
