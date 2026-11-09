package panehandler_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 終了は idle・shell だけ。ペインの今の状態が違えば、画面が古くても通信せず invalid_state の固定文言。
func TestClosePost_RefusedByServerWhenPaneStateDoesNotAllowItWithoutCommunicating(t *testing.T) {
	for _, pos := range []string{"T1", "T2", "T5"} { // working・asking・stalled
		t.Run(pos, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			w := s.Post(opsconsole.PanePath(pos, "/close"), form("confirm", "true"))
			html := body(t, w, 200)
			mustContain(t, html, "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください")
			if w.Header().Get("HX-Redirect") != "" {
				t.Error("拒否したのにメイン画面へ移動している")
			}
			if n := len(s.Console.CallsOf(pathOps, "pane_close")); n != 0 {
				t.Errorf("pane_close = %d 回; want 0", n)
			}
		})
	}
	for _, pos := range []string{"T3", "T4"} { // idle・shell は通る
		s := opsconsole.NewStack(t, pathOps)
		body(t, s.Post(opsconsole.PanePath(pos, "/close"), form("confirm", "true")), 200)
		if n := len(s.Console.CallsOf(pathOps, "pane_close")); n != 1 {
			t.Errorf("%s: pane_close = %d 回; want 1", pos, n)
		}
	}
}

func TestClosePost_UnknownPaneIsRefusedWithoutCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	mustContain(t, body(t, s.Post(opsconsole.PanePath("T9", "/close"), form("confirm", "true")), 200), "ペインが見つかりません")
	if n := len(s.Console.CallsOf(pathOps, "pane_close")); n != 0 {
		t.Errorf("pane_close = %d 回", n)
	}
}

// 起動は shell だけ。確認済みの POST でも、ペインの今の状態が shell でなければ通信しない。
func TestStartPost_RefusedByServerWhenPaneIsNotShellWithoutCommunicating(t *testing.T) {
	for _, pos := range []string{"T1", "T2", "T3", "T5", "T9"} {
		t.Run(pos, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			html := body(t, s.Post(opsconsole.PanePath(pos, "/start"), form("agent", "omp", "cwd", "/work", "title", "t", "confirm", "1")), 200)
			want := "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください"
			if pos == "T9" {
				want = "ペインが見つかりません"
			}
			mustContain(t, html, want)
			if n := len(s.Console.CallsOf(pathOps, "pane_start")); n != 0 {
				t.Errorf("pane_start = %d 回; want 0", n)
			}
		})
	}
}

// 自動更新で作り直されても、「その他」の入力は消えない（hx-preserve。id は質問ごと）。送信が通ればカードが閉じて消える。
func TestCustomAnswerIsPreservedAcrossPollsAndResetPerQuestion(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	first := live(t, s, "T2")
	mustContain(t, first, `hx-preserve`, `<details id="pane-custom-answer-`)
	idOf := func(html string) string {
		i := strings.Index(html, `<details id="`) + len(`<details id="`)
		return html[i : i+strings.Index(html[i:], `"`)]
	}
	if idOf(live(t, s, "T2")) != idOf(first) {
		t.Error("同じ質問なのに id が変わる（保持されない）")
	}
	s.Console.On("ask_card", opsconsole.Reply{Text: `{"asking":true,"question":"別の質問","options":[{"n":1,"label":"a"}],"cursor":1,"allows_custom":true}`})
	if idOf(live(t, s, "T2")) == idOf(first) {
		t.Error("別の質問なのに id が同じ（前の入力を持ち越す）")
	}
	// 選択済み（カードが閉じた）後の本文には、その他の欄が無い。
	s.Console.On("ask_card", opsconsole.Reply{Text: `{"asking":false}`})
	mustNotContain(t, live(t, s, "T2"), "pane-custom-answer-")
}

// 押した操作は、飛行中の自動更新を中断して自分を通す。コンテナの自動更新は this:drop のまま。
func TestOperationSourcesPreemptPollWhileContainerDropsOverlappingPolls(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	const preempt = `hx-sync="#pane-live:replace"`
	// 状態ごとの操作元: 選択肢・その他・更新・もっと見る・中断・終了・送信欄。
	for pos, wantN := range map[string]int{"T2": 4, "T3": 4} {
		html := live(t, s, pos)
		if n := strings.Count(html, preempt); n < wantN {
			t.Errorf("%s: 先取りの hx-sync = %d 個; want %d 以上\\n%s", pos, n, wantN, html)
		}
		mustContain(t, html, `hx-sync="this:drop"`)
		if n := strings.Count(html, `hx-sync="this:drop"`); n != 1 {
			t.Errorf("%s: this:drop = %d 個; want コンテナの 1 個だけ", pos, n)
		}
	}
	html := live(t, s, "T3")
	for _, label := range []string{">更新<", ">もっと見る", ">中断<", ">終了<"} {
		i := strings.Index(html, label)
		if i < 0 {
			t.Fatalf("%s が無い", label)
		}
		if !strings.Contains(html[strings.LastIndex(html[:i], "<button"):i], preempt) {
			t.Errorf("%s のボタンに %s が無い", label, preempt)
		}
	}
	// shell のペイン（T4）の「＋ 始める」リンク。
	html = live(t, s, "T4")
	i := strings.Index(html, "＋ 始める")
	if i < 0 {
		t.Fatal("＋ 始める が無い")
	}
	if !strings.Contains(html[strings.LastIndex(html[:i], "<a"):i], preempt) {
		t.Errorf("＋ 始める のリンクに %s が無い", preempt)
	}
	// 質問カード（T2）の中: 選択肢ボタンと「その他」のフォームのどちらにも付く。
	html = live(t, s, "T2")
	card := html[strings.Index(html, "data-ask-card"):]
	card = card[:strings.Index(card, "</section>")]
	if n := strings.Count(card, preempt); n < 3 {
		t.Errorf("質問カード内の先取りの hx-sync = %d 個; want 3 以上（選択肢 2 つ以上 + その他）", n)
	}
}

// 終了は確認ダイアログを経た（confirm=true の）POST だけ通る。無ければ pane_close を呼ばず confirm_required の固定文言。
func TestClosePost_WithoutConfirmationIsRefusedWithoutCommunicating(t *testing.T) {
	for name, f := range map[string]func() url.Values{
		"フォーム無し":        func() url.Values { return nil },
		"confirm=false": func() url.Values { return form("confirm", "false") },
		"confirm=1":     func() url.Values { return form("confirm", "1") },
	} {
		t.Run(name, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			w := s.Post(opsconsole.PanePath("T3", "/close"), f())
			html := body(t, w, 200)
			mustContain(t, html, "確認が必要です")
			if w.Header().Get("HX-Redirect") != "" {
				t.Error("拒否したのにメイン画面へ移動している")
			}
			if n := len(s.Console.CallsOf(pathOps, "pane_close")); n != 0 {
				t.Errorf("pane_close = %d 回; want 0", n)
			}
		})
	}
}

// 終了ボタンは確認ダイアログを経て confirm=true を付けて送る（「もう一度試す」も同じボタン）。
func TestCloseButtonSendsConfirmationValueThroughDialog(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := live(t, s, "T3")
	i := strings.Index(html, ">終了<")
	btn := html[strings.LastIndex(html[:i], "<button"):i]
	mustContain(t, btn, "hx-confirm=", `&#34;confirm&#34;:&#34;true&#34;`)
}

// 「その他」は保持される <details> の中に lines の入力を持たない（古い値が残るため）。最新の lines は外の hidden を hx-include で送る。
func TestCustomAnswerSendsCurrentLinesNotPreservedStaleValue(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	for _, lines := range []string{"30", "100"} {
		html := body(t, s.Get(opsconsole.PanePath("T2", "/live")+"?lines="+lines), 200)
		a := strings.Index(html, "<details id=\"pane-custom-answer-")
		b := a + strings.Index(html[a:], "</details>")
		if a < strings.Index(html, "<details") || strings.Contains(html[a:b], `name="lines"`) {
			t.Errorf("lines=%s: 保持される <details> の中に name=\"lines\" がある\n%s", lines, html[a:b])
		}
		mustContain(t, html,
			`<input type="hidden" id="pane-custom-lines" name="lines" value="`+lines+`"`,
			`hx-include="#pane-custom-lines"`)
		if strings.Index(html, `id="pane-custom-lines"`) > a {
			t.Error("lines の hidden が <details> の外（前）に無い")
		}
	}
}
