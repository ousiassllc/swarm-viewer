package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func TestOpenMigratesEmptyDB(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	v, err := userVersion(ctx, s.db)
	if err != nil || v != len(ms) {
		t.Fatalf("user_version = %d, %v; want %d", v, err, len(ms))
	}
	for _, table := range []string{"connections", "settings", "snapshots", "notified_states"} {
		if n := count(t, s, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table); n != 1 {
			t.Errorf("table %s missing", table)
		}
	}
	if n := count(t, s, "SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_notified_states_connection_notified_at'"); n != 1 {
		t.Error("notified_states index missing")
	}
	st, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := domain.DefaultSettings(); st != want {
		t.Errorf("initial settings = %+v, want %+v", st, want)
	}
}

func TestOpenPragmas(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	// 接続プールの別接続でも PRAGMA が効いていることを確認するため、固定した接続を複数使う。
	var conns []*sql.Conn
	for range 3 {
		c, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	t.Cleanup(func() {
		for _, c := range conns {
			_ = c.Close()
		}
	})
	for i, c := range conns {
		var fk, busy int
		var mode string
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
			t.Errorf("conn %d foreign_keys = %d, %v", i, fk, err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil || busy != busyTimeoutMillis {
			t.Errorf("conn %d busy_timeout = %d, %v", i, busy, err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Errorf("conn %d journal_mode = %q, %v", i, mode, err)
		}
	}
}

func TestReopenIsIdempotent(t *testing.T) {
	path := dbPath(t)
	ctx := context.Background()
	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustSave(t, s1, newConn("c1", "http://100.64.0.1:48120/report", newFakeClock().Now()))
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = s2.Close() }()
	conns, err := s2.Connections(ctx)
	if err != nil || len(conns) != 1 || conns[0].ID != "c1" {
		t.Fatalf("data lost after reopen: %+v, %v", conns, err)
	}
	if n := count(t, s2, "SELECT count(*) FROM settings"); n != 1 {
		t.Errorf("settings rows = %d, want 1 (migration must not re-run)", n)
	}
}

func TestOpenSchemaTooNew(t *testing.T) {
	path := dbPath(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustSave(t, s, newConn("c1", "http://100.64.0.1:48120/report", newFakeClock().Now()))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	got, err := Open(path)
	if !errors.Is(err, ErrSchemaTooNew) || got != nil {
		t.Fatalf("Open = %v, %v; want ErrSchemaTooNew", got, err)
	}
	if errors.Is(err, ErrCorrupt) {
		t.Error("ErrSchemaTooNew must not be ErrCorrupt")
	}

	// データを壊していない: 版もデータもそのまま。
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	var v, n int
	if err := raw.QueryRow("PRAGMA user_version").Scan(&v); err != nil || v != 99 {
		t.Errorf("user_version = %d, %v; want 99", v, err)
	}
	if err := raw.QueryRow("SELECT count(*) FROM connections").Scan(&n); err != nil || n != 1 {
		t.Errorf("connections = %d, %v; want 1", n, err)
	}
}

func TestOpenCorruptFile(t *testing.T) {
	path := dbPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	garbage := strings.Repeat("this is not a sqlite database. ", 200)
	if err := os.WriteFile(path, []byte(garbage), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if !errors.Is(err, ErrCorrupt) || s != nil {
		t.Fatalf("Open = %v, %v; want ErrCorrupt", s, err)
	}
	// 壊れたファイルは上書きしない（初期化は利用者の操作で MoveAside する）。
	if b, err := os.ReadFile(path); err != nil || string(b) != garbage { //nolint:gosec // テストが作った一時ファイルの読み取り
		t.Errorf("corrupt file was modified: %v", err)
	}
}

func TestOpenUnopenablePath(t *testing.T) {
	dir := t.TempDir()
	// 親がファイル → ディレクトリを作れない。
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(blocker, "viewer.db")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Open under file = %v; want ErrCorrupt", err)
	}
	// パス自体がディレクトリ。
	if _, err := Open(dir); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Open directory = %v; want ErrCorrupt", err)
	}
}

func TestOpenMigrationFailureOnForeignDB(t *testing.T) {
	path := dbPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE TABLE connections (x INTEGER)"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	if _, err := Open(path); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Open foreign DB = %v; want ErrCorrupt", err)
	}
}

func TestOpenFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows は POSIX の権限ビットを持たない（既定の ACL）")
	}
	path := dbPath(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	mustSave(t, s, newConn("c1", "http://100.64.0.1:48120/report", newFakeClock().Now()))

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("stat %s: %v", filepath.Base(p), err)
			continue
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s mode = %o, want 600", filepath.Base(p), perm)
		}
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("dir mode = %o, want no group/other access", perm)
	}
}

func TestOpenPathWithSpecialCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a b#c%d?e")
	s, err := Open(filepath.Join(dir, "viewer.db"))
	if err != nil {
		t.Skipf("このファイルシステムは特殊文字のパスを許さない: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := os.Stat(filepath.Join(dir, "viewer.db")); err != nil {
		t.Fatalf("DB must be created at the requested path: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Skipf("UserConfigDir unavailable: %v", err)
	}
	if filepath.Base(p) != "viewer.db" || filepath.Base(filepath.Dir(p)) != "swarm-viewer" {
		t.Errorf("DefaultPath = %q", p)
	}
}
