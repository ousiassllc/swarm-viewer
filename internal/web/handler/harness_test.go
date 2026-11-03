package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/handler"
	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
)

const csrfValue = "csrf-token-not-secret"

func init() { gin.SetMode(gin.TestMode) }

// app はフェイクの service を載せた画面ルート一式（本番と同じミドルウェア）。
type app struct {
	t   *testing.T
	env *servicetest.Env
	h   http.Handler
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func newApp(t *testing.T) *app {
	t.Helper()
	env := servicetest.NewEnv(t)
	h := handler.New(handler.Deps{
		Logger:      discardLogger(),
		Settings:    env.Settings,
		Connections: env.Conns,
		Snapshots:   env.Snap,
		Pairing:     env.Pairing,
		Notifier:    env.Notifier,
		Clock:       env.Clock,
		CSRFToken:   csrfValue,
	})
	e := gin.New()
	e.Use(middleware.Recover(discardLogger()), middleware.NoStore(), middleware.CSP(), middleware.CSRF(csrfValue))
	h.Register(e)
	e.NoRoute(h.NotFound)
	return &app{t: t, env: env, h: e}
}

type reqOpt func(*http.Request)

// hx は HTMX からのリクエストにする。
func hx() reqOpt { return func(r *http.Request) { r.Header.Set("HX-Request", "true") } }

// form はフォーム本文を付ける。
func form(v url.Values) reqOpt {
	return func(r *http.Request) {
		r.Body = io.NopCloser(strings.NewReader(v.Encode()))
		r.ContentLength = int64(len(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func (a *app) do(method, target string, opts ...reqOpt) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.Header.Set(middleware.CSRFHeaderName, csrfValue)
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

func (a *app) get(target string, opts ...reqOpt) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.do(http.MethodGet, target, opts...)
}

// wantStatus は応答のステータスを確認して本文を返す。
func wantStatus(t *testing.T, w *httptest.ResponseRecorder, status int) string {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d\nbody: %s", w.Code, status, w.Body.String())
	}
	return w.Body.String()
}

// wantHXRedirect は 200 と HX-Redirect だけで移動を指示していること（3xx・Location を使わない）を確認する。
// WebKit の WebView は http.Redirect を扱えない。
func wantHXRedirect(t *testing.T, w *httptest.ResponseRecorder, target string) {
	t.Helper()
	wantStatus(t, w, http.StatusOK)
	if got := w.Header().Get("HX-Redirect"); got != target {
		t.Fatalf("HX-Redirect = %q, want %q", got, target)
	}
	if got := w.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want none", got)
	}
}

func isFullPage(body string) bool { return strings.Contains(body, "<html") }
