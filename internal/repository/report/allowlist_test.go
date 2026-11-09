package report

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func TestViewAllowlistContainsOnlyTheFourTools(t *testing.T) {
	var names []string
	for name := range viewTools {
		names = append(names, name)
	}
	slices.Sort(names)
	if want := []string{"ask_card", "pane_tail", "report", "sessions_list"}; !slices.Equal(names, want) {
		t.Fatalf("viewTools = %v, want %v", names, want)
	}
	for _, bad := range []string{"", "panes_status", "host_status", "report_read", "dispatch", "note_set", "file_get", "Report", "report "} {
		if toolAllowed(domain.KindView, bad) {
			t.Errorf("toolAllowed(view, %q) = true", bad)
		}
	}
}

// 公開メソッドのどれを呼んでも、線路に載るツール名と JSON-RPC メソッドは許可リストの範囲に収まる。
func TestOnlyAllowedToolsReachTheWire(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	_, _ = c.Sessions(t.Context(), conn)
	_, _ = c.Report(t.Context(), conn, "0")
	_, _ = c.Probe(t.Context(), reportURL(ts), testToken)

	seen := map[string]bool{}
	for _, r := range reqs.all() {
		if r.HTTPMethod != http.MethodPost {
			t.Errorf("%s reached the wire", r.HTTPMethod)
		}
		if _, ok := allowedRPCMethods[r.RPCMethod]; !ok {
			t.Errorf("rpc method %q reached the wire", r.RPCMethod)
		}
		if r.RPCMethod == "tools/call" {
			if !toolAllowed(domain.KindView, r.Tool) {
				t.Errorf("tool %q reached the wire", r.Tool)
			}
			seen[r.Tool] = true
		}
	}
	if !seen["sessions_list"] || !seen["report"] {
		t.Errorf("tools seen = %v", seen)
	}
}

func TestCallToolRejectsDisallowedToolBeforeNetwork(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	for _, name := range []string{"panes_status", "ops_send_keys", "", "tools/list"} {
		_, fe := c.callTool(t.Context(), reportURL(ts), func() (string, error) { return testToken, nil }, name, nil)
		if fe == nil || fe.Kind != domain.ErrProtocol || fe.Code != "tool_not_allowed" {
			t.Errorf("callTool(%q) = %v", name, fe)
		}
	}
	if reqs.count() != 0 {
		t.Fatalf("requests = %d", reqs.count())
	}
}

// roundTripper 単体: 許可リスト外の本文・メソッド・宛先は base に渡らない。
func TestRoundTripperRejectsUnlistedRequests(t *testing.T) {
	const endpoint = "http://127.0.0.1:48121/report"
	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"tool not listed", "POST", endpoint, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"panes_status","arguments":{}}}`},
		{"ops tool", "POST", endpoint, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send_text"}}`},
		{"tools/list", "POST", endpoint, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`},
		{"server/discover", "POST", endpoint, `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`},
		{"batch", "POST", endpoint, `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"report"}}]`},
		{"not json", "POST", endpoint, `nope`},
		{"empty body", "POST", endpoint, ``},
		{"GET stream", "GET", endpoint, ``},
		{"PUT", "PUT", endpoint, `{}`},
		{"/ops path", "POST", "http://127.0.0.1:48121/ops", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
		{"other port", "POST", "http://127.0.0.1:48122/report", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
		{"other host", "POST", "http://100.64.0.9:48120/report", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
		{"hostname", "POST", "http://localhost:48121/report", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
		{"query", "POST", endpoint + "?x=1", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
		{"https", "POST", "https://127.0.0.1:48121/report", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := &countingTransport{}
			rt := &roundTripper{base: base, endpoint: endpoint, obs: &observation{},
				token: func() (string, error) { return testToken, nil }}
			req, err := http.NewRequestWithContext(t.Context(), tc.method, tc.url, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := rt.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
				t.Fatal("request was not rejected")
			}
			if base.calls != 0 {
				t.Fatal("rejected request reached the base transport")
			}
			for _, leak := range []string{testToken, tc.url} {
				if leak != "" && strings.Contains(err.Error(), leak) {
					t.Errorf("error leaks %q: %v", leak, err)
				}
			}
			if _, _, rejected, _ := rt.obs.snapshot(); !rejected {
				t.Error("rejection not recorded")
			}
		})
	}
}

func TestRoundTripperAddsBearerToAllowedRequest(t *testing.T) {
	const endpoint = "http://127.0.0.1:48121/report"
	base := &countingTransport{}
	rt := &roundTripper{base: base, endpoint: endpoint, obs: &observation{},
		token: func() (string, error) { return testToken, nil }}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint,
		strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"report","arguments":{"format":"json","session":"0"}}}`))
	req.Header.Set("Authorization", "Bearer attacker-supplied")
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if base.calls != 1 || base.auth != "Bearer "+testToken {
		t.Fatalf("calls=%d auth=%q", base.calls, base.auth)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer attacker-supplied" {
		t.Errorf("original request mutated: %q", got)
	}
	if base.body == "" || !strings.Contains(base.body, `"report"`) {
		t.Errorf("body not forwarded: %q", base.body)
	}
}

func TestRoundTripperTokenFailureSendsNothing(t *testing.T) {
	base := &countingTransport{}
	obs := &observation{}
	rt := &roundTripper{base: base, endpoint: "http://127.0.0.1:48121/report", obs: obs,
		token: func() (string, error) { return "", errors.New("boom") }}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, rt.endpoint,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	resp, err := rt.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || base.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, base.calls)
	}
	if _, _, _, tokenErr := obs.snapshot(); tokenErr == nil {
		t.Error("token error not recorded")
	}
}

type countingTransport struct {
	calls int
	auth  string
	body  string
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls++
	c.auth = req.Header.Get("Authorization")
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
	}
	return &http.Response{StatusCode: http.StatusAccepted, Body: http.NoBody, Header: http.Header{}, Request: req}, nil
}

// URL は呼び出しごとに ValidateURL で再検証する。範囲外は通信せずに拒否する。
func TestInvalidConnectionURLIsRejectedWithoutRequest(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, logs := newTestClient(t)
	port := ts.Listener.Addr().String()
	for name, raw := range map[string]string{
		"empty":              "",
		"hostname":           "http://localhost:48120/report",
		"ops trailing /":     "http://" + port + "/ops/",
		"trailing /":         "http://" + port + "/report/",
		"query":              "http://" + port + "/report?a=1",
		"userinfo":           "http://user:pw@" + port + "/report",
		"https":              "https://" + port + "/report",
		"public ip":          "http://8.8.8.8:48120/report",
		"unspecified":        "http://0.0.0.0:48120/report",
		"tailnet wrong port": "http://100.64.0.5:8080/report",
	} {
		t.Run(name, func(t *testing.T) {
			conn := domain.Connection{ID: testConnID, URL: raw}
			_, err := c.Sessions(t.Context(), conn)
			fe := assertFetchError(t, err, domain.ErrProtocol)
			if fe.Code != "invalid_url" {
				t.Errorf("code = %q", fe.Code)
			}
			_, err = c.Probe(t.Context(), raw, testToken)
			_ = assertFetchError(t, err, domain.ErrProtocol)
			if strings.Contains(logs.String(), "pw@") || (raw != "" && strings.Contains(logs.String(), raw)) {
				t.Errorf("log leaks url: %s", logs)
			}
		})
	}
	if reqs.count() != 0 {
		t.Fatalf("requests = %d", reqs.count())
	}
}

func TestEndpointIsNormalizedBeforeUse(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	conn.URL = "  " + strings.ToUpper(strings.TrimSuffix(conn.URL, "/report"))[:len("HTTP")] + conn.URL[len("http"):] + "  "
	if _, err := c.Sessions(t.Context(), conn); err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if reqs.count() == 0 {
		t.Fatal("no request")
	}
}
