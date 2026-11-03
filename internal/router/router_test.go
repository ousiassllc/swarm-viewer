package router_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/router"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/static"
)

func fakeRouter(t *testing.T) (http.Handler, *servicetest.Env) {
	t.Helper()
	env := servicetest.NewEnv(t)
	env.AddConn("c1", "devbox", servicetest.URL1, true)
	env.Report.SetSessions("c1", "0", "1")
	if err := env.Secrets.Set("c1", servicetest.Token); err != nil {
		t.Fatal(err)
	}
	h, err := router.New(router.Deps{
		Logger: slog.New(slog.DiscardHandler), Settings: env.Settings, Connections: env.Conns,
		Snapshots: env.Snap, Pairing: env.Pairing, Notifier: env.Notifier, Clock: env.Clock, Static: static.FS(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, env
}

func serve(h http.Handler, method, target, csrf string, form url.Values) *httptest.ResponseRecorder {
	var r *http.Request
	if form != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Header.Set("HX-Request", "true")
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// 状態変更ルートは CSRF なしで 403。副作用も起きない。
func TestStateChangingRoutesRequireCSRF(t *testing.T) {
	h, env := fakeRouter(t)
	routes := []struct {
		method, target string
		form           url.Values
	}{
		{http.MethodPost, "/pairing", url.Values{"pairing": {servicetest.PairingStr}}},
		{http.MethodPost, "/pairing/drafts/anything/confirm", nil},
		{http.MethodPost, "/pairing/drafts/anything/discard", nil},
		{http.MethodDelete, "/connections/c1", nil},
		{http.MethodPut, "/settings", url.Values{"screen_poll_seconds": {"30"}, "notify_poll_minutes": {"5"}}},
		{http.MethodPost, "/static/css/app.css", nil},
		{http.MethodPost, "/nowhere", nil},
	}
	for _, rt := range routes {
		for _, csrf := range []string{"", "wrong-token"} {
			w := serve(h, rt.method, rt.target, csrf, rt.form)
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s (csrf=%q): status = %d, want 403", rt.method, rt.target, csrf, w.Code)
			}
			if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s %s: セキュリティヘッダが無い", rt.method, rt.target)
			}
		}
	}
	if _, ok := env.Store.Conn("c1"); !ok {
		t.Fatal("CSRF なしの DELETE で接続が消えた")
	}
}

// 全ルート（エラー応答・404 含む）に CSP と no-store が付き、token は本文に出ない。
func TestEveryResponseHasHeadersAndNoToken(t *testing.T) {
	h, env := fakeRouter(t)
	env.Report.SetReport("c1", "0", servicetest.ReportOf(servicetest.Pane("%1", "T1", "working")))

	targets := []string{
		"/", "/pairing", "/connections", "/connections/c1", "/connections/c1/sessions/0",
		"/connections/c1/sessions/0/snapshot", "/connections/c1/sessions/zzz", "/connections/nope",
		"/pairing/drafts/unknown", "/settings", "/static/js/app.js", "/static/missing", "/nowhere",
		"/api/v1/openapi.json", "/recovery/reset",
	}
	for _, target := range targets {
		for _, hxHdr := range []bool{false, true} {
			r := httptest.NewRequest(http.MethodGet, target, nil)
			if hxHdr {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") {
				t.Errorf("GET %s: CSP が無い", target)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("GET %s: Cache-Control = %q", target, w.Header().Get("Cache-Control"))
			}
			if strings.Contains(w.Body.String(), servicetest.Token) {
				t.Errorf("GET %s: 本文に token が含まれている", target)
			}
		}
	}
}

func TestUnknownRoutes(t *testing.T) {
	h, _ := fakeRouter(t)
	w := serve(h, http.MethodGet, "/nowhere", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	// 通常モードに recovery のルートは無い（CSRF 前に弾かれず 404 になるのは GET のみ）。
	if w := serve(h, http.MethodGet, "/recovery/reset", "", nil); w.Code != http.StatusNotFound {
		t.Fatalf("GET /recovery/reset = %d", w.Code)
	}
}

func TestOpenAPIHasNoOperations(t *testing.T) {
	h, _ := fakeRouter(t)
	w := serve(h, http.MethodGet, "/api/v1/openapi.json", "", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "json") {
		t.Fatalf("status = %d, type = %q", w.Code, w.Header().Get("Content-Type"))
	}
	if strings.Contains(w.Body.String(), `"get"`) || strings.Contains(w.Body.String(), `"post"`) {
		t.Fatalf("オペレーションが登録されている: %s", w.Body.String())
	}
	// Huma の docs UI・スキーマ配信は無効。
	for _, p := range []string{"/docs", "/schemas/x.json"} {
		if w := serve(h, http.MethodGet, p, "", nil); w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d", p, w.Code)
		}
	}
}

func TestRecoveryRouter(t *testing.T) {
	calls := 0
	h, err := router.NewRecovery(router.RecoveryDeps{
		Logger: slog.New(slog.DiscardHandler),
		Reason: router.RecoverySchemaTooNew,
		Reset:  func(context.Context) error { calls++; return nil },
		Static: static.FS(),
	})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `data-recovery-reason="schema_too_new"`) ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src") {
		t.Fatalf("status = %d\n%s", w.Code, body)
	}
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("CSRF トークンが配られていない")
	}
	token := m[1] + m[2]

	if w := serve(h, http.MethodPost, "/recovery/reset", "", nil); w.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("CSRF なし: status = %d calls = %d", w.Code, calls)
	}
	w = serve(h, http.MethodPost, "/recovery/reset", token, nil)
	if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "/" || calls != 1 {
		t.Fatalf("status = %d redirect = %q calls = %d", w.Code, w.Header().Get("HX-Redirect"), calls)
	}
	// 他のページは（リダイレクトせず）初期化導線をその場に描画する。静的ファイルは配る。
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/connections", nil))
	if w.Code != http.StatusOK || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), `data-recovery-reason="schema_too_new"`) {
		t.Fatalf("GET /connections = %d %q", w.Code, w.Header().Get("Location"))
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/css/app.css", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /static/css/app.css = %d", w.Code)
	}
}

func TestRecoveryRouterRejectsMissingReset(t *testing.T) {
	if _, err := router.NewRecovery(router.RecoveryDeps{Logger: slog.New(slog.DiscardHandler), Static: static.FS()}); err == nil {
		t.Fatal("Reset が無いのにエラーにならない")
	}
	_, err := router.NewRecovery(router.RecoveryDeps{
		Logger: slog.New(slog.DiscardHandler), Reset: func(context.Context) error { return errors.New("x") },
	})
	if err == nil {
		t.Fatal("Static が無いのにエラーにならない")
	}
}
