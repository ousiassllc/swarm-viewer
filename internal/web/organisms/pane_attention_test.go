package organisms_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func TestPaneGrid_Layout(t *testing.T) {
	t.Parallel()
	panes := []domain.Pane{{Pos: "T1", State: domain.StateIdle}, {Pos: "T2", State: domain.StateIdle}, {Pos: "B1", State: domain.StateIdle}}
	html := webtest.Render(t, organisms.PaneGrid(organisms.PaneGridProps{Layout: domain.Layout{Rows: 2, Cols: []int{2, 1}}, Panes: panes}))
	webtest.MustContain(t, html, `data-layout="grid"`)
	if strings.Count(html, `<div class="flex gap-3">`) != 2 {
		t.Errorf("段の数が layout と一致しない:\n%s", html)
	}
	if strings.Index(html, "T1") > strings.Index(html, "T2") || strings.Index(html, "T2") > strings.Index(html, "B1") {
		t.Error("ペイン順が崩れている")
	}
}

func TestPaneGrid_InvalidLayoutFallsBackToList(t *testing.T) {
	t.Parallel()
	panes := []domain.Pane{{Pos: "T1", State: domain.StateIdle}, {Pos: "T2", State: domain.StateIdle}, {Pos: "B1", State: domain.StateIdle}}
	for name, layout := range map[string]domain.Layout{
		"欠落（零値）":          {},
		"列数の合計が合わない":      {Rows: 2, Cols: []int{2, 2}},
		"rows と Cols の長さ": {Rows: 3, Cols: []int{2, 1}},
		"0 列の段":           {Rows: 2, Cols: []int{3, 0}},
	} {
		html := webtest.Render(t, organisms.PaneGrid(organisms.PaneGridProps{Layout: layout, Panes: panes}))
		webtest.MustContain(t, html, `data-layout="list"`, "T1", "T2", "B1")
		webtest.MustNotContain(t, html, `data-layout="grid"`)
		if strings.Count(html, "<li>") != 3 {
			t.Errorf("%s: リストが 3 件になっていない", name)
		}
	}
}

func TestPaneGrid_Empty(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.PaneGrid(organisms.PaneGridProps{}))
	webtest.MustContain(t, html, "ペインがありません", "border-dashed", "rounded-lg", "<svg")
}

func TestPaneGrid_HighlightsOnlyAskingAndStalled(t *testing.T) {
	t.Parallel()
	panes := []domain.Pane{{Pos: "T1", State: domain.StateAsking}, {Pos: "T2", State: domain.StateStalled}, {Pos: "T3", State: domain.StateWorking}, {Pos: "T4", State: domain.StateIdle}}
	att := []domain.Attention{
		{Kind: "asking", Pane: "T1"}, {Kind: "stalled", Pane: "T2"},
		{Kind: "report_waiting", Pane: "T3"}, {Kind: "auth_expired", Detail: "claude"},
	}
	html := webtest.Render(t, organisms.PaneGrid(organisms.PaneGridProps{Panes: panes, Attention: att}))
	if strings.Count(html, "ring-2 ring-warning") != 2 {
		t.Errorf("強調は asking と stalled の 2 枚だけ: %d", strings.Count(html, "ring-2 ring-warning"))
	}
}

func TestAttentionBar(t *testing.T) {
	t.Parallel()
	if got := strings.TrimSpace(webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{}))); got != "" {
		t.Errorf("空のときは何も描画しない: %q", got)
	}
	exp := time.Date(2026, 11, 4, 8, 0, 0, 0, time.Local)
	html := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{
			{Kind: "asking", Pane: "T2", Detail: "q"},
			{Kind: "auth_expiring", Detail: "claude"},
			{Kind: "auth_expiring", Detail: "codex"},
		},
		Warnings: []domain.Warning{{Code: "layout_changed", Message: "レイアウトが変わりました"}},
		Auth:     domain.Auth{Agents: []domain.AuthAgent{{Agent: "claude", ExpiresAt: &exp}, {Agent: "codex"}}},
	}))
	webtest.MustContain(t, html, "レイアウトが変わりました", `data-warning-code="layout_changed"`, "答え待ち", "期限 2026/11/04 08:00")
	if strings.Count(html, "期限 ") != 1 {
		t.Error("expires_at がある agent だけ期限を併記する")
	}
}

func TestAttentionBar_WarningsOnlyAndAttentionOnly(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{Warnings: []domain.Warning{{Code: "tmux_unavailable", Message: "m"}}}))
	webtest.MustContain(t, html, "m")
	webtest.MustNotContain(t, html, "divide", "rounded-lg border border-border bg-surface")
	html = webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{Attention: []domain.Attention{{Kind: "x_new", Pane: "T1"}}}))
	webtest.MustContain(t, html, "x_new")
	webtest.MustNotContain(t, html, `role="alert"`)
}

// 複数の警告（枠つきの独立した部品）は部品間の最小 8px（gap-2）で並べる。4px では枠が接して 1 つの塊に見える（ガイド S2）。
func TestAttentionBar_WarningsAreSeparatedBy8px(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{Warnings: []domain.Warning{
		{Code: "a", Message: "m1"}, {Code: "b", Message: "m2"},
	}}))
	webtest.MustContain(t, html, `<div class="flex flex-col gap-2">`, `data-warning-code="a"`, `data-warning-code="b"`)
	webtest.MustNotContain(t, html, `<div class="flex flex-col gap-1"><div role="alert"`)
}

func TestSessionTabs(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SessionTabs(organisms.SessionTabsProps{
		ConnectionID: "c1", Selected: "1",
		Sessions: []domain.Session{{ID: "0", Allowed: true}, {ID: "1", Allowed: true}, {ID: "hidden", Allowed: false}},
	}))
	webtest.MustContain(t, html, `hx-get="/connections/c1/sessions/0"`, `hx-get="/connections/c1/sessions/1"`)
	// タブ間は 24px（ガイドのタブ見本。S2）。
	webtest.MustContain(t, html, "gap-6")
	webtest.MustNotContain(t, html, "hidden")
	if strings.Count(html, "aria-current") != 1 {
		t.Error("選択中は 1 つだけ")
	}
	// セッションが 1 つだけでもタブを出す。
	one := webtest.Render(t, organisms.SessionTabs(organisms.SessionTabsProps{ConnectionID: "c1", Selected: "0", Sessions: []domain.Session{{ID: "0", Allowed: true}}}))
	webtest.MustContain(t, one, "/connections/c1/sessions/0")
}
