package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallUnsupportedOS(t *testing.T) {
	i := &Installer{goos: "linux"}
	if err := i.Install(t.Context(), "x"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestInstallCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	i := &Installer{goos: "windows", launchWindows: func(string) error { called = true; return nil }}
	if err := i.Install(ctx, "x"); err == nil || called {
		t.Fatalf("err = %v, called = %v", err, called)
	}
}

// Windows: 検証済みのインストーラを 1 回だけ起動する。
func TestInstallWindowsLaunchesTheFileOnce(t *testing.T) {
	file := filepath.Join(t.TempDir(), "swarm-viewer_v0.1.13_windows_amd64_installer.exe")
	if err := os.WriteFile(file, []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}
	var launched []string
	i := &Installer{goos: "windows", launchWindows: func(f string) error { launched = append(launched, f); return nil }}
	if err := i.Install(t.Context(), file); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 1 || launched[0] != file {
		t.Fatalf("launched = %v", launched)
	}
}

func TestInstallWindowsFailures(t *testing.T) {
	i := &Installer{goos: "windows", launchWindows: func(string) error { t.Fatal("起動してはいけない"); return nil }}
	if err := i.Install(t.Context(), filepath.Join(t.TempDir(), "missing.exe")); err == nil {
		t.Fatal("無いファイルで成功した")
	}
	if err := i.Install(t.Context(), t.TempDir()); err == nil {
		t.Fatal("ディレクトリで成功した")
	}
	file := filepath.Join(t.TempDir(), "i.exe")
	if err := os.WriteFile(file, []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("uac declined")
	i = &Installer{goos: "windows", launchWindows: func(string) error { return boom }}
	if err := i.Install(t.Context(), file); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want launch error", err)
	}
}
