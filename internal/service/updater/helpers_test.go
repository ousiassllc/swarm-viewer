package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

const (
	testTag     = "v0.1.13"
	winAsset    = "swarm-viewer_v0.1.13_windows_amd64_installer.exe"
	macAsset    = "swarm-viewer_v0.1.13_darwin_universal.zip"
	payloadBody = "installer-bytes"
)

func sumOf(b string) string {
	h := sha256.Sum256([]byte(b))
	return hex.EncodeToString(h[:])
}

// fakeSource は GitHub の代わり。Latest・Download の回数と、渡されたアセット名を覚える。
type fakeSource struct {
	mu        sync.Mutex
	latest    update.Release
	latestErr error
	files     map[string]fileResult // アセット名 → 本文・エラー
	latestHit chan struct{}
	calls     int
	downloads []string
}

type fileResult struct {
	body string
	// partial は本文の途中まで書いてから err を返す（途中で切れた取得）。
	partial bool
	err     error
}

func newFakeSource() *fakeSource {
	return &fakeSource{files: map[string]fileResult{}, latestHit: make(chan struct{}, 64)}
}

func (f *fakeSource) Latest(context.Context) (update.Release, error) {
	f.mu.Lock()
	f.calls++
	rel, err := f.latest, f.latestErr
	f.mu.Unlock()
	f.latestHit <- struct{}{}
	return rel, err
}

func (f *fakeSource) Download(_ context.Context, a update.Asset, dst io.Writer) error {
	f.mu.Lock()
	f.downloads = append(f.downloads, a.Name)
	r, ok := f.files[a.Name]
	f.mu.Unlock()
	if !ok {
		return errors.New("404")
	}
	body := r.body
	if r.partial {
		body = body[:len(body)/2]
	}
	if _, err := io.Copy(dst, bytes.NewReader([]byte(body))); err != nil {
		return err
	}
	return r.err
}

func (f *fakeSource) latestCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSource) downloaded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.downloads...)
}

// relOf は tag のリリース（アセット 2 つ：更新ファイルと SHA256SUMS）。
func relOf(tag string, assets ...string) update.Release {
	r := update.Release{Tag: tag}
	for _, n := range assets {
		r.Assets = append(r.Assets, update.Asset{Name: n, URL: "https://github.com/x/" + n, Size: 0})
	}
	return r
}

// withFiles はアセットの本文と、それに合う SHA256SUMS を fakeSource へ入れる。
func (f *fakeSource) withFiles(assetName, body string) {
	f.files[assetName] = fileResult{body: body}
	f.files[update.SumsName] = fileResult{body: sumOf(body) + "  " + assetName + "\n"}
}

// fakeInstaller は適用の代わり。呼ばれたときのファイルの内容と回数を覚える。
type fakeInstaller struct {
	mu     sync.Mutex
	files  []string
	bodies []string
	err    error
}

func (f *fakeInstaller) Install(_ context.Context, file string) error {
	b, rerr := os.ReadFile(file) //nolint:gosec // テストが作った一時フォルダの下のパス
	f.mu.Lock()
	f.files = append(f.files, file)
	f.bodies = append(f.bodies, string(b))
	err := f.err
	f.mu.Unlock()
	if rerr != nil {
		return fmt.Errorf("installer got an unreadable file: %w", rerr)
	}
	return err
}

func (f *fakeInstaller) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.files...)
}

// fakeSettings は設定。Subscribe の通知は Push で流す。
type fakeSettings struct {
	mu  sync.Mutex
	st  domain.Settings
	sub chan domain.Settings
	err error
}

func newFakeSettings(auto bool) *fakeSettings {
	st := domain.DefaultSettings()
	st.AutoUpdateCheck = auto
	return &fakeSettings{st: st, sub: make(chan domain.Settings, 1)}
}

func (f *fakeSettings) Get(context.Context) (domain.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.st, f.err
}

func (f *fakeSettings) Subscribe() <-chan domain.Settings { return f.sub }

func (f *fakeSettings) Push(auto bool) {
	f.mu.Lock()
	f.st.AutoUpdateCheck = auto
	st := f.st
	f.mu.Unlock()
	f.sub <- st
}

type env struct {
	src      *fakeSource
	inst     *fakeInstaller
	settings *fakeSettings
	clock    *servicetest.Clock
	quits    chan struct{}
	root     string
	svc      *Service
}

// newEnv は現在の版 current（"0.1.9" など）・OS goos の Service を作る。Quit は quits へ送る。
func newEnv(t *testing.T, current, goos string, auto bool) *env {
	t.Helper()
	e := &env{
		src: newFakeSource(), inst: &fakeInstaller{}, settings: newFakeSettings(auto),
		clock: servicetest.NewClock(), quits: make(chan struct{}, 8), root: t.TempDir(),
	}
	e.svc = New(Options{
		Source: e.src, Installer: e.inst, Settings: e.settings, Clock: e.clock,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: current, GOOS: goos,
		Quit:     func() { e.quits <- struct{}{} },
		TempRoot: e.root,
	})
	return e
}

// leftovers は更新用に作られて残っているフォルダの数。
func (e *env) leftovers(t *testing.T) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(e.root, "swarm-viewer-update-*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

// runInBackground は Run を別 goroutine で動かし、止める関数（終了まで待つ）を返す。
func (e *env) runInBackground(t *testing.T) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); e.svc.Run(ctx) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run が止まらない")
		}
	}
}
