package panehandler_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 新しいセッションの最初のタスク（FR-OPS-11）のハンドラのテスト。

const (
	taskBody   = "TASK-BODY-MARKER-do-not-log"
	startPaneT = "T4" // shell
	idlePaneT  = "T3" // idle
)

func startPath(pos string) string { return opsconsole.PanePath(pos, "/start") }

// startToolCalls は /ops への tools/call のうち、新しいセッションに関わるツール（順序のまま）。
func startToolCalls(s *opsconsole.Stack) []string {
	var out []string
	for _, tool := range toolNames(s.Console.Calls(pathOps)) {
		if tool == "pane_start" || tool == "send_text" || tool == "skill_list" {
			out = append(out, tool)
		}
	}
	return out
}

// report の T4（shell）を idle にした応答（「もう一度送る」は idle のペインだけ）。
func idleT4Report() opsconsole.Reply {
	return opsconsole.Reply{Text: strings.Replace(opsconsole.ReportText, `"agent":null,"state":"shell"`, `"agent":"omp","state":"idle"`, 1)}
}

func TestStartScreen_HasSkillPickerAndTaskFields(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	page := body(t, s.Get(startPath(startPaneT)), 200)
	mustContain(t, page, `name="task"`, `/start/skills"`, "スキル（任意）", "本文（任意）")
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("画面を開いただけでツールを呼んだ: %v", got)
	}
	// 戻って直す: 入力済みのスキル・本文を戻し、スキルは選択済みの表示にする（一覧は取り直さない）。
	back := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", "本文です", "edit", "1")), 200)
	mustContain(t, back, "data-skill-selected", `name="skill" value="/impl"`, "本文です")
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("戻っただけでツールを呼んだ: %v", got)
	}
}

func TestStartSkills_CallsSkillListWithAgentOnly_AndRendersAsText(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=omp")), 200)
	mustContain(t, html, "スキルなし", opsconsole.SkillName, opsconsole.SkillDescription, `value="`+opsconsole.SkillInvoke+`"`, "plain",
		"&lt;b&gt;bold&lt;/b&gt;") // description は HTML として解釈しない
	mustNotContain(t, html, "<b>bold</b>")
	r := callOf(t, s, "skill_list")
	if len(r.Args) != 1 || r.Args["agent"] != "omp" {
		t.Errorf("skill_list の引数 = %v; want agent だけ（session・cwd なし）", r.Args)
	}
}

func TestStartSkills_UnsupportedAndFailureShowFixedTextAndStillAllowStartingWithoutSkill(t *testing.T) {
	t.Run("unknown_tool", func(t *testing.T) {
		s := opsconsole.NewStack(t, pathOps)
		s.Console.On("skill_list", opsconsole.ToolError("unknown_tool"))
		html := body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=claude")), 200)
		mustContain(t, html, "スキルの一覧を取得できません（ホストの swarm-console が古い可能性があります）")
		mustNotContain(t, html, "再読み込み", opsconsole.ConsoleMessage, `name="skill"`)
		// スキルなしの起動（本文だけ・タスクなし）は止めない。
		w := s.Post(startPath(startPaneT), form("agent", "claude", "cwd", "/w", "title", "t", "task", "do it", "confirm", "1"))
		body(t, w, 200)
		if got := startToolCalls(s); !slices.Equal(got, []string{"skill_list", "pane_start", "send_text"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("other failure offers reload", func(t *testing.T) {
		s := opsconsole.NewStack(t, pathOps)
		s.Console.On("skill_list", opsconsole.ToolError("tmux_error"))
		html := body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills?agent=codex")), 200)
		mustContain(t, html, "スキルの一覧を読み込めませんでした", "再読み込み", `/start/skills"`)
		mustNotContain(t, html, "古い可能性", opsconsole.ConsoleMessage)
		w := s.Post(startPath(startPaneT), form("agent", "codex", "cwd", "/w", "title", "t", "confirm", "1"))
		body(t, w, 200)
		if got := startToolCalls(s); !slices.Equal(got, []string{"skill_list", "pane_start"}) {
			t.Errorf("タスクなしの起動: calls = %v", got)
		}
	})
}

func TestStartSkills_InvalidAgentIsRejectedWithoutCommunicating(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	for _, q := range []string{"", "?agent=", "?agent=vim", "?agent=OMP"} {
		body(t, s.Get(opsconsole.PanePath(startPaneT, "/start/skills"+q)), http.StatusBadRequest)
	}
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}

func TestStartPost_ConfirmationScreenSummarizesFirstTaskAndCallsNothing(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	long := strings.Repeat("あ", 200)
	html := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", long)), 200)
	mustContain(t, html, "data-start-summary", "data-start-skill", "/impl", "data-start-task", strings.Repeat("あ", 120)+"…",
		`name="skill" value="/impl"`, `name="task" value="`+long+`"`, `name="confirm"`) // 要約は先頭だけ、hidden は全文
	_, after, _ := strings.Cut(html, "data-start-task>")
	summary, _, _ := strings.Cut(after, "</dd>")
	if summary != strings.Repeat("あ", 120)+"…" {
		t.Errorf("本文の要約 = %q", summary)
	}
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("確認画面の前にツールを呼んだ: %v", got)
	}
}

func TestStartPost_ConfirmedStartSendsFirstTaskAfterPaneStart(t *testing.T) {
	cases := []struct {
		name string
		kv   []string
		want string
	}{
		{"skill and task", []string{"skill", "/impl", "task", "#12 を直す"}, "/impl #12 を直す"},
		{"skill only", []string{"skill", "/impl"}, "/impl"},
		{"task only (CRLF is normalized)", []string{"task", "a\r\nb"}, "a\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			kv := append([]string{"agent", "omp", "cwd", "/w", "title", "t", "confirm", "1"}, tc.kv...)
			w := s.Post(startPath(startPaneT), form(kv...))
			body(t, w, 200)
			if got := w.Header().Get("HX-Redirect"); got != opsconsole.PanePath(startPaneT, "") {
				t.Errorf("HX-Redirect = %q", got)
			}
			if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start", "send_text"}) {
				t.Fatalf("calls = %v; want pane_start → send_text", got)
			}
			r := callOf(t, s, "send_text")
			if r.Args["text"] != tc.want || r.Args["pane"] != startPaneT || r.Args["session"] != "0" {
				t.Errorf("send_text の引数 = %v; want text %q", r.Args, tc.want)
			}
		})
	}
}

func TestStartPost_WithoutFirstTaskOnlyStarts(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	w := s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", "  \r\n ", "confirm", "1"))
	body(t, w, 200)
	if w.Header().Get("HX-Redirect") == "" {
		t.Error("HX-Redirect が無い")
	}
	if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start"}) {
		t.Errorf("calls = %v; want pane_start だけ", got)
	}
}

func TestStartPost_FirstTaskIsNotSentWhenStartIsNotOkOrNotIdle(t *testing.T) {
	t.Run("ok:false", func(t *testing.T) {
		s := opsconsole.NewStack(t, pathOps)
		s.Console.On("pane_start", opsconsole.Reply{Text: `{"ok":false,"state":"shell"}`})
		w := s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", "x", "confirm", "1"))
		html := body(t, w, 200)
		mustContain(t, html, "起動を確認できませんでした（状態: shell）", "data-start-summary")
		if w.Header().Get("HX-Redirect") != "" {
			t.Error("失敗なのに HX-Redirect がある")
		}
		if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("pane_start error", func(t *testing.T) {
		s := opsconsole.NewStack(t, pathOps)
		s.Console.On("pane_start", opsconsole.ToolError("forbidden_path"))
		html := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", "x", "confirm", "1")), 200)
		mustContain(t, html, "このフォルダは選べません")
		if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v", got)
		}
	})
	t.Run("not idle", func(t *testing.T) {
		s := opsconsole.NewStack(t, pathOps)
		s.Console.On("pane_start", opsconsole.Reply{Text: `{"ok":true,"state":"working","agent":"omp","cwd":"/w","title":"t"}`})
		w := s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", taskBody, "confirm", "1"))
		html := body(t, w, 200)
		mustContain(t, html, "起動しました。最初のタスクはまだ送っていません（状態: working）。ペイン詳細から送れます",
			"ペイン詳細を開く", `hx-get="`+opsconsole.PanePath(startPaneT, "")+`"`)
		mustNotContain(t, html, "もう一度送る", `name="resend"`, taskBody)
		if w.Header().Get("HX-Redirect") != "" {
			t.Error("idle 以外なのに HX-Redirect がある（トーストが消えて知らせられない）")
		}
		if got := startToolCalls(s); !slices.Equal(got, []string{"pane_start"}) {
			t.Errorf("calls = %v; want send_text なし", got)
		}
	})
}

func TestStartPost_InvalidFirstTaskIsRejectedBeforePaneStart(t *testing.T) {
	for name, task := range map[string]string{
		"control char": "a\x1bb",
		"too long":     strings.Repeat("あ", 8001),
	} {
		for _, confirm := range []string{"", "1"} {
			t.Run(name+" confirm="+confirm, func(t *testing.T) {
				s := opsconsole.NewStack(t, pathOps)
				html := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "task", task, "confirm", confirm)), 200)
				want := "使えない制御文字が含まれています（改行とタブだけ使えます）"
				if name == "too long" {
					want = "長すぎます（8000 字まで）"
				}
				mustContain(t, html, want, `name="task"`)
				mustNotContain(t, html, "data-start-summary")
				if got := startToolCalls(s); len(got) != 0 {
					t.Errorf("calls = %v; want なし", got)
				}
			})
		}
	}
	// スキルと本文の合計が上限を超える場合も同じ。
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Post(startPath(startPaneT), form("agent", "omp", "cwd", "/w", "title", "t", "skill", "/impl", "task", strings.Repeat("a", 7996), "confirm", "1")), 200)
	mustContain(t, html, "長すぎます")
	if got := startToolCalls(s); len(got) != 0 {
		t.Errorf("calls = %v; want なし", got)
	}
}
