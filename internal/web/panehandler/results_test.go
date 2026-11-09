package panehandler_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// ok:false・accepted:false・card_closed:false・interrupted:false などの判定の通知（トースト。契約 §3）。
func TestOperationResults_ShowFixedToastText(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		reply string
		post  func(*opsconsole.Stack) string
		want  string
	}{
		{"ask_pick ok:true card_closed:true", "ask_pick", `{"ok":true,"state":"working","card_closed":true}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T2", "/pick", "n", "1") }, "送りました"},
		{"ask_pick ok:true でも card_closed:false なら失敗", "ask_pick", `{"ok":true,"state":"asking","card_closed":false}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T2", "/pick", "n", "1") }, "選択を確定できませんでした。もう一度お試しください"},
		{"ask_pick ok:false", "ask_pick", `{"ok":false,"state":"asking","card_closed":false}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T2", "/pick", "n", "1") }, "選択を確定できませんでした。もう一度お試しください"},
		{"send_text accepted:true", "send_text", `{"ok":true,"state":"working","accepted":true,"chars":2}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T3", "/send", "text", "hi") }, "送りました"},
		{"send_text accepted:false", "send_text", `{"ok":false,"accepted":false,"state":"idle"}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T3", "/send", "text", "hi") }, "受け付けられませんでした"},
		{"send_escape interrupted:true", "send_escape", `{"ok":true,"state":"idle","interrupted":true}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T1", "/escape", "confirm", "true") }, "中断しました"},
		{"send_escape ok:false", "send_escape", `{"ok":false,"state":"working"}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T1", "/escape", "confirm", "true") }, "状態が変わりませんでした"},
		{"pane_close ok:false は状態つき", "pane_close", `{"ok":false,"state":"idle"}`,
			func(s *opsconsole.Stack) string { return postOp(t, s, "T3", "/close", "confirm", "true") }, "片付けを確認できませんでした（状態: idle）"},
		{"pane_start ok:false は状態つき", "pane_start", `{"ok":false,"state":"shell"}`,
			func(s *opsconsole.Stack) string {
				return postOp(t, s, "T4", "/start", "agent", "omp", "cwd", "/work", "title", "t", "confirm", "1")
			}, "起動を確認できませんでした（状態: shell）"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			s.Console.On(tc.tool, opsconsole.Reply{Text: tc.reply})
			mustContain(t, tc.post(s), tc.want)
		})
	}
}

func postOp(t *testing.T, s *opsconsole.Stack, pos, suffix string, kv ...string) string {
	t.Helper()
	return body(t, s.Post(opsconsole.PanePath(pos, suffix), form(kv...)), 200)
}

// 操作ツールのエラーコードは固定文言で出す。swarm-console の message は画面に出さない。
func TestOperationErrors_ShowFixedMessageNotConsoleMessage(t *testing.T) {
	codes := map[string]string{
		"invalid_state":          "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください",
		"not_asking":             "この質問はすでに閉じています",
		"option_out_of_range":    "その選択肢はありません。最新の質問を読み込み直してください",
		"custom_not_allowed":     "この質問は自由入力に対応していません",
		"cursor_mismatch":        "選択肢を合わせられませんでした（何も送っていません）。もう一度お試しください",
		"confirm_required":       "確認が必要です",
		"forbidden_control_char": "使えない制御文字が含まれています（改行とタブだけ使えます）",
		"text_too_long":          "長すぎます（8000 字まで）",
		"forbidden_path":         "このフォルダは選べません",
		"path_forbidden":         "このフォルダは選べません",
		"file_unreadable":        "ファイルを読めませんでした",
		"unsupported_agent":      "このペインは対応していないエージェントです",
		"pane_not_found":         "ペインが見つかりません。画面を更新してください",
		"busy_timeout":           "同じペインの別の操作がまだ終わっていません。しばらくしてからお試しください",
		"exit_timeout":           "終了が間に合いませんでした（もう一度試せます）",
		"not_found":              "フォルダが見つかりません",
		"unknown_tool":           "この接続では使えない操作です（ホストの swarm-console が古い可能性があります）",
		"invalid_args":           "入力内容を受け付けられませんでした",
	}
	for code, want := range codes {
		t.Run(code, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			s.Console.On("send_text", opsconsole.ToolError(code))
			html := postOp(t, s, "T3", "/send", "text", "hi")
			mustContain(t, html, want, "data-alert")
			mustNotContain(t, html, opsconsole.ConsoleMessage)
		})
	}
}

func TestOperationErrors_StaleCardIsRefetched(t *testing.T) {
	for _, code := range []string{"cursor_mismatch", "not_asking", "option_out_of_range"} {
		t.Run(code, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			s.Console.On("ask_pick", opsconsole.ToolError(code))
			html := postOp(t, s, "T2", "/pick", "n", "9")
			// 送信の前には取り直さない（押したらすぐ送る）。失敗のあとに最新のカードを取り直して出す。
			if n := len(s.Console.CallsOf(pathOps, "ask_card")); n != 1 {
				t.Errorf("ask_card = %d 回; want 失敗後の 1 回", n)
			}
			mustContain(t, html, "data-ask-card", "ASK-QUESTION-MARKER")
		})
	}
}

func TestOperationErrors_ExitTimeoutOffersRetry(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("pane_close", opsconsole.ToolError("exit_timeout"))
	w := s.Post(opsconsole.PanePath("T3", "/close"), form("confirm", "true"))
	html := body(t, w, 200)
	if w.Header().Get("HX-Redirect") != "" {
		t.Error("失敗なのにメイン画面へ移動している")
	}
	mustContain(t, html, "終了が間に合いませんでした（もう一度試せます）", "もう一度試す", "hx-confirm")
}

// 操作ツールの invalid_args は、ペイン・入力の不正（operation）。session の名前の不在（details.field=session）と区別する。
func TestOperationErrors_InvalidArgsOnSessionIsSessionMissing(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("send_text", opsconsole.Reply{IsError: true, Text: `{"error":{"code":"invalid_args","message":"x","details":{"field":"session"}}}`})
	html := postOp(t, s, "T3", "/send", "text", "hi")
	mustContain(t, html, "セッションが存在しません")
	mustNotContain(t, html, "入力内容を受け付けられませんでした")
}

// 送る前の検査（通信しない）。
func TestSendText_InvalidInputIsRejectedBeforeCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	cases := map[string]struct{ text, want string }{
		"空":      {"", "入力内容を受け付けられませんでした"},
		"制御文字":   {"a\x1bb", "使えない制御文字が含まれています（改行とタブだけ使えます）"},
		"8001 字": {strings.Repeat("あ", 8001), "長すぎます（8000 字まで）"},
	}
	for name, tc := range cases {
		mustContain(t, postOp(t, s, "T3", "/send", "text", tc.text), tc.want)
		_ = name
	}
	if n := len(s.Console.CallsOf(pathOps, "send_text")); n != 0 {
		t.Errorf("send_text = %d 回; want 0", n)
	}
	// 上限ちょうどは通る。
	postOp(t, s, "T3", "/send", "text", strings.Repeat("あ", 8000))
	if n := len(s.Console.CallsOf(pathOps, "send_text")); n != 1 {
		t.Errorf("8000 字の send_text = %d 回; want 1", n)
	}
}
