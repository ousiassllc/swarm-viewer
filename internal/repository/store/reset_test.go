package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMoveAsideThenOpenCreatesFreshDB(t *testing.T) {
	path := dbPath(t)
	garbage := strings.Repeat("broken ", 400)
	writeFile(t, path, garbage)
	if _, err := Open(path); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("precondition: Open = %v", err)
	}
	// 開こうとした後に残った WAL 系ファイルを想定して、改めて置く。
	writeFile(t, path+"-wal", "wal")
	writeFile(t, path+"-shm", "shm")

	backup, err := MoveAside(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(backup, path+backupInfix) {
		t.Errorf("backup = %q", backup)
	}
	if b, err := os.ReadFile(backup); err != nil || string(b) != garbage { //nolint:gosec // テストが作った一時ファイルの読み取り
		t.Errorf("backup content lost: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(backup + suffix); err != nil {
			t.Errorf("sidecar %s not moved: %v", suffix, err)
		}
		if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("sidecar %s left behind: %v", suffix, err)
		}
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(backup); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("backup mode: %v, %v", fi, err)
		}
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open after MoveAside: %v", err)
	}
	defer func() { _ = s.Close() }()
	if conns, err := s.Connections(context.Background()); err != nil || len(conns) != 0 {
		t.Errorf("fresh DB must be empty: %v, %v", conns, err)
	}
}

func TestMoveAsideSchemaTooNewKeepsData(t *testing.T) {
	path := dbPath(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustSave(t, s, newConn("c1", urlA, newFakeClock().Now()))
	_, _ = s.db.Exec("PRAGMA user_version = 99")
	_ = s.Close()
	if _, err := Open(path); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("precondition: Open = %v", err)
	}
	backup, err := MoveAside(path)
	if err != nil || backup == "" {
		t.Fatalf("MoveAside = %q, %v", backup, err)
	}
	// 退避した DB は新しい版のアプリが読めるよう、そのまま残る（Open は拒否するが壊れていない）。
	if _, err := Open(backup); !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("backup must be intact: %v", err)
	}
}

func TestMoveAsideMissingFileIsNoop(t *testing.T) {
	backup, err := MoveAside(dbPath(t))
	if err != nil || backup != "" {
		t.Fatalf("= %q, %v; want empty, nil", backup, err)
	}
}

func TestMoveAsideKeepsOnlyLatestBackup(t *testing.T) {
	path := dbPath(t)
	t1 := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

	writeFile(t, path, "one")
	b1, err := moveAside(path, t1)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "two")
	writeFile(t, path+"-wal", "w2")
	b2, err := moveAside(path, t1.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if b1 == b2 {
		t.Fatal("backups must have distinct names")
	}
	if _, err := os.Stat(b1); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("older backup must be removed: %v", err)
	}
	if b, err := os.ReadFile(b2); err != nil || string(b) != "two" { //nolint:gosec // テストが作った一時ファイルの読み取り
		t.Errorf("latest backup: %q, %v", b, err)
	}
	matches, _ := filepath.Glob(path + backupInfix + "*")
	if len(matches) != 2 { // 本体 + -wal
		t.Errorf("backup files = %v", matches)
	}
}
