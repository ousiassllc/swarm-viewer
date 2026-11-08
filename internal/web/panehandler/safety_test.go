package panehandler_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 同じペインの操作が同時に 2 つ走らない。2 つ目は通信せずに「前の操作がまだ終わっていません」。
func TestSamePaneOperationsDoNotRunConcurrently_SecondIsRefusedWithoutCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	entered, release := s.Console.Hold("send_text")
	defer release()

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- s.Post(opsconsole.PanePath("T3", "/send"), form("text", "one")) }()
	<-entered // 1 つ目が console で止まっている

	// 同じペインへの別の操作（種類が違っても）は断る。別のペインは通る。
	for _, p := range []struct{ path, key, val string }{{"/send", "text", "two"}, {"/escape", "confirm", "true"}, {"/pick", "n", "1"}} {
		html := body(t, s.Post(opsconsole.PanePath("T3", p.path), form(p.key, p.val)), 200)
		mustContain(t, html, "前の操作がまだ終わっていません")
	}
	if n := len(s.Console.CallsOf(pathOps, "send_text")); n != 1 {
		t.Errorf("send_text = %d 回; want 1（2 つ目は通信しない）", n)
	}
	if n := len(s.Console.CallsOf(pathOps, "send_escape")) + len(s.Console.CallsOf(pathOps, "ask_pick")); n != 0 {
		t.Errorf("同じペインの別操作が通信した（%d 回）", n)
	}
	body(t, s.Post(opsconsole.PanePath("T1", "/escape"), form("confirm", "true")), 200)
	if n := len(s.Console.CallsOf(pathOps, "send_escape")); n != 1 {
		t.Errorf("別のペインの send_escape = %d 回; want 1", n)
	}

	release()
	mustContain(t, body(t, <-first, 200), "送りました")
	// 終わったあとは、同じペインへもう一度送れる。
	body(t, s.Post(opsconsole.PanePath("T3", "/send"), form("text", "three")), 200)
	if n := len(s.Console.CallsOf(pathOps, "send_text")); n != 2 {
		t.Errorf("終了後の send_text = %d 回; want 合計 2", n)
	}
}

// 401・403 はその接続の自動更新と操作を止める（needs_repair）。以後、コンソールには通信しない。
func TestUnauthorizedStopsPollingAndFurtherOperations(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	body(t, s.Get(opsconsole.PanePath("T3", "/live")), 200) // 正常
	s.Console.RejectAuth()

	html := body(t, s.Get(opsconsole.PanePath("T3", "/live")), 200)
	mustContain(t, html, "認証できません。再ペアしてください", "再ペアリング", `href="/pairing"`)
	mustNotContain(t, html, "hx-trigger", `hx-get="`+opsconsole.PanePath("T3", "/live"), "<form", "<textarea", "hx-post")

	before := len(s.Console.AllRequests())
	html = body(t, s.Post(opsconsole.PanePath("T3", "/send"), form("text", "hi")), 200)
	mustContain(t, html, "認証できません。再ペアしてください")
	body(t, s.Get(opsconsole.PanePath("T3", "/live")), 200)
	if after := len(s.Console.AllRequests()); after != before {
		t.Errorf("needs_repair のあとに通信した（%d 件）", after-before)
	}
}

// 取得に失敗したら、直前の表示を残し（HX-Reswap: none）、固定文言のバナーだけを OOB で差し替える。
func TestLiveFailureKeepsPreviousViewAndShowsBannerOnly(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("pane_tail", opsconsole.ToolError("tmux_error"))

	w := s.Get(opsconsole.PanePath("T3", "/live"))
	html := body(t, w, 200)
	if got := w.Header().Get("HX-Reswap"); got != "none" {
		t.Errorf("HX-Reswap = %q; want none", got)
	}
	mustContain(t, html, `id="pane-banner" hx-swap-oob="true"`, "ホスト側でエラーが発生しました")
	mustNotContain(t, html, "pane-live", "data-pane-tail", opsconsole.ConsoleMessage)

	// 最初の読み込み（first=1）は、直前の表示が無いので本文ごと返す（バナー付き。操作の部品は出さない）。
	html = body(t, s.Get(opsconsole.PanePath("T3", "/live?first=1")), 200)
	mustContain(t, html, `id="pane-live"`, "ホスト側でエラーが発生しました", `hx-trigger="every 5s, refresh from:body"`)
	mustNotContain(t, html, "<form", "hx-post", "<textarea")
}

// ログに token・送信本文・pane_tail の本文・ask_card の本文を出さない（操作の成功・失敗の両方）。
func TestLogsNeverContainTokenSendBodyOrPaneText(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	const sendBody = "SEND-BODY-MARKER-do-not-log"
	const pickBody = "PICK-TEXT-MARKER-do-not-log"

	live(t, s, "T2") // pane_tail・ask_card・report
	postOp(t, s, "T3", "/send", "text", sendBody)
	postOp(t, s, "T2", "/pick", "text", pickBody)
	postOp(t, s, "T1", "/escape", "confirm", "true")
	s.Console.On("send_text", opsconsole.ToolError("invalid_state"))
	postOp(t, s, "T3", "/send", "text", sendBody) // 失敗
	s.Console.On("pane_tail", opsconsole.ToolError("tmux_error"))
	live(t, s, "T3") // 取得失敗

	logs := s.Logs.String()
	if !strings.Contains(logs, "pane operation done") || !strings.Contains(logs, "pane call failed") {
		t.Fatalf("操作のログが無い:\n%s", logs)
	}
	for _, secret := range []string{opsconsole.Token, sendBody, pickBody, opsconsole.TailSecret, opsconsole.QuestionSecret, opsconsole.ConsoleMessage, "Bearer"} {
		if strings.Contains(logs, secret) {
			t.Errorf("ログに %q が含まれる:\n%s", secret, logs)
		}
	}
	// ログに残すのは、接続 ID・ツール名・種別・コード・ペイン位置まで。
	mustContain(t, logs, "connection_id=c1", "tool=send_text", "kind=operation", "code=invalid_state", "pane=T3")
}
