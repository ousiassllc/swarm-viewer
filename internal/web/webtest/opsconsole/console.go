// Package opsconsole は操作モード（Issue #97）のテスト用の偽 swarm-console と、本物のハンドラの配線を提供する。
// テストコードからだけ使う（本番コードは import しない）。
//
// 偽 console は swarm-console と同じ形の stateless な Streamable HTTP（go-sdk のサーバー実装）で、`/report` には
// 見るだけの 4 ツール、`/ops` には操作の 11 ツール（見るだけの 4 つ ＋ 7 つ）を出す。受けたリクエストを path 別に記録する。
package opsconsole

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Token は偽 console が受け付ける Bearer。明らかなダミー。
const Token = "test-token-not-secret"

// Reply はツールの応答。IsError なら HTTP 200 の isError（ツール失敗）。
type Reply struct {
	Text    string
	IsError bool
}

// Request は偽 console が受けた 1 リクエスト。
type Request struct {
	Path       string
	HTTPMethod string
	Auth       string
	RPCMethod  string
	Tool       string // tools/call のときだけ
	Args       map[string]any
}

// Console は偽 swarm-console。
type Console struct {
	Server *httptest.Server

	mu      sync.Mutex
	reqs    []Request
	replies map[string]Reply
	reject  bool                     // true の間は全リクエストを 401 にする（失効した token の再現）
	holds   map[string]chan struct{} // ツール名 → 解除されるまで応答を待たせる
	entered map[string]chan struct{} // ツール名 → 呼ばれたことの通知
}

// 操作の 7 ツール。`/report` には出さない。
var opsOnly = map[string]bool{"ask_pick": true, "send_text": true, "send_escape": true, "pane_start": true, "pane_close": true, "dir_list": true, "skill_list": true}

var allTools = []string{"sessions_list", "report", "pane_tail", "ask_card", "ask_pick", "send_text", "send_escape", "pane_start", "pane_close", "dir_list", "skill_list"}

// New は偽 console を起動する。t.Cleanup で止める。応答は既定（fixtures.go）で、On で差し替える。
func New(t testing.TB) *Console {
	t.Helper()
	c := &Console{replies: defaultReplies(), holds: map[string]chan struct{}{}, entered: map[string]chan struct{}{}}
	mux := http.NewServeMux()
	mux.Handle("/report", c.endpoint(false))
	mux.Handle("/ops", c.endpoint(true))
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		reject := c.reject
		c.mu.Unlock()
		if reject || r.Header.Get("Authorization") != "Bearer "+Token {
			c.record(Request{Path: r.URL.Path, HTTPMethod: r.Method, Auth: r.Header.Get("Authorization")})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad token"}}`))
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(c.Server.Close)
	return c
}

// URL は接続文字列の url に使う接続先。kind は "/report"（見るだけ）か "/ops"（操作）。
func (c *Console) URL(path string) string { return c.Server.URL + path }

// On はツールの応答を差し替える。
func (c *Console) On(tool string, r Reply) {
	c.mu.Lock()
	c.replies[tool] = r
	c.mu.Unlock()
}

// RejectAuth は以後の全リクエストを 401 にする（token が失効した場合の再現）。
func (c *Console) RejectAuth() {
	c.mu.Lock()
	c.reject = true
	c.mu.Unlock()
}

// Hold はツールの応答を、返された release が呼ばれるまで待たせる。呼ばれたことは entered で受け取れる。
func (c *Console) Hold(tool string) (entered <-chan struct{}, release func()) {
	gate, in := make(chan struct{}), make(chan struct{}, 8)
	c.mu.Lock()
	c.holds[tool], c.entered[tool] = gate, in
	c.mu.Unlock()
	var once sync.Once
	return in, func() { once.Do(func() { close(gate) }) }
}

// Requests は path 宛てに受けたリクエスト（HTTP の POST/GET と JSON-RPC の全部。DELETE を含む）。
func (c *Console) Requests(path string) []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Request
	for _, r := range c.reqs {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// AllRequests は受けたリクエスト全部。
func (c *Console) AllRequests() []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Request(nil), c.reqs...)
}

// Calls は path 宛ての tools/call だけ。
func (c *Console) Calls(path string) []Request {
	var out []Request
	for _, r := range c.Requests(path) {
		if r.RPCMethod == "tools/call" {
			out = append(out, r)
		}
	}
	return out
}

// CallsOf は path 宛ての、ツール名が tool の tools/call。
func (c *Console) CallsOf(path, tool string) []Request {
	var out []Request
	for _, r := range c.Calls(path) {
		if r.Tool == tool {
			out = append(out, r)
		}
	}
	return out
}

func (c *Console) record(r Request) {
	c.mu.Lock()
	c.reqs = append(c.reqs, r)
	c.mu.Unlock()
}

// endpoint は 1 つの path の MCP サーバー。ops が false なら見るだけの 4 ツールだけ。
func (c *Console) endpoint(ops bool) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-console", Version: "0"}, nil)
	for _, name := range allTools {
		if opsOnly[name] && !ops {
			continue
		}
		srv.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, c.tool(name))
	}
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := Request{Path: r.URL.Path, HTTPMethod: r.Method, Auth: r.Header.Get("Authorization")}
		if r.Method == http.MethodPost {
			var msg struct {
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			body := readBody(r)
			_ = json.Unmarshal(body, &msg)
			rec.RPCMethod, rec.Tool, rec.Args = msg.Method, msg.Params.Name, msg.Params.Arguments
		}
		c.record(rec)
		h.ServeHTTP(w, r)
	})
}

func (c *Console) tool(name string) mcp.ToolHandler {
	return func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c.mu.Lock()
		reply, gate, in := c.replies[name], c.holds[name], c.entered[name]
		c.mu.Unlock()
		if in != nil {
			in <- struct{}{}
		}
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: reply.Text}}, IsError: reply.IsError}, nil
	}
}
