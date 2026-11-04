package middleware_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
)

const testToken = "csrf-test-token-not-secret" //nolint:gosec // テスト用の偽の値

func init() { gin.SetMode(gin.TestMode) }

func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(mw...)
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	r.GET("/x", ok)
	r.HEAD("/x", ok)
	r.OPTIONS("/x", ok)
	r.POST("/x", ok)
	r.PUT("/x", ok)
	r.DELETE("/x", ok)
	r.PATCH("/x", ok)
	return r
}

func do(r http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set(middleware.CSRFHeaderName, token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCSRF_StateChangingMethods(t *testing.T) {
	t.Parallel()
	r := newEngine(middleware.CSRF(testToken))
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if w := do(r, m, "/x", ""); w.Code != http.StatusForbidden {
			t.Errorf("%s トークンなし: %d, want 403", m, w.Code)
		}
		if w := do(r, m, "/x", "wrong"); w.Code != http.StatusForbidden {
			t.Errorf("%s 誤ったトークン: %d, want 403", m, w.Code)
		}
		if w := do(r, m, "/x", testToken+"x"); w.Code != http.StatusForbidden {
			t.Errorf("%s 長いトークン: %d, want 403", m, w.Code)
		}
		if w := do(r, m, "/x", testToken[:len(testToken)-1]); w.Code != http.StatusForbidden {
			t.Errorf("%s 短いトークン: %d, want 403", m, w.Code)
		}
		if w := do(r, m, "/x", testToken); w.Code != http.StatusOK {
			t.Errorf("%s 正しいトークン: %d, want 200", m, w.Code)
		}
	}
}

func TestCSRF_SafeMethodsPassWithoutToken(t *testing.T) {
	t.Parallel()
	r := newEngine(middleware.CSRF(testToken))
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if w := do(r, m, "/x", ""); w.Code != http.StatusOK {
			t.Errorf("%s: %d, want 200", m, w.Code)
		}
	}
}

func TestCSRF_RejectionIsGenericFragmentAndAborts(t *testing.T) {
	t.Parallel()
	reached := false
	r := gin.New()
	r.Use(middleware.CSRF(testToken))
	r.POST("/x", func(c *gin.Context) { reached = true })
	w := do(r, http.MethodPost, "/x", "wrong-token-value")
	if reached {
		t.Fatal("拒否したのにハンドラが実行された")
	}
	body := w.Body.String()
	if !strings.Contains(body, `role="alert"`) || strings.Contains(body, testToken) || strings.Contains(body, "wrong-token-value") {
		t.Errorf("本文: %q", body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestCSRF_EmptyConfiguredTokenRejectsEverything(t *testing.T) {
	t.Parallel()
	r := newEngine(middleware.CSRF(""))
	if w := do(r, http.MethodPost, "/x", ""); w.Code != http.StatusForbidden {
		t.Errorf("空トークン設定で通った: %d", w.Code)
	}
}

func TestNewCSRFToken(t *testing.T) {
	t.Parallel()
	a, err := middleware.NewCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := middleware.NewCSRFToken()
	if len(a) < 43 || a == b {
		t.Errorf("トークンが短い・重複: %q %q", a, b)
	}
	if strings.ContainsAny(a, "+/= ") {
		t.Errorf("base64url でない: %q", a)
	}
}

func TestCSP_ExactHeader(t *testing.T) {
	t.Parallel()
	w := do(newEngine(middleware.CSP()), http.MethodGet, "/x", "")
	const want = "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'"
	if got := w.Header().Get("Content-Security-Policy"); got != want {
		t.Errorf("CSP = %q", got)
	}
	if middleware.ContentSecurityPolicy != want || strings.Contains(want, "unsafe") {
		t.Error("定数が仕様と異なる")
	}
}

func TestNoStore(t *testing.T) {
	t.Parallel()
	r := newEngine(middleware.NoStore())
	for _, m := range []string{http.MethodGet, http.MethodPost} {
		if got := do(r, m, "/x", "").Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q", m, got)
		}
	}
}

// CSRF が 403 を返す応答にも CSP・NoStore が付く（順序を問わず先に付けておけば効く）。
func TestHeadersApplyToRejectedResponses(t *testing.T) {
	t.Parallel()
	r := newEngine(middleware.NoStore(), middleware.CSP(), middleware.CSRF(testToken))
	w := do(r, http.MethodPost, "/x", "")
	if w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Security-Policy") == "" {
		t.Errorf("code=%d headers=%v", w.Code, w.Header())
	}
}

func TestRecover_GenericFragmentWithoutPanicText(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	r := gin.New()
	r.Use(middleware.Recover(logger))
	r.GET("/boom", func(*gin.Context) { panic("secret-token-in-panic-value") })
	w := do(r, http.MethodGet, "/boom", "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-token-in-panic-value") || strings.Contains(body, "goroutine") || strings.Contains(body, "panic") {
		t.Errorf("本文に panic の内容が出ている: %q", body)
	}
	if !strings.Contains(body, "内部エラーが発生しました") || !strings.Contains(body, `role="alert"`) {
		t.Errorf("汎用フラグメントでない: %q", body)
	}
	out := logs.String()
	if !strings.Contains(out, "panic recovered") || !strings.Contains(out, "panic_type=string") || !strings.Contains(out, "stack=") {
		t.Errorf("ログにスタックと型が無い: %q", out)
	}
	if strings.Contains(out, "secret-token-in-panic-value") {
		t.Errorf("panic 値がログに出ている: %q", out)
	}
	// panic のあともサーバーは次のリクエストを処理できる。
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	if w := do(r, http.MethodGet, "/ok", ""); w.Code != http.StatusOK {
		t.Errorf("後続のリクエスト: %d", w.Code)
	}
}

func TestRecover_AfterResponseStartedDoesNotAppend(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.Use(middleware.Recover(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	r.GET("/half", func(c *gin.Context) {
		c.String(http.StatusOK, "partial")
		panic("later")
	})
	w := do(r, http.MethodGet, "/half", "")
	if w.Code != http.StatusOK || w.Body.String() != "partial" {
		t.Errorf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestRecover_ErrAbortHandlerPropagates(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.Use(middleware.Recover(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	r.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
	defer func() {
		if rec := recover(); !errors.Is(asError(rec), http.ErrAbortHandler) {
			t.Errorf("recover = %v, want http.ErrAbortHandler", rec)
		}
	}()
	do(r, http.MethodGet, "/abort", "")
	t.Error("panic が伝播しなかった")
}

func TestRequestLog_NoQueryBodyOrHeaders(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := gin.New()
	r.Use(middleware.RequestLog(logger))
	r.POST("/connections/:cid", func(c *gin.Context) { c.String(http.StatusCreated, "ok") })

	req := httptest.NewRequest(http.MethodPost, "/connections/01ABC?token=test-token-not-secret&x=1", strings.NewReader("pairing=swarm-viewer://pair?token=test-token-not-secret"))
	req.Header.Set("Authorization", "Bearer test-token-not-secret")
	req.Header.Set(middleware.CSRFHeaderName, testToken)
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := logs.String()
	for _, banned := range []string{"token", "test-token-not-secret", "x=1", "?", "Authorization", "Bearer", testToken, "pairing", "01ABC"} {
		if strings.Contains(out, banned) {
			t.Errorf("ログに %q が含まれている: %q", banned, out)
		}
	}
	for _, want := range []string{"method=POST", "path=/connections/:cid", "status=201", "duration="} {
		if !strings.Contains(out, want) {
			t.Errorf("ログに %q が無い: %q", want, out)
		}
	}
}

func TestRequestLog_UnmatchedPathAndLevels(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	r := gin.New()
	r.Use(middleware.RequestLog(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))))
	r.GET("/err", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	r.GET("/fine", func(c *gin.Context) { c.Status(http.StatusOK) })

	do(r, http.MethodGet, "/fine", "")
	if logs.Len() != 0 {
		t.Errorf("通常のリクエストは Debug のため Info では出ない: %q", logs.String())
	}
	do(r, http.MethodGet, "/err", "")
	if !strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("5xx は Warn: %q", logs.String())
	}
	logs.Reset()
	do(r, http.MethodGet, "/no/such/secret-id", "")
	out := logs.String()
	if strings.Contains(out, "secret-id") {
		t.Errorf("未一致の実パスを出している: %q", out)
	}
}

// asError は recover の戻り値を error として扱う（error でなければ nil）。
func asError(v any) error {
	err, _ := v.(error)
	return err
}
