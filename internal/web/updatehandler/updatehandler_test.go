package updatehandler_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/service/updater"
	"github.com/ousiassllc/swarm-viewer/internal/web/handler"
	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
	"github.com/ousiassllc/swarm-viewer/internal/web/updatehandler"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

const csrf = "csrf-token-not-secret"

func init() { gin.SetMode(gin.TestMode) }

// fakeUpdates は update service の代わり。ApplyAsync の呼び出し回数と、返すエラーを持つ。
type fakeUpdates struct {
	mu      sync.Mutex
	enabled bool
	st      updater.Status
	applies int
	err     error
}

func (f *fakeUpdates) Enabled() bool { return f.enabled }

func (f *fakeUpdates) Status() updater.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.st
}

func (f *fakeUpdates) ApplyAsync() (<-chan struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applies++
	if f.err != nil {
		return nil, f.err
	}
	f.st.Updating = true
	return make(chan struct{}), nil
}

type site struct {
	t   *testing.T
	env *servicetest.Env
	h   http.Handler
}

func newSite(t *testing.T, u updatehandler.Service) *site {
	t.Helper()
	env := servicetest.NewEnv(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := handler.New(handler.Deps{
		Logger: log, Settings: env.Settings, Connections: env.Conns, Snapshots: env.Snap, Pairing: env.Pairing,
		Notifier: env.Notifier, Clock: env.Clock, Updates: u, CSRFToken: csrf,
	})
	e := gin.New()
	e.Use(middleware.Recover(log), middleware.NoStore(), middleware.CSP(), middleware.CSRF(csrf))
	h.Register(e)
	e.NoRoute(h.NotFound)
	return &site{t: t, env: env, h: e}
}

func (s *site) do(method, target string, headers map[string]string) *httptest.ResponseRecorder {
	s.t.Helper()
	r := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

var hx = map[string]string{"HX-Request": "true", middleware.CSRFHeaderName: csrf}

func TestNoticeShowsAvailableVersionWithUpdateButton(t *testing.T) {
	u := &fakeUpdates{enabled: true, st: updater.Status{Available: true, Latest: "v0.1.13"}}
	w := newSite(t, u).do(http.MethodGet, "/update/notice", hx)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	webtest.MustContain(t, body, `id="update-notice"`, "新しい版 v0.1.13 があります", "更新する", `hx-post="/update/apply"`, `hx-target="#update-notice"`, `hx-trigger="every 60s"`)
	webtest.MustNotContain(t, body, "更新中", "script", "onclick")
	if buttonDisabled(t, body) {
		t.Error("更新できるのにボタンが無効")
	}
}

func TestNoticeUpdatingDisablesTheButtonAndShowsProgress(t *testing.T) {
	u := &fakeUpdates{enabled: true, st: updater.Status{Available: true, Latest: "v0.1.13", Updating: true, Done: 25, Total: 100}}
	body := newSite(t, u).do(http.MethodGet, "/update/notice", hx).Body.String()
	webtest.MustContain(t, body, "更新中…", `value="25"`, "25%", `hx-trigger="every 1s"`, "animate-spin")
	webtest.MustNotContain(t, body, `hx-post=`, "があります")
	if !buttonDisabled(t, body) {
		t.Error("更新中なのにボタンが有効")
	}
	// 全体のサイズが分からない間は進み具合を出さない（スピナーと文言だけ）。
	u.st.Total = 0
	body = newSite(t, u).do(http.MethodGet, "/update/notice", hx).Body.String()
	webtest.MustContain(t, body, "更新中…")
	webtest.MustNotContain(t, body, "<progress", "%")
}

// 更新の失敗と確認の失敗は、小さな固定文言だけ。原因の詳細は出さず、通知は残して押し直せる。
func TestNoticeFailuresAreSmallFixedMessages(t *testing.T) {
	for kind, want := range map[updater.FailKind]string{
		updater.FailDownload: "取得できませんでした",
		updater.FailVerify:   "検証できなかった",
		updater.FailInstall:  "更新を始められませんでした",
	} {
		u := &fakeUpdates{enabled: true, st: updater.Status{Available: true, Latest: "v0.1.13", ApplyFailed: kind}}
		body := newSite(t, u).do(http.MethodGet, "/update/notice", hx).Body.String()
		webtest.MustContain(t, body, want, "data-update-error", "新しい版 v0.1.13 があります", `hx-post="/update/apply"`)
		webtest.MustNotContain(t, body, "https://", ".exe", ".zip")
		if buttonDisabled(t, body) {
			t.Error("失敗のあとは押し直せる")
		}
	}
	u := &fakeUpdates{enabled: true, st: updater.Status{CheckFailed: true}}
	body := newSite(t, u).do(http.MethodGet, "/update/notice", hx).Body.String()
	webtest.MustContain(t, body, "新しい版を確認できませんでした", "data-update-check-failed")
	webtest.MustNotContain(t, body, "更新する", "data-alert")
}

// 何も出すものが無い・無効な環境（開発ビルド・Linux・recovery）では、空の領域だけを返す。
func TestNoticeEmptyWhenNothingToShowOrDisabled(t *testing.T) {
	for name, u := range map[string]updatehandler.Service{
		"最新":         &fakeUpdates{enabled: true},
		"無効":         &fakeUpdates{enabled: false, st: updater.Status{Available: true, Latest: "v9.9.9"}},
		"service なし": nil,
	} {
		body := newSite(t, u).do(http.MethodGet, "/update/notice", hx).Body.String()
		webtest.MustNotContain(t, body, "更新する", "新しい版", "data-alert")
		if !strings.Contains(body, `id="update-notice"`) {
			t.Errorf("%s: 領域そのものが無い: %s", name, body)
		}
	}
}

// 「更新する」: 更新を 1 回だけ始めて、更新中の表示を返す。無効な環境では始めない。
func TestApplyStartsTheUpdateOnce(t *testing.T) {
	u := &fakeUpdates{enabled: true, st: updater.Status{Available: true, Latest: "v0.1.13"}}
	s := newSite(t, u)
	w := s.do(http.MethodPost, "/update/apply", hx)
	if w.Code != 200 || u.applies != 1 {
		t.Fatalf("status = %d, applies = %d", w.Code, u.applies)
	}
	webtest.MustContain(t, w.Body.String(), "更新中…")
	if !buttonDisabled(t, w.Body.String()) {
		t.Error("更新中なのにボタンが有効")
	}

	off := &fakeUpdates{enabled: false}
	newSite(t, off).do(http.MethodPost, "/update/apply", hx)
	if off.applies != 0 {
		t.Fatal("無効な環境で更新を始めた")
	}
}

// すでに更新中・新しい版が無いときに押されても、エラーにせず状態を返す。
func TestApplyRefusedStillRendersState(t *testing.T) {
	for _, err := range []error{updater.ErrBusy, updater.ErrNotAvailable} {
		u := &fakeUpdates{enabled: true, err: err, st: updater.Status{Available: true, Latest: "v0.1.13"}}
		w := newSite(t, u).do(http.MethodPost, "/update/apply", hx)
		if w.Code != 200 {
			t.Fatalf("%v: status = %d", err, w.Code)
		}
		webtest.MustContain(t, w.Body.String(), "新しい版 v0.1.13 があります")
	}
}

// CSRF: トークンの無い POST は、更新を始めずに拒否される。
func TestApplyRequiresCSRFToken(t *testing.T) {
	u := &fakeUpdates{enabled: true, st: updater.Status{Available: true, Latest: "v0.1.13"}}
	w := newSite(t, u).do(http.MethodPost, "/update/apply", map[string]string{"HX-Request": "true"})
	if w.Code != http.StatusForbidden || u.applies != 0 {
		t.Fatalf("status = %d, applies = %d", w.Code, u.applies)
	}
}

// メイン画面: 有効なとき通知の領域（load で読み込む空の枠）が付き、無効なときは付かない。通知の取得が失敗しても本体は描画される。
func TestMainScreenHasNoticeSlotOnlyWhenEnabled(t *testing.T) {
	for enabled, u := range map[bool]updatehandler.Service{true: &fakeUpdates{enabled: true}, false: &fakeUpdates{}} {
		s := newSite(t, u)
		s.env.AddConn("c1", "devbox", servicetest.URL1, true)
		s.env.Report.SetSessions("c1", "0")
		body := s.do(http.MethodGet, "/", nil).Body.String()
		webtest.MustContain(t, body, "接続: devbox", `id="snapshot"`)
		has := strings.Contains(body, `id="update-notice"`)
		if has != enabled {
			t.Errorf("enabled=%v: 通知の領域 = %v", enabled, has)
		}
		if enabled {
			webtest.MustContain(t, body, `hx-get="/update/notice"`, `hx-trigger="load, every 60s"`)
		}
	}
}

func TestProps(t *testing.T) {
	p := updatehandler.Props(updater.Status{Available: true, Latest: "v0.1.13", Updating: true, Done: 50, Total: 200})
	if p.Percent != 25 || !p.Updating || p.Latest != "v0.1.13" {
		t.Fatalf("props = %+v", p)
	}
	if p := updatehandler.Props(updater.Status{Available: true, Updating: true, Done: 5}); p.Percent >= 0 {
		t.Fatalf("全体が不明なのに進み具合がある: %+v", p)
	}
}

// buttonDisabled は最初の <button> の disabled 属性の有無（クラスの disabled: 接頭辞とは区別する）。
func buttonDisabled(t *testing.T, html string) bool {
	t.Helper()
	btns := webtest.Elements(t, html, "button")
	if len(btns) == 0 {
		t.Fatalf("button が無い: %s", html)
	}
	_, ok := webtest.Attr(btns[0], "disabled")
	return ok
}
