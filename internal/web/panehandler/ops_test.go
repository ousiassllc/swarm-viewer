package panehandler_test

import (
	"net/http"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// callOf は path 宛ての tool の呼び出しがちょうど 1 回だったことを確かめ、その記録を返す。
func callOf(t *testing.T, s *opsconsole.Stack, tool string) opsconsole.Request {
	t.Helper()
	got := s.Console.CallsOf(pathOps, tool)
	if len(got) != 1 {
		t.Fatalf("%s の呼び出し = %d 回; want 1（全体: %v）", tool, len(got), toolNames(s.Console.Calls(pathOps)))
	}
	r := got[0]
	if r.Path != pathOps || r.Auth != "Bearer "+opsconsole.Token {
		t.Errorf("%s: path=%q auth=%q; want /ops と Bearer", tool, r.Path, r.Auth)
	}
	if r.Args["session"] == nil && tool != "dir_list" && tool != "skill_list" {
		t.Errorf("%s: session が明示されていない: %v", tool, r.Args)
	}
	return r
}

func TestOpsConnection_AskPickByNumber_ReachesConsoleWithSessionPaneAndN(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Post(opsconsole.PanePath("T2", "/pick"), form("n", "2", "lines", "100")), 200)

	r := callOf(t, s, "ask_pick")
	if r.Args["session"] != "0" || r.Args["pane"] != "T2" || r.Args["n"] != float64(2) {
		t.Errorf("引数 = %v; want session=0 pane=T2 n=2", r.Args)
	}
	if _, hasText := r.Args["text"]; hasText {
		t.Errorf("n のときに text を送っている: %v", r.Args)
	}
	mustContain(t, html, "送りました", `id="toasts"`, `id="pane-live"`, "lines=100")
}

func TestOpsConnection_AskPickCustomText_SendsTextWithoutN(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	body(t, s.Post(opsconsole.PanePath("T2", "/pick"), form("text", "自分の答え\r\n2 行目")), 200)

	r := callOf(t, s, "ask_pick")
	if r.Args["text"] != "自分の答え\n2 行目" {
		t.Errorf("text = %q; want 改行を \\n に揃えた本文", r.Args["text"])
	}
	if _, hasN := r.Args["n"]; hasN {
		t.Errorf("text のときに n を送っている: %v", r.Args)
	}
}

func TestOpsConnection_AskPickRejectsBothNAndText_WithoutCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Post(opsconsole.PanePath("T2", "/pick"), form("n", "1", "text", "x")), 200)
	mustContain(t, html, "入力内容を受け付けられませんでした")
	if n := len(s.Console.CallsOf(pathOps, "ask_pick")); n != 0 {
		t.Errorf("ask_pick = %d 回; want 0", n)
	}
}

func TestOpsConnection_SendText_ReachesConsoleAndClearsInput(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Post(opsconsole.PanePath("T3", "/send"), form("text", "続けてください\n次の行")), 200)

	r := callOf(t, s, "send_text")
	if r.Args["session"] != "0" || r.Args["pane"] != "T3" || r.Args["text"] != "続けてください\n次の行" {
		t.Errorf("引数 = %v", r.Args)
	}
	mustContain(t, html, "送りました")
	// 受理されたので、入力欄は空で出す（入力中の文字を残す hx-preserve を付けない）。
	mustNotContain(t, html, "hx-preserve")
}

func TestOpsConnection_SendEscape_RequiresConfirmedForm(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)

	// 確認ダイアログを経ていない（confirm なし・true 以外）送信は、コンソールへ通信しない。
	for _, f := range []map[string]string{nil, {"confirm": ""}, {"confirm": "1"}, {"confirm": "false"}} {
		var kv []string
		for k, v := range f {
			kv = append(kv, k, v)
		}
		html := body(t, s.Post(opsconsole.PanePath("T1", "/escape"), form(kv...)), 200)
		mustContain(t, html, "確認が必要です")
	}
	if n := len(s.Console.CallsOf(pathOps, "send_escape")); n != 0 {
		t.Fatalf("確認なしで send_escape が %d 回呼ばれた", n)
	}

	html := body(t, s.Post(opsconsole.PanePath("T1", "/escape"), form("confirm", "true")), 200)
	r := callOf(t, s, "send_escape")
	if r.Args["session"] != "0" || r.Args["pane"] != "T1" || r.Args["confirm"] != true {
		t.Errorf("引数 = %v; want confirm: true", r.Args)
	}
	mustContain(t, html, "中断しました")
}

func TestOpsConnection_StartFlow_DirListThenPaneStart_ConfirmationDoesNotStart(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	start := opsconsole.PanePath("T4", "/start")

	// 画面（shell のペイン）→ ピッカーが dir_list（path なし = 許可された根）を読む。
	page := body(t, s.Get(start), 200)
	mustContain(t, page, "omp", "claude", "codex", `hx-get="`+opsconsole.PanePath("T4", "/start/dirs")+`"`, `hx-trigger="load"`)
	dirs := body(t, s.Get(opsconsole.PanePath("T4", "/start/dirs")), 200)
	mustContain(t, dirs, "work", "docs", "git")
	if r := callOf(t, s, "dir_list"); r.Args["path"] != nil || r.Args["session"] != nil || r.Args["limit"] != nil {
		t.Errorf("dir_list の引数 = %v; want path・session・limit なし", r.Args)
	}
	// 1 階層下へ（path を渡す）。「このフォルダを選ぶ」は通信せず選択済みの表示にする。
	body(t, s.Get(opsconsole.PanePath("T4", "/start/dirs?path=%2Fwork")), 200)
	if got := s.Console.CallsOf(pathOps, "dir_list"); len(got) != 2 || got[1].Args["path"] != "/work" {
		t.Errorf("dir_list = %v", got)
	}
	picked := body(t, s.Get(opsconsole.PanePath("T4", "/start/dirs?pick=1&path=%2Fwork%2Fnew")), 200)
	mustContain(t, picked, `name="cwd" value="/work/new"`)
	if n := len(s.Console.CallsOf(pathOps, "dir_list")); n != 2 {
		t.Errorf("選択で dir_list を呼んだ（%d 回）", n)
	}

	// 入力 → 確認画面（pane_start はまだ呼ばない。タイトルの前後の空白は除く）。
	conf := body(t, s.Post(start, form("agent", "claude", "cwd", "/work/new", "title", "  funid #982 ")), 200)
	mustContain(t, conf, "data-start-summary", "claude", "/work/new", "funid #982", `name="confirm"`)
	if n := len(s.Console.CallsOf(pathOps, "pane_start")); n != 0 {
		t.Fatalf("確認画面の前に pane_start が呼ばれた（%d 回）", n)
	}

	// 確定 → pane_start（session 明示）→ ペイン詳細へ HX-Redirect。
	w := s.Post(start, form("agent", "claude", "cwd", "/work/new", "title", "funid #982", "confirm", "1"))
	body(t, w, 200)
	if got := w.Header().Get("HX-Redirect"); got != opsconsole.PanePath("T4", "") {
		t.Errorf("HX-Redirect = %q", got)
	}
	r := callOf(t, s, "pane_start")
	if r.Args["session"] != "0" || r.Args["pane"] != "T4" || r.Args["agent"] != "claude" || r.Args["cwd"] != "/work/new" || r.Args["title"] != "funid #982" {
		t.Errorf("引数 = %v", r.Args)
	}
}

func TestOpsConnection_PaneClose_ReachesConsoleAndReturnsToMainScreen(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	w := s.Post(opsconsole.PanePath("T3", "/close"), form("confirm", "true"))
	body(t, w, http.StatusOK)
	if got := w.Header().Get("HX-Redirect"); got != "/connections/c1/sessions/0" {
		t.Errorf("HX-Redirect = %q; want メイン画面", got)
	}
	r := callOf(t, s, "pane_close")
	if r.Args["session"] != "0" || r.Args["pane"] != "T3" || len(r.Args) != 2 {
		t.Errorf("引数 = %v", r.Args)
	}
}
