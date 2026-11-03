package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/web/handler"
	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func recoveryApp(reset func(context.Context) error, reason pages.RecoveryReason) http.Handler {
	h := handler.NewRecovery(handler.RecoveryDeps{Logger: discardLogger(), Reason: reason, Reset: reset, CSRFToken: csrfValue})
	e := gin.New()
	e.Use(middleware.CSRF(csrfValue))
	h.Register(e)
	e.NoRoute(h.NotFound)
	return e
}

func recoveryDo(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Header.Set(middleware.CSRFHeaderName, csrfValue)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRecoveryPage(t *testing.T) {
	h := recoveryApp(func(context.Context) error { return nil }, pages.RecoverySchemaTooNew)
	body := wantStatus(t, recoveryDo(h, http.MethodGet, "/", nil), http.StatusOK)
	webtest.MustContain(t, body, "<html", "保存データを開けません", `data-recovery-reason="schema_too_new"`, `hx-post="/recovery/reset"`)
	frag := wantStatus(t, recoveryDo(h, http.MethodGet, "/", map[string]string{"HX-Request": "true"}), http.StatusOK)
	webtest.MustNotContain(t, frag, "<html")
}

func TestRecoveryResetSuccess(t *testing.T) {
	calls := 0
	h := recoveryApp(func(context.Context) error { calls++; return nil }, pages.RecoveryCorrupt)

	w := recoveryDo(h, http.MethodPost, "/recovery/reset", map[string]string{"HX-Request": "true"})
	wantStatus(t, w, http.StatusOK)
	if got := w.Header().Get("HX-Redirect"); got != "/" {
		t.Fatalf("HX-Redirect = %q", got)
	}
	wantRedirect(t, recoveryDo(h, http.MethodPost, "/recovery/reset", nil), http.StatusSeeOther, "/")
	if calls != 2 {
		t.Fatalf("Reset calls = %d", calls)
	}
}

func TestRecoveryResetFailure(t *testing.T) {
	h := recoveryApp(func(context.Context) error { return errors.New("rename /secret/path failed") }, pages.RecoveryCorrupt)
	body := wantStatus(t, recoveryDo(h, http.MethodPost, "/recovery/reset", map[string]string{"HX-Request": "true"}), http.StatusOK)
	webtest.MustContain(t, body, "初期化できませんでした", `hx-post="/recovery/reset"`)
	webtest.MustNotContain(t, body, "/secret/path")
}

func TestRecoveryResetRequiresCSRF(t *testing.T) {
	calls := 0
	h := recoveryApp(func(context.Context) error { calls++; return nil }, pages.RecoveryCorrupt)
	r := httptest.NewRequest(http.MethodPost, "/recovery/reset", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	wantStatus(t, w, http.StatusForbidden)
	if calls != 0 {
		t.Fatal("CSRF なしで Reset を呼んだ")
	}
}

func TestRecoveryOtherRoutes(t *testing.T) {
	h := recoveryApp(func(context.Context) error { return nil }, pages.RecoveryCorrupt)
	wantRedirect(t, recoveryDo(h, http.MethodGet, "/connections", nil), http.StatusFound, "/")
	wantRedirect(t, recoveryDo(h, http.MethodGet, "/settings", nil), http.StatusFound, "/")
	wantStatus(t, recoveryDo(h, http.MethodPost, "/pairing", nil), http.StatusNotFound)
}
