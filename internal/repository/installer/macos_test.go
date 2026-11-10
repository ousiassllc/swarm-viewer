package installer

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type zipEntry struct {
	name string
	body string
	mode os.FileMode // 0 は 0o644
}

func writeZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if !mode.IsDir() {
			_, _ = w.Write([]byte(e.body))
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func goodZip(t *testing.T, version string) string {
	t.Helper()
	return writeZip(t, []zipEntry{
		{name: "swarm-viewer.app/", mode: os.ModeDir | 0o755},
		{name: "swarm-viewer.app/Contents/Info.plist", body: "plist " + version},
		{name: "swarm-viewer.app/Contents/MacOS/swarm-viewer", body: "binary " + version, mode: 0o755},
		{name: "swarm-viewer.app/Contents/Resources/icons.icns", body: "icon"},
	})
}

// installedApp は Applications/swarm-viewer.app（旧版）を作り、実行中のバイナリのパスを返す。
func installedApp(t *testing.T) (apps, exe string) {
	t.Helper()
	apps = filepath.Join(t.TempDir(), "Applications")
	macos := filepath.Join(apps, "swarm-viewer.app", "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(macos, "swarm-viewer")
	if err := os.WriteFile(filepath.Join(apps, "swarm-viewer.app", "Contents", "Info.plist"), []byte("plist old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("binary old"), 0o755); err != nil { //nolint:gosec // 実行ファイルの見本
		t.Fatal(err)
	}
	return apps, exe
}

type relaunchCall struct {
	pid          int
	app, cleanup string
}

func macInstaller(exe string, calls *[]relaunchCall) *Installer {
	return &Installer{
		goos:       "darwin",
		executable: func() (string, error) { return exe, nil },
		relaunch: func(pid int, app, cleanup string) error {
			*calls = append(*calls, relaunchCall{pid, app, cleanup})
			return nil
		},
		rename: os.Rename,
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// macOS: 実行中の .app が新しい .app に入れ替わり、開き直しの補助が 1 回起動され、zip が消える。
func TestInstallMacOSSwapsTheApp(t *testing.T) {
	apps, exe := installedApp(t)
	zipFile := goodZip(t, "new")
	var calls []relaunchCall
	if err := macInstaller(exe, &calls).Install(t.Context(), zipFile); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(apps, "swarm-viewer.app")
	if got := readFile(t, filepath.Join(app, "Contents", "Info.plist")); got != "plist new" {
		t.Fatalf("Info.plist = %q", got)
	}
	bin := filepath.Join(app, "Contents", "MacOS", "swarm-viewer")
	if got := readFile(t, bin); got != "binary new" {
		t.Fatalf("binary = %q", got)
	}
	if st, err := os.Stat(bin); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("実行ビットが無い: %v %v", st, err)
	}
	if len(calls) != 1 || calls[0].app != app || calls[0].pid != os.Getpid() {
		t.Fatalf("relaunch = %+v", calls)
	}
	// 旧版は作業用フォルダへ退避されている（補助プロセスが終了後に消す）。
	if got := readFile(t, filepath.Join(calls[0].cleanup, "old.app", "Contents", "MacOS", "swarm-viewer")); got != "binary old" {
		t.Fatalf("old binary = %q", got)
	}
	if filepath.Dir(calls[0].cleanup) != apps {
		t.Fatalf("作業用フォルダが同じボリューム（.app の隣）にない: %s", calls[0].cleanup)
	}
	if _, err := os.Stat(zipFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("zip が残っている: %v", err)
	}
}

// 不正な zip では、実行中の .app に触れず、作業用フォルダも残さず、開き直しも起動しない。
func TestInstallMacOSBadArchiveLeavesTheAppUntouched(t *testing.T) {
	for name, entries := range map[string][]zipEntry{
		"パスの脱出（..）":   {{name: "swarm-viewer.app/../../evil", body: "x"}},
		"絶対パス":        {{name: "/etc/evil", body: "x"}},
		"バックスラッシュ":    {{name: `swarm-viewer.app\Contents\x`, body: "x"}},
		".app でない最上位": {{name: "evil/Contents/MacOS/swarm-viewer", body: "x", mode: 0o755}},
		"最上位が 2 つ": {
			{name: "swarm-viewer.app/Contents/Info.plist", body: "p"},
			{name: "other.app/Contents/Info.plist", body: "p"},
		},
		"実行ファイルが無い":      {{name: "swarm-viewer.app/Contents/Info.plist", body: "p"}},
		"Info.plist が無い": {{name: "swarm-viewer.app/Contents/MacOS/swarm-viewer", body: "b", mode: 0o755}},
		"実行ビットが無い": {
			{name: "swarm-viewer.app/Contents/Info.plist", body: "p"},
			{name: "swarm-viewer.app/Contents/MacOS/swarm-viewer", body: "b", mode: 0o644},
		},
		"シンボリックリンク": {
			{name: "swarm-viewer.app/Contents/Info.plist", body: "p"},
			{name: "swarm-viewer.app/Contents/MacOS/swarm-viewer", body: "b", mode: 0o755},
			{name: "swarm-viewer.app/link", body: "../../..", mode: os.ModeSymlink | 0o777},
		},
		"空": {},
	} {
		t.Run(name, func(t *testing.T) {
			apps, exe := installedApp(t)
			var calls []relaunchCall
			err := macInstaller(exe, &calls).Install(t.Context(), writeZip(t, entries))
			if err == nil {
				t.Fatal("不正な zip で成功した")
			}
			if got := readFile(t, exe); got != "binary old" {
				t.Fatalf("実行中の .app が書き換わった: %q", got)
			}
			if len(calls) != 0 {
				t.Fatalf("開き直しが起動した: %+v", calls)
			}
			ents, _ := os.ReadDir(apps)
			if len(ents) != 1 {
				t.Fatalf("作業用フォルダが残った: %v", ents)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(apps), "evil")); err == nil {
				t.Fatal("展開先の外へ書いた")
			}
		})
	}
}

func TestInstallMacOSNotAZip(t *testing.T) {
	_, exe := installedApp(t)
	bad := filepath.Join(t.TempDir(), "x.zip")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []relaunchCall
	if err := macInstaller(exe, &calls).Install(t.Context(), bad); !errors.Is(err, ErrBadArchive) {
		t.Fatalf("err = %v, want ErrBadArchive", err)
	}
}

// .app の外から動いている（開発ビルド）ときは何もしない。
func TestInstallMacOSNotInABundle(t *testing.T) {
	var calls []relaunchCall
	i := macInstaller(filepath.Join(t.TempDir(), "bin", "swarm-viewer"), &calls)
	if err := i.Install(t.Context(), goodZip(t, "new")); !errors.Is(err, ErrNotBundle) {
		t.Fatalf("err = %v, want ErrNotBundle", err)
	}
	if len(calls) != 0 {
		t.Fatal("開き直しが起動した")
	}
}

// 新しい .app を置けなかったら、旧い .app を元へ戻す。
func TestInstallMacOSRollsBackWhenSwapFails(t *testing.T) {
	apps, exe := installedApp(t)
	var calls []relaunchCall
	i := macInstaller(exe, &calls)
	n := 0
	i.rename = func(oldPath, newPath string) error {
		n++
		if n == 2 { // 新しい .app を置くところで失敗
			return errors.New("disk error")
		}
		return os.Rename(oldPath, newPath)
	}
	if err := i.Install(t.Context(), goodZip(t, "new")); err == nil {
		t.Fatal("差し替えの失敗で成功した")
	}
	if got := readFile(t, exe); got != "binary old" {
		t.Fatalf("旧い .app が戻っていない: %q", got)
	}
	if ents, _ := os.ReadDir(apps); len(ents) != 1 {
		t.Fatalf("作業用フォルダが残った: %v", ents)
	}
	if len(calls) != 0 {
		t.Fatal("開き直しが起動した")
	}
}

// 補助プロセスを起動できなくても差し替えは済んでいるので、エラーとして返す（呼び出し側は終了しない）。
func TestInstallMacOSRelaunchFailureIsReported(t *testing.T) {
	_, exe := installedApp(t)
	boom := errors.New("no sh")
	i := macInstaller(exe, new([]relaunchCall))
	i.relaunch = func(int, string, string) error { return boom }
	if err := i.Install(t.Context(), goodZip(t, "new")); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestBundleOf(t *testing.T) {
	app, bin, err := bundleOf("/Applications/swarm-viewer.app/Contents/MacOS/swarm-viewer")
	if err != nil || app != "/Applications/swarm-viewer.app" || bin != "swarm-viewer" {
		t.Fatalf("bundleOf = %q %q %v", app, bin, err)
	}
	for _, bad := range []string{"/usr/local/bin/swarm-viewer", "/x/Contents/MacOS/swarm-viewer", "/x.app/MacOS/swarm-viewer"} {
		if _, _, err := bundleOf(bad); !errors.Is(err, ErrNotBundle) {
			t.Errorf("bundleOf(%q) err = %v", bad, err)
		}
	}
}
