package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

const connectionColumns = "id, name, url, last_session, baselined_at, created_at, updated_at"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanConnection(r rowScanner) (domain.Connection, error) {
	var (
		c                    domain.Connection
		last, baselined      sql.NullString
		createdAt, updatedAt string
		err                  error
	)
	if err = r.Scan(&c.ID, &c.Name, &c.URL, &last, &baselined, &createdAt, &updatedAt); err != nil {
		return domain.Connection{}, err
	}
	c.LastSession = last.String
	if baselined.Valid {
		if c.BaselinedAt, err = parseTime(baselined.String); err != nil {
			return domain.Connection{}, fmt.Errorf("store: invalid baselined_at: %w", err)
		}
	}
	if c.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Connection{}, fmt.Errorf("store: invalid created_at: %w", err)
	}
	if c.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Connection{}, fmt.Errorf("store: invalid updated_at: %w", err)
	}
	return c, nil
}

// Connections は保存済みの接続を作成時刻の昇順で返す。
func (s *Store) Connections(ctx context.Context) ([]domain.Connection, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+connectionColumns+" FROM connections ORDER BY created_at ASC, rowid ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ConnectionByID は id の接続を返す。無ければ ErrNotFound。
func (s *Store) ConnectionByID(ctx context.Context, id string) (domain.Connection, error) {
	c, err := scanConnection(s.db.QueryRowContext(ctx,
		"SELECT "+connectionColumns+" FROM connections WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Connection{}, ErrNotFound
	}
	return c, err
}

// ConnectionByURL は url の接続を返す。無ければ ok=false（エラーではない）。
func (s *Store) ConnectionByURL(ctx context.Context, url string) (domain.Connection, bool, error) {
	c, err := scanConnection(s.db.QueryRowContext(ctx,
		"SELECT "+connectionColumns+" FROM connections WHERE url = ?", url))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Connection{}, false, nil
	}
	if err != nil {
		return domain.Connection{}, false, err
	}
	return c, true, nil
}

// SaveConnection は接続を保存する（url で一意の UPSERT）。
// 同じ url の行があれば、その行の id・last_session・baselined_at・created_at を保ったまま
// name と updated_at だけを置き換える（再ペアリング。ER §3.1）。無ければ c をそのまま挿入する。
func (s *Store) SaveConnection(ctx context.Context, c domain.Connection) error {
	var last, baselined any
	if c.LastSession != "" {
		last = c.LastSession
	}
	if !c.BaselinedAt.IsZero() {
		baselined = formatTime(c.BaselinedAt)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO connections (`+connectionColumns+`)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (url) DO UPDATE SET
    name = excluded.name,
    updated_at = excluded.updated_at`,
		c.ID, c.Name, c.URL, last, baselined, formatTime(c.CreatedAt), formatTime(c.UpdatedAt))
	return err
}

// DeleteConnection は接続を削除する。snapshots・notified_states は連鎖削除され、
// settings.last_connection_id は NULL に戻る。無い id は何もしない（nil）。
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM connections WHERE id = ?", id)
	return err
}

// SetLastSession は最後に選んだセッションを記録する（空文字は NULL）。接続が無ければ ErrNotFound。
func (s *Store) SetLastSession(ctx context.Context, id, session string) error {
	var v any
	if session != "" {
		v = session
	}
	res, err := s.db.ExecContext(ctx,
		"UPDATE connections SET last_session = ?, updated_at = ? WHERE id = ?",
		v, formatTime(s.now()), id)
	return requireAffected(res, err)
}

// MarkBaselined は初回の取得パスの完了時刻を記録する。すでに記録済みなら最初の時刻を保つ。
// 接続が無ければ ErrNotFound。
func (s *Store) MarkBaselined(ctx context.Context, id string, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE connections SET baselined_at = COALESCE(baselined_at, ?) WHERE id = ?",
		formatTime(at), id)
	return requireAffected(res, err)
}

func requireAffected(res sql.Result, err error) error {
	if err != nil {
		return mapFK(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
