package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// MaxNotifiedPerConnection は接続あたりの通知済み記録の上限件数（NFR-DAT-3）。
const MaxNotifiedPerConnection = 1000

// NotifiedKeys は接続×セッションの通知済み記録をペイン ID・状態の順で返す。
func (s *Store) NotifiedKeys(ctx context.Context, connID, session string) ([]domain.NotifiedKey, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT pane_id, state FROM notified_states
WHERE connection_id = ? AND session_id = ?
ORDER BY pane_id, state`, connID, session)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.NotifiedKey
	for rows.Next() {
		var k domain.NotifiedKey
		var state string
		if err := rows.Scan(&k.PaneID, &state); err != nil {
			return nil, err
		}
		k.State = domain.PaneState(state)
		out = append(out, k)
	}
	return out, rows.Err()
}

// ApplyNotified は Diff の Delete・Insert を 1 トランザクションで適用する。
// INSERT は OR IGNORE で、実際に挿入できたキーだけを返す（主キー制約が二重通知の最終防衛線）。
// 適用後、接続あたりの件数が MaxNotifiedPerConnection を超えた分を古い順に削除する。
// 接続が無ければ ErrNotFound。
func (s *Store) ApplyNotified(ctx context.Context, connID, session string, d domain.Diff) ([]domain.NotifiedKey, error) {
	tx, err := s.db.BeginTx(ctx, nil) // 書き込みロックを先に取るため、同時の呼び出しは直列化される
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM connections WHERE id = ?", connID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	for _, k := range d.Delete {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM notified_states
WHERE connection_id = ? AND session_id = ? AND pane_id = ? AND state = ?`,
			connID, session, k.PaneID, string(k.State)); err != nil {
			return nil, err
		}
	}

	now := formatTime(s.now())
	var inserted []domain.NotifiedKey
	for _, k := range d.Insert {
		res, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO notified_states (connection_id, session_id, pane_id, state, notified_at)
VALUES (?, ?, ?, ?, ?)`, connID, session, k.PaneID, string(k.State), now)
		if err != nil {
			return nil, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n == 1 {
			inserted = append(inserted, k)
		}
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM notified_states WHERE rowid IN (
    SELECT rowid FROM notified_states WHERE connection_id = ?
    ORDER BY notified_at DESC, rowid DESC
    LIMIT -1 OFFSET ?
)`, connID, MaxNotifiedPerConnection); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inserted, nil
}
