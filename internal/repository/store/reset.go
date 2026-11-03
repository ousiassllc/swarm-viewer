package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const backupInfix = ".bak-"

// MoveAside は開けない・新しすぎる DB を初期化するための退避。
// path とその -wal・-shm を `<path>.bak-<UTC 時刻>` へ改名する（新しい版のアプリで DB を読み直せるよう、
// 削除はしない。権限 0600 は改名で保たれる）。直前の退避（`<path>.bak-*`）は新しい退避で置き換え、
// 平文のペイン本文が積み上がらないよう最新の 1 世代だけを残す。
// 退避後は Open(path) が空の DB を新規作成する（秘密領域の token は孤立エントリとして残る。ER §5）。
// path が無ければ何もせず空文字を返す。Store を開いたまま呼ばないこと（Windows では改名に失敗する）。
func MoveAside(path string) (backup string, err error) {
	return moveAside(path, time.Now())
}

func moveAside(path string, now time.Time) (string, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	if old, err := filepath.Glob(escapeGlob(path) + backupInfix + "*"); err == nil {
		for _, f := range old {
			if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("store: remove old backup: %w", err)
			}
		}
	}

	backup := path + backupInfix + now.UTC().Format("20060102T150405Z")
	if err := os.Rename(path, backup); err != nil {
		return "", fmt.Errorf("store: move database aside: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(path+suffix, backup+suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("store: move database sidecar aside: %w", err)
		}
	}
	return backup, nil
}

// escapeGlob は filepath.Glob のメタ文字をパス中でエスケープする。
func escapeGlob(p string) string {
	var out []rune
	for _, r := range p {
		switch r {
		case '*', '?', '[', '\\':
			if filepath.Separator != '\\' { // Windows ではバックスラッシュが区切りのためエスケープできない
				out = append(out, '\\')
			}
		}
		out = append(out, r)
	}
	return string(out)
}
