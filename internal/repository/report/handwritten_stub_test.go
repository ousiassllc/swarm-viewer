package report

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// newHandwrittenServer は go-sdk のサーバー実装を使わず、swarm-console の挙動（POST のみ、GET・DELETE は 405、
// Mcp-Session-Id なし、通知は 202）を手書きで再現した stub。SDK のクライアントがサーバー実装の癖に
// 依存していないことを確かめる。sse が true なら応答を SSE（text/event-stream）で返す。
func newHandwrittenServer(t *testing.T, sse bool) (*httptest.Server, *requestLog) {
	t.Helper()
	return newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeHTTPError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.Header().Set("Www-Authenticate", "Bearer")
			writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		body, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &msg)
		var result string
		switch msg.Method {
		case "initialize":
			result = `{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"swarm-console","version":"0"}}`
		case "tools/call":
			text, _ := json.Marshal(okTools()[msg.Params.Name].text)
			result = `{"content":[{"type":"text","text":` + string(text) + `}]}`
		default:
			w.WriteHeader(http.StatusAccepted) // notifications/initialized など
			return
		}
		payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, result)
		if sse {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	})
}

func TestWorksWithHandwrittenStatelessServer(t *testing.T) {
	eachResponseMode(t, func(t *testing.T, jsonResponse bool) {
		ts, reqs := newHandwrittenServer(t, !jsonResponse)
		c, logs := newTestClient(t)

		sessions, err := c.Sessions(t.Context(), testConn(ts))
		if err != nil || len(sessions) != 2 {
			t.Fatalf("Sessions = %v, %v", sessions, err)
		}
		rep, err := c.Report(t.Context(), testConn(ts), "0")
		if err != nil || len(rep.Panes) != 2 {
			t.Fatalf("Report = %+v, %v", rep, err)
		}
		// GET・DELETE（405 になる経路）は一度も送らない。
		for _, r := range reqs.all() {
			if r.HTTPMethod != http.MethodPost {
				t.Errorf("unexpected %s request", r.HTTPMethod)
			}
		}
		want := []string{
			"POST initialize", "POST notifications/initialized", "POST tools/call",
			"POST initialize", "POST notifications/initialized", "POST tools/call",
		}
		if got := rpcMethods(reqs); !slices.Equal(got, want) {
			t.Fatalf("wire = %v, want %v", got, want)
		}
		if logs.Len() != 0 {
			t.Errorf("success path logged: %s", logs)
		}
	})
}
