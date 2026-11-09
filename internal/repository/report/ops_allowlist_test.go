package report

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

var (
	viewToolNames = []string{"ask_card", "pane_tail", "report", "sessions_list"}
	opsToolNames  = []string{"ask_card", "ask_pick", "dir_list", "pane_close", "pane_start", "pane_tail", "report", "send_escape", "send_text", "sessions_list"}
	// 範囲外（どちらの接続でも呼ばない）。
	outOfScope = []string{"", "dispatch", "note_set", "note_log", "file_get", "panes_status", "host_status", "report_read", "tools/list", "Report", "send_text "}
)

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestAllowlistConstants_ViewAndOpsToolsPerConnectionKind(t *testing.T) {
	if got := sortedKeys(viewTools); !slices.Equal(got, viewToolNames) {
		t.Errorf("viewTools = %v; want %v", got, viewToolNames)
	}
	if got := sortedKeys(opsTools); !slices.Equal(got, opsToolNames) {
		t.Errorf("opsTools = %v; want %v", got, opsToolNames)
	}
	for _, name := range viewToolNames {
		if !toolAllowed(domain.KindView, name) || !toolAllowed(domain.KindOps, name) {
			t.Errorf("%s は両方の種類で許可される", name)
		}
	}
	for _, name := range opsToolNames {
		if !slices.Contains(viewToolNames, name) && toolAllowed(domain.KindView, name) {
			t.Errorf("操作ツール %s が見るだけの接続で許可されている", name)
		}
		if !toolAllowed(domain.KindOps, name) {
			t.Errorf("%s が操作の接続で許可されない", name)
		}
	}
	for _, name := range outOfScope {
		if toolAllowed(domain.KindView, name) || toolAllowed(domain.KindOps, name) {
			t.Errorf("範囲外 %q が許可されている", name)
		}
	}
	// 判定できない種類（空）は見るだけと同じ。
	if toolAllowed("", "send_text") {
		t.Error("種類が空のとき操作ツールが許可されている")
	}
}

// 見るだけの接続で操作ツールを呼ぶと、通信せずに protocol / tool_not_allowed。
func TestViewConnection_OperationToolsAreRejectedWithoutCommunicating(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: opsServerTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts) // /report

	calls := map[string]func() error{
		"ask_pick":    func() error { _, err := c.AskPick(t.Context(), conn, "0", "T1", ops.Pick{N: 1}); return err },
		"send_text":   func() error { _, err := c.SendText(t.Context(), conn, "0", "T1", "hi"); return err },
		"send_escape": func() error { _, err := c.SendEscape(t.Context(), conn, "0", "T1", true); return err },
		"pane_start": func() error {
			_, err := c.PaneStart(t.Context(), conn, "0", "T1", ops.StartRequest{Agent: "omp", Cwd: "/w", Title: "t"})
			return err
		},
		"pane_close": func() error { _, err := c.PaneClose(t.Context(), conn, "0", "T1"); return err },
		"dir_list":   func() error { _, err := c.DirList(t.Context(), conn, ""); return err },
	}
	for name, call := range calls {
		fe := assertFetchError(t, call(), domain.ErrProtocol)
		if fe.Code != "tool_not_allowed" {
			t.Errorf("%s: code = %q; want tool_not_allowed", name, fe.Code)
		}
	}
	if reqs.count() != 0 {
		t.Fatalf("requests = %d; want 0", reqs.count())
	}
}

// 送信内容の検査: `/report` に操作ツールは出せない。`/ops` なら出せる。範囲外はどちらにも出せない。
func TestRoundTripperChecksToolsPerEndpointPath(t *testing.T) {
	call := func(tool string) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":{}}}`
	}
	send := func(t *testing.T, endpoint, tool string) (calls int, err error) {
		t.Helper()
		base := &countingTransport{}
		rt := &roundTripper{base: base, endpoint: endpoint, obs: &observation{}, token: func() (string, error) { return testToken, nil }}
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, strings.NewReader(call(tool)))
		resp, err := rt.RoundTrip(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return base.calls, err
	}
	for _, tool := range opsToolNames {
		_, viewErr := send(t, "http://127.0.0.1:48121/report", tool)
		wantAllowed := slices.Contains(viewToolNames, tool)
		if (viewErr == nil) != wantAllowed {
			t.Errorf("/report で %s: err=%v; 許可=%v のはず", tool, viewErr, wantAllowed)
		}
		if n, opsErr := send(t, "http://127.0.0.1:48121/ops", tool); opsErr != nil || n != 1 {
			t.Errorf("/ops で %s: err=%v calls=%d", tool, opsErr, n)
		}
	}
	for _, tool := range outOfScope[1:] { // "" は tools/call として不正
		for _, endpoint := range []string{"http://127.0.0.1:48121/report", "http://127.0.0.1:48121/ops"} {
			if n, err := send(t, endpoint, tool); err == nil || n != 0 {
				t.Errorf("%s で範囲外 %q が通った（calls=%d）", endpoint, tool, n)
			}
		}
	}
	// 経路の取り違え: /report 用に作った roundTripper で /ops 宛ては送れない（宛先は呼び出し開始時の 1 つだけ）。
	base := &countingTransport{}
	rt := &roundTripper{base: base, endpoint: "http://127.0.0.1:48121/report", obs: &observation{}, token: func() (string, error) { return testToken, nil }}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://127.0.0.1:48121/ops", strings.NewReader(call("send_text")))
	if resp, err := rt.RoundTrip(req); err == nil || base.calls != 0 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Errorf("別 path への送信が通った（calls=%d, err=%v）", base.calls, err)
	}
}
