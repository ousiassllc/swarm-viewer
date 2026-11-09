package panehandler_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 最初のタスクの送信失敗・再送・排他・ログ・見るだけの接続のテスト（共通の定数・補助は first_task_test.go）。

func TestStartPost_SendFailureKeepsTheStartAndResendSendsOnlySendText(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("send_text", opsconsole.ToolError("invalid_state"))
	start := form("agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", "do it", "confirm", "1")

	w := s.Post(startPath(startPaneT), start)
	html := body(t, w, 200)
	mustContain(t, html, "起動しました。最初のタスクを送れませんでした",
		"ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください",
		"ペイン詳細を開く", "もう一度送る", `name="resend" value="1"`, `name="skill" value="/impl"`, `name="task" value="do it"`)
	mustNotContain(t, html, opsconsole.ConsoleMessage)
	if w.Header().Get("HX-Redirect") != "" {
		t.Error("送信失敗なのに HX-Redirect がある")
	}
	if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start", "send_text"}) {
		t.Fatalf("calls = %v; 自動の再送をしない", got)
	}
	if n := len(s.Console.CallsOf(pathOps, "pane_close")); n != 0 {
		t.Errorf("起動を取り消した（pane_close %d 回）", n)
	}

	resend := form("resend", "1", "skill", "/impl", "task", "do it")
	// 送る前に最新の report でペインが idle かを確かめる。shell のままなら通信せず invalid_state。
	html = body(t, s.Post(startPath(startPaneT), resend), 200)
	mustContain(t, html, "ペインの状態が変わったため、操作できませんでした", "もう一度送る")
	if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start", "send_text"}) {
		t.Fatalf("idle でない再送が通信した: %v", got)
	}

	// idle なら send_text だけを送り直す（pane_start の二重起動なし）。まだ失敗なら同じ表示に戻り、自動の再送はしない。
	s.Console.On("report", idleT4Report())
	html = body(t, s.Post(startPath(startPaneT), resend), 200)
	mustContain(t, html, "もう一度送る")
	if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start", "send_text", "send_text"}) {
		t.Fatalf("calls = %v; want 再送は send_text 1 回だけ", got)
	}

	// 通れば S4 へ。
	s.Console.On("send_text", opsconsole.Reply{Text: `{"ok":true,"state":"working","accepted":true,"chars":9}`})
	w = s.Post(startPath(startPaneT), resend)
	body(t, w, 200)
	if got := w.Header().Get("HX-Redirect"); got != opsconsole.PanePath(startPaneT, "") {
		t.Errorf("HX-Redirect = %q", got)
	}
	if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start", "send_text", "send_text", "send_text"}) {
		t.Errorf("calls = %v", got)
	}
	if last := s.Console.CallsOf(pathOps, "send_text")[2]; last.Args["text"] != "/impl do it" || last.Args["pane"] != startPaneT {
		t.Errorf("再送の引数 = %v", last.Args)
	}
}

func TestStartPost_NotAcceptedIsShownAsSendFailureWithResend(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("send_text", opsconsole.Reply{Text: `{"ok":true,"state":"idle","accepted":false,"chars":0}`})
	html := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", "x", "confirm", "1")), 200)
	mustContain(t, html, "起動しました。最初のタスクを送れませんでした", "受け付けられませんでした", "もう一度送る")
}

func TestStartPost_ResendWithEmptyOrInvalidTaskIsRejectedWithoutCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.Console.On("report", idleT4Report())
	for _, kv := range [][]string{{}, {"task", "a\x00b"}} {
		html := body(t, s.Post(startPath(startPaneT), form(append([]string{"resend", "1"}, kv...)...)), 200)
		mustContain(t, html, "起動しました。最初のタスクを送れませんでした")
		mustNotContain(t, html, "もう一度送る")
	}
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}

// pane_start から send_text までは 1 つの操作で、同じペインの排他を保つ。途中の別の操作は通信せずに断る。
func TestStartPost_HoldsThePaneExclusivelyUntilFirstTaskIsSent(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	entered, release := s.Console.Hold("send_text")
	defer release()

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		first <- s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", "x", "confirm", "1"))
	}()
	<-entered // pane_start は済み、最初のタスクの send_text が console で止まっている

	html := body(t, s.Post(opsconsole.PanePath(startPaneT, "/send"), form("text", "other")), 200)
	mustContain(t, html, "前の操作がまだ終わっていません")
	if n := len(s.Console.CallsOf(pathOps, "send_text")); n != 1 {
		t.Errorf("send_text = %d 回; want 1（割り込みは通信しない）", n)
	}
	release()
	w := <-first
	body(t, w, 200)
	if w.Header().Get("HX-Redirect") == "" {
		t.Error("HX-Redirect が無い")
	}
}

func TestStartFlow_LogsNeverContainSkillNamesInvokeOrTaskBody(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=omp")), 200)
	body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", opsconsole.SkillInvoke, "task", taskBody)), 200)
	s.Console.On("send_text", opsconsole.ToolError("invalid_state"))
	body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", opsconsole.SkillInvoke, "task", taskBody, "confirm", "1")), 200)
	s.Console.On("report", idleT4Report())
	body(t, s.Post(startPath(startPaneT), form("resend", "1", "skill", opsconsole.SkillInvoke, "task", taskBody)), 200)
	s.Console.On("skill_list", opsconsole.ToolError("unknown_tool"))
	body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=omp")), 200)

	logs := s.Logs.String()
	if !strings.Contains(logs, "tool=skill_list") || !strings.Contains(logs, "tool=send_text") {
		t.Fatalf("操作のログが無い:\n%s", logs)
	}
	for _, secret := range []string{opsconsole.SkillName, opsconsole.SkillDescription, opsconsole.SkillInvoke, taskBody, opsconsole.ConsoleMessage, opsconsole.Token} {
		if strings.Contains(logs, secret) {
			t.Errorf("ログに %q が含まれる:\n%s", secret, logs)
		}
	}
}

// 見るだけの接続: スキル・本文の部品は描画せず、skill_list も呼ばない（通信 0 件・403）。
func TestViewOnlyConnection_FirstTaskRoutesAreForbiddenWithoutContactingConsole(t *testing.T) {
	s := opsconsole.NewStack(t, pathReport)
	const msg = "この接続では操作できません（見るだけの接続です）"
	html := body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=omp")), http.StatusForbidden)
	mustContain(t, html, msg)
	mustNotContain(t, html, opsconsole.SkillName, `name="skill"`)
	html = body(t, s.Get(startPath(startPaneT)), http.StatusForbidden)
	mustNotContain(t, html, `name="task"`, `name="skill"`, "スキル")
	for _, kv := range [][]string{
		{"agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", "x", "confirm", "1"},
		{"resend", "1", "skill", "/impl", "task", "x"},
	} {
		mustContain(t, body(t, s.Post(startPath(idlePaneT), form(kv...)), http.StatusForbidden), msg)
	}
	if all := s.Console.AllRequests(); len(all) != 0 {
		t.Fatalf("偽 console への通信 = %d 件; want 0（%v）", len(all), all)
	}
}
