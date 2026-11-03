package router_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ousiassllc/swarm-viewer/internal/service"
)

// スモークテスト用の固定値。token は明らかなダミー。
const smokeToken = "test-token-not-secret"

const smokeSessionsJSON = `{"sessions":[` +
	`{"id":"0","allowed":true},{"id":"hidden","allowed":false},{"id":"work","allowed":true},` +
	`{"id":"feature/x","allowed":true},{"id":"my session","allowed":true},{"id":"100%","allowed":true}]}`

// smokeReportJSON は session ごとに title を変えた report（usage・host 付き）。
func smokeReportJSON(session string) string {
	return `{"at":"2026-11-03T10:00:00Z","layout":{"rows":1,"cols":[2]},` +
		`"panes":[{"pos":"T1","id":"%1","agent":"omp","state":"working","title":"pane-title-` + session + `","cwd":"/w"},` +
		`{"pos":"T2","id":"%2","agent":null,"state":"idle","title":"second-pane","cwd":"/w"}],` +
		`"attention":[],` +
		`"usage":{"status":"ok","fetched_at":"2026-11-03T09:59:00Z","claude":[{"account":"acct-a",` +
		`"five_hour":{"available":true,"used":42,"unit":"percent","status":"ok","resets_at":null},` +
		`"seven_day":{"available":false,"used":null,"unit":null,"status":null,"resets_at":null}}]},` +
		`"host":{"cpu":{"available":true,"used_percent":37},"swap":{"available":false},"disk":[{"mount":"/","available":true,"free_gb":120,"total_gb":450,"used_percent":73}]}}`
}

// newStubConsole は swarm-console を模した stateless な Streamable HTTP の stub（go-sdk のサーバー実装）。
func newStubConsole(t *testing.T) *httptest.Server {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "stub-console", Version: "0"}, nil)
	textResult := func(text string) *mcp.CallToolResult {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
	}
	srv.AddTool(&mcp.Tool{Name: "sessions_list", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return textResult(smokeSessionsJSON), nil
		})
	srv.AddTool(&mcp.Tool{Name: "report", InputSchema: map[string]any{"type": "object"}},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				Session string `json:"session"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			return textResult(smokeReportJSON(args.Session)), nil
		})
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+smokeToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad token"}}`))
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

type seqIDs struct{ n atomic.Int32 }

func (s *seqIDs) NewID() string { return "01SMOKE" + strconv.Itoa(int(s.n.Add(1))) }

type grantedNotifier struct{}

func (grantedNotifier) Notify(context.Context, service.Notification) error { return nil }
func (grantedNotifier) Permitted() service.PermissionState                 { return service.PermissionGranted }

// client は Cookie なしの簡易クライアント。全応答の本文を集めて token の漏洩を確認できるようにする。
type client struct {
	t      *testing.T
	h      http.Handler
	csrf   string
	mu     sync.Mutex
	bodies []string
}

func (c *client) do(method, target string, hdr map[string]string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, target, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.csrf != "" {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	c.mu.Lock()
	c.bodies = append(c.bodies, w.Body.String())
	c.mu.Unlock()
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		c.t.Fatalf("%s %s: CSP = %q", method, target, got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		c.t.Fatalf("%s %s: Cache-Control = %q", method, target, got)
	}
	return w
}

func (c *client) expect(w *httptest.ResponseRecorder, status int) string {
	c.t.Helper()
	if w.Code != status {
		c.t.Fatalf("status = %d, want %d\n%s", w.Code, status, w.Body.String())
	}
	return w.Body.String()
}

// noLocation はリダイレクト（Location）を使っていないことを確認する。WebKit の WebView は http.Redirect を扱えない。
func (c *client) noLocation(w *httptest.ResponseRecorder) {
	c.t.Helper()
	if loc := w.Header().Get("Location"); loc != "" {
		c.t.Fatalf("Location = %q。リダイレクトを使ってはいけない", loc)
	}
}

func mustContain(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("%q が含まれない:\n%s", w, body)
		}
	}
}

func mustNotContain(t *testing.T, body string, banned ...string) {
	t.Helper()
	for _, b := range banned {
		if strings.Contains(body, b) {
			t.Errorf("%q が含まれている:\n%s", b, body)
		}
	}
}
