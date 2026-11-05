package molecules_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/molecules"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func str(s string) *string { return &s }

func TestPaneCard_StateLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state domain.PaneState
		label string
	}{
		{domain.StateWorking, "作業中"},
		{domain.StateAsking, "答え待ち"},
		{domain.StateIdle, "指示待ち"},
		{domain.StateStalled, "止まってるかも"},
		{domain.StateShell, "空き"},
		{domain.StateUnknown, "判定できない"},
		{domain.PaneState("brand-new"), "判定できない"},
	}
	for _, tt := range tests {
		html := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: domain.Pane{Pos: "T1", State: tt.state, Title: "t"}}))
		webtest.MustContain(t, html, tt.label)
		if tt.state == "brand-new" {
			webtest.MustNotContain(t, html, "brand-new")
		}
	}
}

// ペインカードは読み取り専用（FR-PANE・機能要件 §4.4）。
func TestPaneCard_HasNoActionableElements(t *testing.T) {
	t.Parallel()
	pane := domain.Pane{
		Pos: "B2", ID: "%12", Agent: str("claude"), State: domain.StateAsking, Title: "title", CWD: "/work",
		Elapsed: "1h05m", Activity: "running", Note: &domain.Note{Repo: "repo", Items: []string{"a"}},
	}
	for _, stale := range []bool{false, true} {
		html := strings.ToLower(webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane, Stale: stale, Highlight: true})))
		webtest.MustNotContain(t, html, "<button", "<form", "<input", "<a ", "<select", "<textarea", "hx-post", "hx-put", "hx-delete", "hx-get", "hx-patch", "onclick")
	}
}

func TestPaneCard_Fields(t *testing.T) {
	t.Parallel()
	pane := domain.Pane{
		Pos: "T3", Agent: str("codex"), State: domain.StateStalled, Title: "My Title", CWD: "/srv/work",
		Elapsed: "42m", Activity: "compiling", ActivityTruncated: true, ChangedAgo: "17m",
		Note: &domain.Note{Repo: "org/repo", Items: []string{"one", "two", "three", "four", "five"}},
	}
	html := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane}))
	webtest.MustContain(t, html, "T3", "codex", "My Title", "/srv/work", "42m", "compiling…", "17m", "org/repo", "one", "three", "+2")
	// 本文は先頭 3 件のみ。全文は title（ツールチップ）にだけある。
	webtest.MustNotContain(t, html, "<li class=\"truncate\">four", "<li class=\"truncate\">five")
}

func TestPaneCard_OptionalFieldsHidden(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: domain.Pane{Pos: "T1", State: domain.StateIdle, Title: "x", ChangedAgo: "5m"}}))
	// agent なし・elapsed なし・activity なし・note なし。changed_ago は stalled 以外では出さない。
	webtest.MustNotContain(t, html, "5m", "<ul", "<p class=\"truncate text-xs")
}

func TestPaneCard_StaleAndHighlightClasses(t *testing.T) {
	t.Parallel()
	pane := domain.Pane{Pos: "T1", State: domain.StateWorking, Title: "x"}
	normal := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane}))
	stale := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane, Stale: true}))
	hi := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane, Highlight: true}))
	webtest.MustNotContain(t, normal, "opacity-50", "ring-2 ring-warning")
	webtest.MustContain(t, stale, "opacity-50")
	webtest.MustContain(t, hi, "ring-2 ring-warning")
}

// title・activity・note は信頼できない入力。HTML として解釈しない（NFR-SEC-10）。
func TestPaneCard_EscapesUntrustedText(t *testing.T) {
	t.Parallel()
	evil := `<script>alert(1)</script><img src=x onerror=alert(2)>`
	pane := domain.Pane{
		Pos: "T1", State: domain.StateWorking, Title: evil, CWD: evil, Activity: evil, Agent: str(evil),
		Note: &domain.Note{Repo: evil, Items: []string{evil}},
	}
	html := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane}))
	if len(webtest.Elements(t, html, "script")) > 0 || len(webtest.Elements(t, html, "img")) > 0 {
		t.Errorf("信頼できない入力が要素として解釈されている:\n%s", html)
	}
	if keys := webtest.AllAttrKeys(t, html); keys["onerror"] || keys["src"] {
		t.Errorf("信頼できない入力が属性として解釈されている: %v", keys)
	}
	webtest.MustContain(t, html, "&lt;script&gt;")
}

func TestPaneCard_ReportWaitingBadge(t *testing.T) {
	t.Parallel()
	pane := domain.Pane{Pos: "T1", State: domain.StateIdle, Title: "t"}

	html := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane, ReportWaiting: true}))
	webtest.MustContain(t, html, "報告待ち", "指示待ち")
	if strings.Index(html, "指示待ち") > strings.Index(html, "報告待ち") {
		t.Error("「報告待ち」は state バッジの後に出る")
	}
	// report_waiting は枠色で強調しない（強調は asking / stalled だけ）。
	webtest.MustNotContain(t, html, "ring-2 ring-warning")

	html = webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane}))
	webtest.MustContain(t, html, "指示待ち")
	webtest.MustNotContain(t, html, "報告待ち")
}
