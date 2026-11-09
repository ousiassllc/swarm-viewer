package report

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

func opsServerTools() map[string]toolReply {
	tools := okTools()
	tools[toolPaneTail] = toolReply{text: `{"pane":"T1","lines":30,"text":"hello","truncated":false}`}
	tools[toolAskCard] = toolReply{text: `{"asking":true,"question":"q","options":[{"n":1,"label":"a","recommended":true}],"cursor":1,"allows_custom":false}`}
	tools[toolAskPick] = toolReply{text: `{"ok":true,"state":"working","picked":{"n":1,"label":"a"},"card_closed":true}`}
	tools[toolSendText] = toolReply{text: `{"ok":true,"state":"working","accepted":true,"chars":2}`}
	tools[toolSendEscape] = toolReply{text: `{"ok":true,"state":"idle","interrupted":true}`}
	tools[toolPaneStart] = toolReply{text: `{"ok":true,"state":"idle","agent":"omp","cwd":"/w","title":"agent: t"}`}
	tools[toolPaneClose] = toolReply{text: `{"ok":true,"state":"shell","note_archived":true}`}
	tools[toolDirList] = toolReply{text: `{"path":"/w","entries":[{"name":"a","path":"/w/a","git":true}],"truncated":true}`}
	return tools
}

// 操作の接続（/ops）: 各メソッドが期待どおりのツール名・引数・path・Bearer で届き、結果を解析する。
func TestOpsConnection_MethodsSendExpectedToolsAndArguments(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: opsServerTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	conn.URL = ts.URL + domain.OpsPath

	if _, err := c.PaneTail(t.Context(), conn, "0", "T1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PaneTail(t.Context(), conn, "0", "T1", 999); err != nil {
		t.Fatal(err)
	}
	if card, err := c.AskCard(t.Context(), conn, "0", "T1"); err != nil || !card.Asking || len(card.Options) != 1 || !card.Options[0].Recommended {
		t.Fatalf("AskCard = %+v, %v", card, err)
	}
	if r, err := c.AskPick(t.Context(), conn, "0", "T1", ops.Pick{N: 1}); err != nil || !r.OK || !r.CardClosed || r.PickedLabel != "a" {
		t.Fatalf("AskPick(n) = %+v, %v", r, err)
	}
	if _, err := c.AskPick(t.Context(), conn, "0", "T1", ops.Pick{Text: "other"}); err != nil {
		t.Fatal(err)
	}
	if r, err := c.SendText(t.Context(), conn, "0", "T1", "a\nb"); err != nil || !r.Accepted || r.Chars != 2 {
		t.Fatalf("SendText = %+v, %v", r, err)
	}
	if r, err := c.SendEscape(t.Context(), conn, "0", "T1", true); err != nil || !r.Interrupted {
		t.Fatalf("SendEscape = %+v, %v", r, err)
	}
	if r, err := c.PaneStart(t.Context(), conn, "0", "T1", ops.StartRequest{Agent: "omp", Cwd: "/w", Title: " t "}); err != nil || !r.OK || r.Agent != "omp" {
		t.Fatalf("PaneStart = %+v, %v", r, err)
	}
	if r, err := c.PaneClose(t.Context(), conn, "0", "T1"); err != nil || !r.OK || !r.NoteArchived {
		t.Fatalf("PaneClose = %+v, %v", r, err)
	}
	if r, err := c.DirList(t.Context(), conn, ""); err != nil || len(r.Entries) != 1 || !r.Entries[0].Git || !r.Truncated {
		t.Fatalf("DirList = %+v, %v", r, err)
	}
	if _, err := c.DirList(t.Context(), conn, "/w"); err != nil {
		t.Fatal(err)
	}

	type want struct {
		tool string
		args map[string]any
	}
	wants := []want{
		{"pane_tail", map[string]any{"session": "0", "pane": "T1", "lines": float64(30)}},
		{"pane_tail", map[string]any{"session": "0", "pane": "T1", "lines": float64(200)}},
		{"ask_card", map[string]any{"session": "0", "pane": "T1"}},
		{"ask_pick", map[string]any{"session": "0", "pane": "T1", "n": float64(1)}},
		{"ask_pick", map[string]any{"session": "0", "pane": "T1", "text": "other"}},
		{"send_text", map[string]any{"session": "0", "pane": "T1", "text": "a\nb"}},
		{"send_escape", map[string]any{"session": "0", "pane": "T1", "confirm": true}},
		{"pane_start", map[string]any{"session": "0", "pane": "T1", "agent": "omp", "cwd": "/w", "title": "t"}},
		{"pane_close", map[string]any{"session": "0", "pane": "T1"}},
		{"dir_list", map[string]any{}},
		{"dir_list", map[string]any{"path": "/w"}},
	}
	var calls []recordedRequest
	for _, r := range reqs.all() {
		if r.RPCMethod == "tools/call" {
			calls = append(calls, r)
		}
		if r.Auth != "Bearer "+testToken {
			t.Errorf("Authorization = %q", r.Auth)
		}
	}
	if len(calls) != len(wants) {
		t.Fatalf("tools/call = %d 件; want %d", len(calls), len(wants))
	}
	for i, w := range wants {
		if calls[i].Tool != w.tool || !mapsEqual(calls[i].Args, w.args) {
			t.Errorf("#%d: %s %v; want %s %v", i, calls[i].Tool, calls[i].Args, w.tool, w.args)
		}
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// 送る前の検査（通信しない）。session は常に明示、確認なしの Esc・同時指定・不正な入力は送らない。
func TestOpsConnection_InvalidInputIsRejectedBeforeCommunicating(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: opsServerTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	conn.URL = ts.URL + domain.OpsPath
	ctx := t.Context()

	type result struct {
		err  error
		code string
		kind domain.ErrKind
	}
	cases := map[string]result{
		"session 空": {err: func() error { _, err := c.SendText(ctx, conn, "", "T1", "x"); return err }(), code: "session_required", kind: domain.ErrSessionRequired},
		"pane 空":    {err: func() error { _, err := c.SendText(ctx, conn, "0", "", "x"); return err }(), code: "invalid_args", kind: domain.ErrOperation},
		"本文が空":      {err: func() error { _, err := c.SendText(ctx, conn, "0", "T1", ""); return err }(), code: "invalid_args", kind: domain.ErrOperation},
		"制御文字":      {err: func() error { _, err := c.SendText(ctx, conn, "0", "T1", "a\x00"); return err }(), code: "forbidden_control_char", kind: domain.ErrOperation},
		"長すぎる":      {err: func() error { _, err := c.SendText(ctx, conn, "0", "T1", strings.Repeat("a", 8001)); return err }(), code: "text_too_long", kind: domain.ErrOperation},
		"Esc 確認なし":  {err: func() error { _, err := c.SendEscape(ctx, conn, "0", "T1", false); return err }(), code: "confirm_required", kind: domain.ErrOperation},
		"pick 同時指定": {err: func() error { _, err := c.AskPick(ctx, conn, "0", "T1", ops.Pick{N: 1, Text: "x"}); return err }(), code: "invalid_args", kind: domain.ErrOperation},
		"start 不正": {err: func() error {
			_, err := c.PaneStart(ctx, conn, "0", "T1", ops.StartRequest{Agent: "vim", Cwd: "/w", Title: "t"})
			return err
		}(), code: "invalid_args", kind: domain.ErrOperation},
	}
	for name, tc := range cases {
		fe := assertFetchError(t, tc.err, tc.kind)
		if fe.Code != tc.code {
			t.Errorf("%s: code = %q; want %q", name, fe.Code, tc.code)
		}
	}
	if reqs.count() != 0 {
		t.Fatalf("requests = %d; want 0", reqs.count())
	}
}

// 操作ツールのエラー（HTTP 200 の isError）は operation で、Code を保つ。本文（message）は出さない。
func TestOpsConnection_ToolErrorsAreClassifiedAsOperation(t *testing.T) {
	codes := []string{
		"invalid_state", "not_asking", "option_out_of_range", "custom_not_allowed", "cursor_mismatch", "confirm_required",
		"forbidden_control_char", "text_too_long", "forbidden_path", "path_forbidden", "file_unreadable", "unsupported_agent",
		"pane_not_found", "busy_timeout", "exit_timeout", "not_found", "unknown_tool", "invalid_args",
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{toolSendText: toolError(code)}})
			c, logs := newTestClient(t)
			conn := testConn(ts)
			conn.URL = ts.URL + domain.OpsPath
			_, err := c.SendText(t.Context(), conn, "0", "T1", "x")
			fe := assertFetchError(t, err, domain.ErrOperation)
			if fe.Code != code {
				t.Errorf("code = %q", fe.Code)
			}
			if strings.Contains(err.Error(), secretMessage) || strings.Contains(logs.String(), secretMessage) {
				t.Error("エラー・ログに応答本文が含まれる")
			}
		})
	}
}

// invalid_args の details.field が session のときだけ session_missing（従来どおり）。
func TestOpsConnection_InvalidArgsOnSessionFieldIsSessionMissing(t *testing.T) {
	reply := toolReply{isError: true, text: `{"error":{"code":"invalid_args","message":"` + secretMessage + `","details":{"field":"session"}}}`}
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{toolPaneTail: reply}})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	conn.URL = ts.URL + domain.OpsPath
	_, err := c.PaneTail(t.Context(), conn, "gone", "T1", 30)
	_ = assertFetchError(t, err, domain.ErrSessionMissing)
}

func TestOpsConnection_UnexpectedToolResultIsProtocol(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{toolSendText: {text: "not json"}, toolPaneTail: {text: `["x"]`}}})
	c, logs := newTestClient(t)
	conn := testConn(ts)
	conn.URL = ts.URL + domain.OpsPath
	_, err := c.SendText(t.Context(), conn, "0", "T1", "x")
	_ = assertFetchError(t, err, domain.ErrProtocol)
	_, err = c.PaneTail(t.Context(), conn, "0", "T1", 30)
	_ = assertFetchError(t, err, domain.ErrProtocol)
	if strings.Contains(logs.String(), "not json") {
		t.Error("ログに応答本文が含まれる")
	}
}

// 401 は unauthorized。操作ツールでも他と同じ（service が needs_repair にする）。
func TestOpsConnection_BadTokenIsUnauthorized(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: opsServerTools()})
	c, _ := newTestClient(t)
	conn := testConn(ts)
	conn.URL = ts.URL + domain.OpsPath
	_, err := c.Probe(t.Context(), conn.URL, "wrong")
	_ = assertFetchError(t, err, domain.ErrUnauthorized)
}

func TestToolTimeouts_OperationToolsWaitLongerThanReads(t *testing.T) {
	c := &Client{timeout: 10 * 1_000_000_000}
	for tool, scale := range map[string]int64{toolSessionsList: 1, toolReport: 1, toolPaneTail: 1, toolAskCard: 1, toolDirList: 1, toolAskPick: 3, toolSendText: 3, toolSendEscape: 3, toolPaneStart: 5, toolPaneClose: 5} {
		if got := int64(c.timeoutFor(tool)); got != scale*int64(c.timeout) {
			t.Errorf("%s: timeout = %v; want %d 倍", tool, got, scale)
		}
	}
}
