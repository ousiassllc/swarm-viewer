package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func TestSettingsSaveRoundTrip(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()

	want := domain.Settings{NotificationsEnabled: false, ScreenPollSeconds: 60, NotifyPollMinutes: 5, DebugLog: true}
	clock.Advance(time.Hour)
	if err := s.SaveSettings(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Settings(ctx)
	if err != nil || got != want {
		t.Fatalf("Settings = %+v, %v; want %+v", got, err, want)
	}
	var updated string
	if err := s.db.QueryRow("SELECT updated_at FROM settings WHERE id = 1").Scan(&updated); err != nil || updated != formatTime(clock.Now()) {
		t.Errorf("updated_at = %q, %v", updated, err)
	}
	if n := count(t, s, "SELECT count(*) FROM settings"); n != 1 {
		t.Errorf("settings rows = %d, want 1 (SaveSettings must UPDATE)", n)
	}
}

// 新しい版の自動確認（#109）: 既定はオンで、保存して読み戻せる。版 1 の DB は列が足され、既存の値を保ったままオンになる。
func TestSettingsAutoUpdateCheck(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)
	for _, on := range []bool{true, false, true} {
		st := domain.DefaultSettings()
		st.AutoUpdateCheck = on
		if err := s.SaveSettings(ctx, st); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Settings(ctx); err != nil || got.AutoUpdateCheck != on {
			t.Fatalf("AutoUpdateCheck = %v, %v; want %v", got.AutoUpdateCheck, err, on)
		}
	}
	path := filepath.Join(t.TempDir(), "v1.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := migrationFS.ReadFile("migrations/001_init.sql")
	if _, err := raw.Exec(string(v1) + "; PRAGMA user_version = 1; UPDATE settings SET debug_log = 1"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	old, err := open(path, newFakeClock().Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Close() }()
	if got, err := old.Settings(ctx); err != nil || !got.AutoUpdateCheck || !got.DebugLog {
		t.Fatalf("Settings = %+v, %v", got, err)
	}
}

func TestSaveSettingsRejectsInvalid(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	for name, mut := range map[string]func(*domain.Settings){
		"screen 7":  func(st *domain.Settings) { st.ScreenPollSeconds = 7 },
		"screen 0":  func(st *domain.Settings) { st.ScreenPollSeconds = 0 },
		"notify 15": func(st *domain.Settings) { st.NotifyPollMinutes = 15 },
		"notify -5": func(st *domain.Settings) { st.NotifyPollMinutes = -5 },
	} {
		st := domain.DefaultSettings()
		mut(&st)
		err := s.SaveSettings(ctx, st)
		var se *domain.SettingsError
		if !errors.As(err, &se) {
			t.Errorf("%s: err = %v; want *domain.SettingsError", name, err)
		}
	}
	got, err := s.Settings(ctx)
	if err != nil || got != domain.DefaultSettings() {
		t.Errorf("rejected input must not change settings: %+v, %v", got, err)
	}
}

// アプリ側の検証をすり抜けても DB の CHECK が拒否する（二重の防御）。
func TestSettingsCheckConstraints(t *testing.T) {
	s, _ := newTestStore(t)
	for _, stmt := range []string{
		"UPDATE settings SET screen_poll_seconds = 7",
		"UPDATE settings SET notify_poll_minutes = 15",
		"UPDATE settings SET notifications_enabled = 2",
		"UPDATE settings SET debug_log = -1",
		"UPDATE settings SET id = 2",
		"INSERT INTO settings (id, updated_at) VALUES (2, 'x')",
	} {
		if _, err := s.db.Exec(stmt); err == nil {
			t.Errorf("%q must violate a CHECK constraint", stmt)
		}
	}
	for _, v := range []int{5, 10, 30, 60} {
		if _, err := s.db.Exec("UPDATE settings SET screen_poll_seconds = ?, notify_poll_minutes = ?", v, v); err != nil {
			t.Errorf("allowed value %d rejected: %v", v, err)
		}
	}
}

func TestSetLastConnection(t *testing.T) {
	s, clock := newTestStore(t)
	ctx := context.Background()
	mustSave(t, s, newConn("c1", urlA, clock.Now()))

	if err := s.SetLastConnection(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if st, err := s.Settings(ctx); err != nil || st.LastConnectionID != "c1" {
		t.Fatalf("Settings = %+v, %v", st, err)
	}

	// SaveSettings は last_connection_id を巻き戻さない。
	st := domain.DefaultSettings()
	st.LastConnectionID = "ignored"
	if err := s.SaveSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Settings(ctx); got.LastConnectionID != "c1" {
		t.Errorf("SaveSettings changed LastConnectionID to %q", got.LastConnectionID)
	}

	if err := s.SetLastConnection(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetLastConnection(missing) = %v; want ErrNotFound", err)
	}
	if err := s.SetLastConnection(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Settings(ctx); got.LastConnectionID != "" {
		t.Errorf("empty id must clear: %q", got.LastConnectionID)
	}
	if n := count(t, s, "SELECT count(*) FROM settings WHERE last_connection_id IS NULL"); n != 1 {
		t.Error("cleared value must be NULL")
	}
}

func TestSettingsMissingRow(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	if _, err := s.db.Exec("DELETE FROM settings"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Settings(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("Settings = %v; want ErrNotFound", err)
	}
	if err := s.SaveSettings(ctx, domain.DefaultSettings()); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveSettings = %v; want ErrNotFound (no INSERT)", err)
	}
}
