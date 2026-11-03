package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// 版ごとの SQL。ファイル名は `<版>_<名前>.sql`（版は 1 から連番）。
//
//go:embed migrations/*.sql
var migrationFS embed.FS

type migration struct {
	version int
	sql     string
}

// loadMigrations は埋め込み SQL を版の昇順で返す。版が連番でなければエラー（ビルド時の取り違え検出）。
func loadMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	ms := make([]migration, 0, len(entries))
	for _, e := range entries {
		prefix, _, _ := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("store: bad migration name %q", e.Name())
		}
		body, err := migrationFS.ReadFile(path.Join("migrations", e.Name()))
		if err != nil {
			return nil, err
		}
		ms = append(ms, migration{version: v, sql: string(body)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	for i, m := range ms {
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration versions must be sequential from 1 (got %d at index %d)", m.version, i)
		}
	}
	return ms, nil
}

// userVersion は PRAGMA user_version を返す。
func userVersion(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (int, error) {
	var v int
	if err := q.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// checkSchemaNotNewer は DB の版がこのアプリの知る最新より新しければ ErrSchemaTooNew を返す（DB は変更しない）。
func checkSchemaNotNewer(ctx context.Context, db *sql.DB) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := userVersion(ctx, db)
	if err != nil {
		return err
	}
	if current > len(ms) {
		return ErrSchemaTooNew
	}
	return nil
}

// migrate は user_version より新しい版を順に適用する。各版は 1 つのトランザクションで、
// 最後に user_version を更新する（途中で失敗すれば版ごと巻き戻る）。
// DB の版がアプリの知る最新より新しければ何も変更せず ErrSchemaTooNew を返す。
func migrate(ctx context.Context, db *sql.DB) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := userVersion(ctx, db)
	if err != nil {
		return err
	}
	if current > len(ms) {
		return ErrSchemaTooNew
	}
	for _, m := range ms[current:] {
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("store: apply migration %d: %w", m.version, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil) // _txlock=immediate: 書き込みロックを先に取る
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 別プロセスが先に適用した場合に二重適用しないよう、ロックを取った後で版を読み直す。
	cur, err := userVersion(ctx, tx)
	if err != nil {
		return err
	}
	if cur >= m.version {
		return nil
	}
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return err
	}
	// PRAGMA は引数を束縛できない。m.version は埋め込みファイル名由来の整数。
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(m.version)); err != nil {
		return err
	}
	return tx.Commit()
}
