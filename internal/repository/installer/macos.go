package installer

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// maxExtractBytes は zip を展開した合計の上限（zip 爆弾への備え。.app は数十 MB）。
	maxExtractBytes = 1 << 30
	// maxExtractEntries は zip の項目数の上限。
	maxExtractEntries = 20000
)

// macOS の更新の失敗。
var (
	// ErrNotBundle は実行中のバイナリが .app の中にない（開発ビルドなど）。差し替える対象が無い。
	ErrNotBundle = errors.New("installer: the running binary is not inside a .app bundle")
	// ErrBadArchive は zip が .app を 1 つ含む形でない、または危険なパスを含む。
	ErrBadArchive = errors.New("installer: the update archive is not a single .app bundle")
)

// bundleOf は実行中のバイナリ（.../X.app/Contents/MacOS/<bin>）から .app のパスとバイナリ名を返す。
func bundleOf(exe string) (app, bin string, err error) {
	dir := filepath.Dir(exe)
	if filepath.Base(dir) != "MacOS" || filepath.Base(filepath.Dir(dir)) != "Contents" {
		return "", "", ErrNotBundle
	}
	app = filepath.Dir(filepath.Dir(dir))
	if !strings.HasSuffix(app, ".app") {
		return "", "", ErrNotBundle
	}
	return app, filepath.Base(exe), nil
}

// installMacOS は zipFile の .app で実行中の .app を差し替え、アプリの終了後に開き直す補助を起動する。
//
// 作業用フォルダは実行中の .app と同じフォルダの中に作る（rename が同じボリュームで原子的に済む。書き込めなければ何も変えずに失敗）。
// 差し替えは「実行中の .app → 作業用フォルダの old.app」「展開した .app → 元の場所」の 2 回の rename で、2 回目が失敗したら元へ戻す。
// 成功したら zip を消す。差し替え前の .app と作業用フォルダは、補助プロセスがアプリの終了後に消す。
func (i *Installer) installMacOS(zipFile string) error {
	exe, err := i.executable()
	if err != nil {
		return fmt.Errorf("installer: locating the running app: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = resolved
	}
	app, bin, err := bundleOf(exe)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(app), ".swarm-viewer-update-")
	if err != nil {
		return fmt.Errorf("installer: creating a staging folder next to the app (is the folder writable?): %w", err)
	}
	swapped := false
	defer func() {
		if !swapped {
			_ = os.RemoveAll(staging)
		}
	}()

	extracted := filepath.Join(staging, "new")
	newApp, err := extractApp(zipFile, extracted, bin)
	if err != nil {
		return err
	}
	oldApp := filepath.Join(staging, "old.app")
	if err := i.rename(app, oldApp); err != nil {
		return fmt.Errorf("installer: moving the running app aside: %w", err)
	}
	if err := i.rename(newApp, app); err != nil {
		if rerr := i.rename(oldApp, app); rerr != nil {
			// 戻せなかった。作業用フォルダを残して（old.app が入っている）、手で戻せるようにする。
			swapped = true
			return fmt.Errorf("installer: installing the new app failed and the old app could not be restored (kept at %s): %w", oldApp, errors.Join(err, rerr))
		}
		return fmt.Errorf("installer: installing the new app: %w", err)
	}
	swapped = true // 差し替え後の staging（old.app を含む）は補助プロセスが消す
	if err := i.relaunch(os.Getpid(), app, staging); err != nil {
		// 差し替えは済んでいる。開き直しだけ手動になる（呼び出し側はエラーとして扱い、アプリは終了しない）。
		return fmt.Errorf("installer: the app was updated but could not be relaunched automatically: %w", err)
	}
	_ = os.Remove(zipFile)
	return nil
}

// extractApp は zip を dest へ安全に展開し、中の .app のパスを返す。
// 最上位は .app のフォルダ 1 つだけ（`__MACOSX/` は読み飛ばす）で、Contents/Info.plist と Contents/MacOS/<bin> が無ければ ErrBadArchive。
// 絶対パス・`..`・シンボリックリンク（リンク越しの書き込みを防ぐ）は拒否する。
func extractApp(zipFile, dest, bin string) (string, error) {
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrBadArchive, err)
	}
	defer func() { _ = zr.Close() }()
	if len(zr.File) == 0 || len(zr.File) > maxExtractEntries {
		return "", ErrBadArchive
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	var top string
	var total int64
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "__MACOSX") {
			continue
		}
		first, _, _ := strings.Cut(name, "/")
		if !strings.HasSuffix(first, ".app") || (top != "" && first != top) || !filepath.IsLocal(filepath.FromSlash(name)) || strings.Contains(f.Name, `\`) {
			return "", ErrBadArchive
		}
		top = first
		n, err := extractEntry(f, dest, name)
		if err != nil {
			return "", err
		}
		if total += n; total > maxExtractBytes {
			return "", ErrBadArchive
		}
	}
	if top == "" {
		return "", ErrBadArchive
	}
	app := filepath.Join(dest, top)
	for _, must := range []string{filepath.Join("Contents", "Info.plist"), filepath.Join("Contents", "MacOS", bin)} {
		if st, err := os.Stat(filepath.Join(app, must)); err != nil || !st.Mode().IsRegular() {
			return "", ErrBadArchive
		}
	}
	if st, err := os.Stat(filepath.Join(app, "Contents", "MacOS", bin)); err != nil || st.Mode().Perm()&0o100 == 0 {
		return "", ErrBadArchive // 実行できない
	}
	return app, nil
}

// extractEntry は zip の 1 項目を dest の下へ書く。書いたバイト数を返す。
func extractEntry(f *zip.File, dest, name string) (int64, error) {
	target := filepath.Join(dest, filepath.FromSlash(name))
	mode := f.Mode()
	switch {
	case mode.IsDir():
		return 0, os.MkdirAll(target, 0o755) //nolint:gosec // .app のフォルダは他のユーザーも読めるのが標準
	case !mode.IsRegular():
		// シンボリックリンク・デバイス・FIFO は拒否する。リンクを許すと、リンク越しの書き込みで展開先の外へ出られる
		// （swarm-viewer の .app はシンボリックリンクを含まない）。
		return 0, ErrBadArchive
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // 同上
		return 0, err
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = rc.Close() }()
	perm := mode.Perm() | 0o600
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // 展開先は検証済みの dest の下。実行ビットは zip の記録を引き継ぐ
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(out, io.LimitReader(rc, maxExtractBytes+1))
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return n, err
	}
	// umask で落ちた実行ビットを戻す。
	if err := os.Chmod(target, perm); err != nil {
		return n, err
	}
	return n, nil
}
