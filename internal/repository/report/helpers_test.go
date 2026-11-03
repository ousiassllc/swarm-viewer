package report

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/repository/secret"
)

const (
	testToken  = "test-token-not-secret"
	testConnID = "conn-1"
	// body に出てはいけない値（エラー・ログへの混入検査用）。
	secretMessage = "secret-message-body-do-not-leak"
)

// recordedRequest は stub が受けた 1 リクエスト。
type recordedRequest struct {
	HTTPMethod string
	Auth       string
	RPCMethod  string
	Tool       string
	Args       map[string]any
}

type requestLog struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

func (l *requestLog) add(r recordedRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reqs = append(l.reqs, r)
}

func (l *requestLog) all() []recordedRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]recordedRequest(nil), l.reqs...)
}

func (l *requestLog) count() int { return len(l.all()) }

// record は受けたリクエスト（本文は戻す）を requestLog へ残す。
func (l *requestLog) record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{HTTPMethod: r.Method, Auth: r.Header.Get("Authorization")}
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			var msg struct {
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			_ = json.Unmarshal(body, &msg)
			rec.RPCMethod, rec.Tool, rec.Args = msg.Method, msg.Params.Name, msg.Params.Arguments
		}
		l.add(rec)
		next.ServeHTTP(w, r)
	})
}

// toolReply は stub のツールの応答。
type toolReply struct {
	text    string
	isError bool
}

// stubOptions は stateless サーバーの挙動。
type stubOptions struct {
	jsonResponse bool // true: 単発 JSON、false: SSE
	tools        map[string]toolReply
}

// newStatelessServer は swarm-console を模した stateless な Streamable HTTP サーバー（go-sdk のサーバー実装、
// GET・DELETE は 405）を立てる。認証が合わなければ swarm-console と同じ形の 401 JSON を返す。
func newStatelessServer(t *testing.T, opts stubOptions) (*httptest.Server, *requestLog) {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "stub-console", Version: "0"}, nil)
	for name, reply := range opts.tools {
		srv.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: reply.text}},
					IsError: reply.isError,
				}, nil
			})
	}
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: opts.jsonResponse})
	log := &requestLog{}
	ts := httptest.NewServer(log.record(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})))
	t.Cleanup(ts.Close)
	return ts, log
}

// newRawServer は handler をそのまま立てる（HTTP レベルの失敗の再現用）。
func newRawServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *requestLog) {
	t.Helper()
	log := &requestLog{}
	ts := httptest.NewServer(log.record(h))
	t.Cleanup(ts.Close)
	return ts, log
}

func writeHTTPError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": secretMessage}})
	_, _ = w.Write(body)
}

func reportURL(ts *httptest.Server) string { return ts.URL + domain.ReportPath }

func testConn(ts *httptest.Server) domain.Connection {
	return domain.Connection{ID: testConnID, Name: "stub", URL: reportURL(ts)}
}

// newTestClient は token を保存済みの Client と、ログの出力先を返す。
func newTestClient(t *testing.T) (*Client, *bytes.Buffer) {
	t.Helper()
	keyring.MockInit()
	store := secret.NewStore()
	if err := store.Set(testConnID, testToken); err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewClient(store, logger), logs
}

const sessionsJSON = `{"sessions":[` +
	`{"id":"0","allowed":true,"operable":false,"layout":{"rows":1,"cols":[1]},"panes":["T1"]},` +
	`{"id":"1","allowed":false},` +
	`{"id":"2","allowed":true}]}`

const reportJSON = `{"at":"2026-11-03T10:00:00Z","layout":{"rows":1,"cols":[2]},` +
	`"panes":[{"pos":"T1","id":"%1","agent":"omp","state":"asking","title":"t","cwd":"/w"},` +
	`{"pos":"T2","id":"%2","agent":null,"state":"idle","title":"u","cwd":"/w"}],` +
	`"attention":[],"future_key":{"x":1}}`

func okTools() map[string]toolReply {
	return map[string]toolReply{
		toolSessionsList: {text: sessionsJSON},
		toolReport:       {text: reportJSON},
	}
}

func toolError(code string) toolReply {
	return toolReply{isError: true, text: `{"error":{"code":"` + code + `","message":"` + secretMessage + `","details":{"allowed_count":2}}}`}
}
