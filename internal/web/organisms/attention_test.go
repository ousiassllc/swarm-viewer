package organisms_test

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func TestPaneGrid_ReportWaitingBadgeOnlyOnMatchingPane(t *testing.T) {
	t.Parallel()
	panes := []domain.Pane{{Pos: "T1", State: domain.StateIdle}, {Pos: "T2", State: domain.StateIdle}, {Pos: "T3", State: domain.StateIdle}}
	att := []domain.Attention{
		{Kind: "report_waiting", Pane: "T1", Detail: "/srv/secret/path.md"},
		{Kind: "report_waiting", Pane: ""}, // pane が空の項目はどのカードにも付けない
		{Kind: "asking", Pane: "T3"},       // report_waiting 以外では付けない
	}
	html := webtest.Render(t, organisms.PaneGrid(organisms.PaneGridProps{Panes: panes, Attention: att}))
	if got := strings.Count(html, "報告待ち"); got != 1 {
		t.Fatalf("「報告待ち」は T1 のカードだけ: %d", got)
	}
	cards := webtest.Elements(t, html, "article")
	if len(cards) != 3 {
		t.Fatalf("カードが 3 枚ない: %d", len(cards))
	}
	for _, c := range cards {
		pos, _ := webtest.Attr(c, "data-pane-pos")
		if got := strings.Contains(nodeText(c), "報告待ち"); got != (pos == "T1") {
			t.Errorf("%s: 「報告待ち」の有無が違う: %v", pos, got)
		}
	}
	// detail（サーバー上のファイルパス）はカードに出さない。
	webtest.MustNotContain(t, html, "/srv/secret/path.md")
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func TestAttentionBar_SplitsPaneAndAgentGroups(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 9, 5, 17, 49, 0, 0, time.Local)
	// API の順は保証がないので、混ぜて渡しても 2 グループに分かれ、グループ内は決めた順になる。
	html := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{
			{Kind: "auth_expiring", Detail: "codex"},
			{Kind: "report_waiting", Pane: "B1", Detail: "/srv/x.md"},
			{Kind: "auth_expired", Detail: "claude"},
			{Kind: "asking", Pane: "T2", Detail: "q"},
			{Kind: "brand_new_kind", Detail: "cursor"},
			{Kind: "stalled", Pane: "T1"},
			{Kind: "auth_expiring", Detail: "claude"},
		},
		Auth: domain.Auth{Agents: []domain.AuthAgent{{Agent: "codex", ExpiresAt: &exp}}},
	}))
	webtest.MustContain(t, html, `data-attention-group="panes"`, `data-attention-group="agents"`, "brand_new_kind", "期限 2026/09/05 17:49")
	webtest.MustNotContain(t, html, "/srv/x.md")
	// グループ間もエージェント間も同じ線（--border。透過で薄めない。ガイド C2・C8）。
	webtest.MustContain(t, html, "divide-y divide-border")
	webtest.MustNotContain(t, html, "divide-border/")

	// ペイン関連 → ログイン関連の順。
	if strings.Index(html, `data-attention-group="panes"`) > strings.Index(html, `data-attention-group="agents"`) {
		t.Error("ペイン関連がログイン関連より先に出る")
	}
	// ペイン関連は T1 → T2 → B1。
	idx := func(s string) int { return strings.Index(html, s) }
	if idx(">T1<") >= idx(">T2<") || idx(">T2<") >= idx(">B1<") {
		t.Error("ペイン関連はペイン位置順（T1, T2, B1）")
	}
	// エージェントごとに別ブロック。名前順で、同じエージェントの項目は 1 ブロックにまとまる。
	if got := strings.Count(html, "data-attention-agent="); got != 3 {
		t.Fatalf("エージェントのブロックは 3 つ: %d", got)
	}
	if idx(`data-attention-agent="claude"`) >= idx(`data-attention-agent="codex"`) || idx(`data-attention-agent="codex"`) >= idx(`data-attention-agent="cursor"`) {
		t.Error("エージェントは名前順（claude, codex, cursor）")
	}
	// テキストリストの項目間は 8px（ガイド §4 リスト）。
	webtest.MustContain(t, html, `<ul class="flex flex-col gap-2 px-4 py-3" data-attention-group="panes">`, `<ul class="flex flex-col gap-2 px-4 py-3" data-attention-agent="claude">`)
	webtest.MustNotContain(t, html, "gap-1")
	claude := html[idx(`data-attention-agent="claude"`):idx(`data-attention-agent="codex"`)]
	if strings.Count(claude, "<li") != 2 {
		t.Errorf("claude の 2 件は 1 ブロックにまとまる: %q", claude)
	}
}

func TestAttentionBar_NoDividerWithSingleGroup(t *testing.T) {
	t.Parallel()
	paneOnly := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{{Kind: "asking", Pane: "T1"}, {Kind: "stalled", Pane: "T2"}},
	}))
	webtest.MustContain(t, paneOnly, `data-attention-group="panes"`)
	webtest.MustNotContain(t, paneOnly, "divide-y", `data-attention-group="agents"`)

	// ログイン関連だけで 1 エージェントなら、線は出ない。
	oneAgent := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{{Kind: "auth_expired", Detail: "claude"}, {Kind: "auth_expiring", Detail: "claude"}},
	}))
	webtest.MustContain(t, oneAgent, `data-attention-group="agents"`)
	webtest.MustNotContain(t, oneAgent, "divide-y", `data-attention-group="panes"`)

	// ログイン関連だけで複数エージェントなら、エージェント間にだけ線が出る。
	twoAgents := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{{Kind: "auth_expired", Detail: "claude"}, {Kind: "auth_expiring", Detail: "codex"}},
	}))
	webtest.MustContain(t, twoAgents, "divide-y divide-border")
	webtest.MustNotContain(t, twoAgents, "divide-border/")
	webtest.MustNotContain(t, twoAgents, "divide-y divide-border ")
}

func TestAttentionBar_AuthKindWithPaneStaysInAgentGroup(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.AttentionBar(organisms.AttentionBarProps{
		Attention: []domain.Attention{{Kind: "auth_expired", Pane: "T1", Detail: "claude"}},
	}))
	webtest.MustContain(t, html, `data-attention-group="agents"`, `data-attention-agent="claude"`)
	webtest.MustNotContain(t, html, `data-attention-group="panes"`)
}
