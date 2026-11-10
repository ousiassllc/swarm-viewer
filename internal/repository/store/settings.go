package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Settings は設定（id=1 の 1 行）を返す。行が無ければ ErrNotFound（マイグレーションが必ず作る）。
func (s *Store) Settings(ctx context.Context) (domain.Settings, error) {
	var (
		st                   domain.Settings
		notifications, debug int
		autoUpdate           int
		last                 sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
SELECT notifications_enabled, screen_poll_seconds, notify_poll_minutes, debug_log, auto_update_check, last_connection_id
FROM settings WHERE id = 1`).Scan(
		&notifications, &st.ScreenPollSeconds, &st.NotifyPollMinutes, &debug, &autoUpdate, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Settings{}, ErrNotFound
	}
	if err != nil {
		return domain.Settings{}, err
	}
	st.NotificationsEnabled = notifications == 1
	st.DebugLog = debug == 1
	st.AutoUpdateCheck = autoUpdate == 1
	st.LastConnectionID = last.String
	return st, nil
}

// SaveSettings は設定を保存する（id=1 を UPDATE。INSERT はしない）。
// 許容外の値は *domain.SettingsError（DB の CHECK と二重に守る）。
// st.LastConnectionID は書き込まない。最後に使った接続は SetLastConnection だけが更新する
// （設定画面の更新が、並行する接続の切替を巻き戻さないため）。
func (s *Store) SaveSettings(ctx context.Context, st domain.Settings) error {
	if err := st.Validate(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE settings SET
    notifications_enabled = ?,
    screen_poll_seconds = ?,
    notify_poll_minutes = ?,
    debug_log = ?,
    auto_update_check = ?,
    updated_at = ?
WHERE id = 1`,
		boolInt(st.NotificationsEnabled), st.ScreenPollSeconds, st.NotifyPollMinutes,
		boolInt(st.DebugLog), boolInt(st.AutoUpdateCheck), formatTime(s.now()))
	return requireAffected(res, err)
}

// SetLastConnection は起動時に開く接続を記録する（空文字は NULL）。
// 存在しない接続 id は ErrNotFound。
func (s *Store) SetLastConnection(ctx context.Context, id string) error {
	var v any
	if id != "" {
		v = id
	}
	res, err := s.db.ExecContext(ctx,
		"UPDATE settings SET last_connection_id = ?, updated_at = ? WHERE id = 1",
		v, formatTime(s.now()))
	return requireAffected(res, err)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
