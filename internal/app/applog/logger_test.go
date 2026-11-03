package applog

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func TestRotatorConfig(t *testing.T) {
	dir := t.TempDir()
	r := NewRotator(dir)
	// NFR-LOG-1: 5 MB × 3 世代（現行 1 + 退避 2）。
	if r.MaxSize != 5 {
		t.Errorf("MaxSize = %d, want 5", r.MaxSize)
	}
	if r.MaxBackups != 2 {
		t.Errorf("MaxBackups = %d, want 2 (current + 2 backups = 3 generations)", r.MaxBackups)
	}
	if r.MaxAge != 0 {
		t.Errorf("MaxAge = %d, want 0 (generation count only)", r.MaxAge)
	}
	if r.Compress {
		t.Error("Compress = true, want false")
	}
	if want := filepath.Join(dir, "swarm-viewer.log"); r.Filename != want {
		t.Errorf("Filename = %q, want %q", r.Filename, want)
	}
}

func TestOpenCreatesPrivateLogFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	l.Logger.Warn("hello", "token", "test-token-not-secret")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "swarm-viewer.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"msg":"hello"`) || strings.Contains(string(data), "test-token-not-secret") {
		t.Fatalf("unexpected log content: %s", data)
	}
	if runtime.GOOS == "windows" {
		return // POSIX 権限は Windows では検査しない
	}
	for _, p := range []string{dir, path} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode = %v, want no group/other access", p, fi.Mode().Perm())
		}
	}
}

func TestRotationKeepsAtMostThreeFiles(t *testing.T) {
	dir := t.TempDir()
	r := NewRotator(dir)
	r.MaxSize = 1 // MB。設定値は別のテストで固定しているので、ここは回転の動作だけを短い入力で確認する
	line := bytes.Repeat([]byte("x"), 1024)
	line = append(line, '\n')
	for range 4 * 1100 { // 約 4.3 MB → 1 MB 上限で 4 回ほど回転する
		if _, err := r.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	// lumberjack の古いファイル削除は goroutine で行われるので、残りが上限に収まるまで待つ（上限付き）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, err := filepath.Glob(filepath.Join(dir, "swarm-viewer*.log"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) <= 3 {
			if len(files) < 2 {
				t.Fatalf("expected rotation to create backups, files = %v", files)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("files = %d, want <= 3: %v", len(files), files)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDefaultLevelIsWarn(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.Logger.Info("quiet")
	l.Logger.Debug("quiet")
	l.Logger.Warn("loud")
	out := buf.String()
	if strings.Contains(out, "quiet") || !strings.Contains(out, "loud") {
		t.Fatalf("default level should be warn and above only: %s", out)
	}
}

func TestLevelFor(t *testing.T) {
	if LevelFor(false) != slog.LevelWarn || LevelFor(true) != slog.LevelDebug {
		t.Fatalf("LevelFor: off=%v on=%v", LevelFor(false), LevelFor(true))
	}
}

func TestFollowTogglesDebugAtRuntime(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	changes := make(chan domain.Settings)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Follow(ctx, changes); close(done) }()

	changes <- domain.Settings{DebugLog: true}
	// 無バッファ channel の送信が受理された後に Set が走る。レベルが反映されるまで上限付きで待つ。
	waitLevel(t, l, slog.LevelDebug)
	l.Logger.Debug("now-visible")
	if !strings.Contains(buf.String(), "now-visible") {
		t.Fatalf("debug log expected after enabling: %q", buf.String())
	}

	changes <- domain.Settings{DebugLog: false}
	waitLevel(t, l, slog.LevelWarn)
	l.Logger.Debug("hidden-again")
	if strings.Contains(buf.String(), "hidden-again") {
		t.Fatalf("debug log must be off after disabling: %q", buf.String())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Follow did not stop on cancel")
	}
}

func TestFollowStopsWhenChannelCloses(t *testing.T) {
	l := New(&bytes.Buffer{})
	changes := make(chan domain.Settings)
	done := make(chan struct{})
	go func() { l.Follow(context.Background(), changes); close(done) }()
	close(changes)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Follow did not stop when the channel closed")
	}
}

func waitLevel(t *testing.T, l *Logging, want slog.Level) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for l.Level.Level() != want {
		if time.Now().After(deadline) {
			t.Fatalf("level = %v, want %v", l.Level.Level(), want)
		}
		time.Sleep(time.Millisecond)
	}
}
