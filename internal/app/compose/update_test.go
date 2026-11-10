package compose

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/swarm-viewer/internal/app/applog"
	"github.com/ousiassllc/swarm-viewer/internal/app/swap"
	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

const (
	updWinAsset = "swarm-viewer_v0.1.13_windows_amd64_installer.exe"
	updPayload  = "installer-bytes"
)

// updSource は GitHub の代わり。最新の版と、アセットの本文を返す。
type updSource struct{ checks atomic.Int32 }

func (s *updSource) Latest(context.Context) (update.Release, error) {
	s.checks.Add(1)
	return update.Release{Tag: "v0.1.13", Version: update.Version{Major: 0, Minor: 1, Patch: 13}, Assets: []update.Asset{
		{Name: updWinAsset, URL: "https://github.com/x/" + updWinAsset},
		{Name: update.SumsName, URL: "https://github.com/x/SHA256SUMS"},
	}}, nil
}

func (s *updSource) Download(_ context.Context, a update.Asset, dst io.Writer) error {
	body := updPayload
	if a.Name == update.SumsName {
		h := sha256.Sum256([]byte(updPayload))
		body = hex.EncodeToString(h[:]) + "  " + updWinAsset + "\n"
	}
	_, err := io.Copy(dst, bytes.NewReader([]byte(body)))
	return err
}

type updInstaller struct{ files chan string }

func (i *updInstaller) Install(_ context.Context, file string) error {
	i.files <- file
	return nil
}

func newUpdateEnv(t *testing.T, version string) (*env, *updSource, *updInstaller, chan struct{}) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir()) // 取得したファイルを、このテストのフォルダへ
	src, inst, quit := &updSource{}, &updInstaller{files: make(chan string, 4)}, make(chan struct{}, 4)
	e := &env{handler: &swap.Handler{}, dbPath: filepath.Join(t.TempDir(), "viewer.db"), clock: servicetest.NewClock(), logBuf: &bytes.Buffer{}}
	e.logging = applog.New(e.logBuf)
	rec := &servicetest.Recorder{}
	e.rt = New(Config{
		DBPath: e.dbPath, Logging: e.logging, Notifier: &servicetest.Notifier{}, Clock: e.clock, IDs: &servicetest.IDs{},
		Secrets: servicetest.NewSecrets(rec), Report: servicetest.NewReport(rec),
		Static:  fstest.MapFS{"js/app.js": {Data: []byte("//")}},
		Handler: e.handler,
		Version: version, GOOS: "windows", Releases: src, Installer: inst, Quit: func() { quit <- struct{}{} },
	})
	t.Cleanup(e.rt.Close)
	return e, src, inst, quit
}

// 組み立ての通し: 起動して準備ができると新しい版を確認し、通知が出て、「更新する」で取得・検証してインストーラを 1 回起動し、終了する。
func TestUpdateEndToEnd(t *testing.T) {
	e, src, inst, quit := newUpdateEnv(t, "0.1.9")
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.rt.Ready()
	waitFor(t, "新しい版の通知", func() bool {
		return strings.Contains(e.send(http.MethodGet, "/update/notice", e.csrf(t, "/settings"), nil).Body.String(), "新しい版 v0.1.13 があります")
	})
	token := e.csrf(t, "/settings")
	if body := e.send(http.MethodPost, "/update/apply", token, nil).Body.String(); !strings.Contains(body, "更新中") {
		t.Fatalf("更新中の表示でない: %s", body)
	}
	file := <-inst.files
	if !strings.HasSuffix(file, updWinAsset) {
		t.Fatalf("installer file = %s", file)
	}
	<-quit
	if len(inst.files) != 0 || len(quit) != 0 {
		t.Fatal("インストーラの起動・終了が複数回")
	}
	if src.checks.Load() < 1 {
		t.Fatal("確認していない")
	}
}

// 開発ビルド（版が dev）では、準備ができても GitHub を確認せず、通知の領域も空のまま。
func TestUpdateDisabledForDevBuild(t *testing.T) {
	e, src, _, _ := newUpdateEnv(t, "dev")
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.rt.Ready()
	token := e.csrf(t, "/settings")
	e.rt.Close() // バックグラウンドの処理（確認）が走り終わるまで待つ
	if n := src.checks.Load(); n != 0 {
		t.Fatalf("dev ビルドで %d 回確認した", n)
	}
	body := e.send(http.MethodGet, "/update/notice", token, nil).Body.String()
	if strings.Contains(body, "新しい版") {
		t.Fatalf("dev ビルドで通知が出た: %s", body)
	}
}
