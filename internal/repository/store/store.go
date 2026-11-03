// Package store は SQLite（modernc.org/sqlite、CGO 不要）への永続化を担う（docs/architecture/er.md）。
//
// token は扱わない（秘密領域は repository/secret）。ログも出さない。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// 失敗の種別。service/app は errors.Is で判定し、起動を止めず初期化導線を出す（NFR-AVL-5・CMP-4）。
var (
	// ErrCorrupt は DB を開けない・壊れている・マイグレーションに失敗したとき。
	ErrCorrupt = errors.New("store: database cannot be opened or is corrupt")
	// ErrSchemaTooNew は DB の user_version がこのアプリの知る版より新しいとき。DB は変更しない。
	ErrSchemaTooNew = errors.New("store: database schema is newer than this application supports")
	// ErrNotFound は対象の行（接続・設定）が無いとき。errors.Is(err, sql.ErrNoRows) も真になる。
	ErrNotFound = fmt.Errorf("store: not found: %w", sql.ErrNoRows)
)

const (
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600

	busyTimeoutMillis = 5000
	dbFileName        = "viewer.db"
	appDirName        = "swarm-viewer"
)

// Store は SQLite の読み書きを持つ。複数 goroutine から使ってよい。
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// DefaultPath は既定の DB パス（os.UserConfigDir()/swarm-viewer/viewer.db）を返す。
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDirName, dbFileName), nil
}

// Open は path の DB を開く。無ければ 0600 で作成する（-wal・-shm も可能な範囲で 0600）。
// PRAGMA を設定し、整合性を確認して、未適用のマイグレーションを実行する。
// 開けない・壊れている場合は ErrCorrupt、新しい版の DB は ErrSchemaTooNew を返す
// （どちらも errors.Is で判定。呼び出し側はストアなしで続行する）。
func Open(path string) (*Store, error) {
	return open(path, time.Now)
}

func open(path string, now func() time.Time) (*Store, error) {
	if err := ensureFile(path); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	dsn, err := dsnFor(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if err := prepare(context.Background(), db); err != nil {
		_ = db.Close()
		if errors.Is(err, ErrSchemaTooNew) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	restrictSidecars(path)
	return &Store{db: db, now: now}, nil
}

// prepare は接続の確認・整合性チェック・マイグレーションを行う。
func prepare(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil { // PRAGMA は接続ごとに DSN で適用される
		return err
	}
	// 版は WAL への切り替えや整合性チェックより先に読む。新しい版の DB は一切変更せず、壊れているとも誤報しない。
	if err := checkSchemaNotNewer(ctx, db); err != nil {
		return err
	}
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check(1)").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return errors.New("store: integrity check failed")
	}
	// WAL への切り替えはファイルを書き換えるため、版と整合性の確認が済んでから行う（以後の接続は DB ファイルの設定を引き継ぐ）。
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode(WAL)").Scan(&mode); err != nil {
		return err
	}
	return migrate(ctx, db)
}

// ensureFile は DB ファイルとその親ディレクトリを（無ければ）所有者のみの権限で作る。
// 既存ファイルの権限が 0600 より緩ければ 0600 に締める（Windows を除く。ペイン本文を含むため）。
func ensureFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, filePerm) //nolint:gosec // 呼び出し元が決めた DB パス
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if runtime.GOOS == "windows" {
		return nil
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&^filePerm != 0 {
		return f.Chmod(filePerm)
	}
	return nil
}

// restrictSidecars は WAL の -wal・-shm を 0600 にする（SQLite は通常 DB ファイルの権限を引き継ぐ）。
// 失敗しても致命ではないため無視する（Windows では実質何もしない）。
func restrictSidecars(path string) {
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Chmod(path+suffix, filePerm)
	}
}

// dsnFor は PRAGMA を付けた file: URI を作る。パス中の ? や # は URL エスケープで安全に渡す。
func dsnFor(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") { // Windows のドライブレター: /C:/...
		p = "/" + p
	}
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	q.Add("_pragma", "foreign_keys(1)")
	// 書き込みトランザクションは開始時にロックを取る（読み→書きの昇格による SQLITE_BUSY を避ける）。
	q.Set("_txlock", "immediate")
	u := url.URL{Scheme: "file", Path: p, RawQuery: q.Encode()}
	return u.String(), nil
}

// Close は DB を閉じる。
func (s *Store) Close() error {
	return s.db.Close()
}

// formatTime は UTC の RFC 3339 文字列（例 2026-11-03T09:10:05Z）にする。
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// mapFK は外部キー違反（親の接続が無い）を ErrNotFound に写す。
func mapFK(err error) error {
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		return ErrNotFound
	}
	return err
}
