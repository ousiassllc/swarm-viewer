package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// 新しい版の DB は、WAL への切り替えなども含めて 1 バイトも変更せず ErrSchemaTooNew を返す。
func TestOpenSchemaTooNewLeavesFileUntouched(t *testing.T) {
	path := dbPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// 既定のジャーナルモード（WAL ではない）の DB。Open が WAL へ切り替えるとヘッダが変わる。
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE TABLE future (x); PRAGMA user_version = " + strconv.Itoa(len(ms)+1)); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	before, err := os.ReadFile(path) //nolint:gosec // テストが作った一時ファイルの読み取り
	if err != nil {
		t.Fatal(err)
	}

	got, err := Open(path)
	if !errors.Is(err, ErrSchemaTooNew) || got != nil {
		t.Fatalf("Open = %v, %v; want ErrSchemaTooNew", got, err)
	}
	if errors.Is(err, ErrCorrupt) {
		t.Error("ErrSchemaTooNew must not be ErrCorrupt")
	}
	after, err := os.ReadFile(path) //nolint:gosec // テストが作った一時ファイルの読み取り
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the database file was modified by Open")
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if fi, err := os.Stat(path + suffix); err == nil && fi.Size() > 0 {
			t.Errorf("%s left with content", suffix)
		}
	}
}

func TestOpenTightensLoosePermissionsOfExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows は POSIX の権限ビットを持たない（既定の ACL）")
	}
	path := dbPath(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustSave(t, s, newConn("c1", "http://100.64.0.1:48120/report", newFakeClock().Now()))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // 緩い権限の既存ファイルを再現する
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
	if conns, err := s.Connections(context.Background()); err != nil || len(conns) != 1 {
		t.Errorf("existing data lost: %v, %v", conns, err)
	}
}
