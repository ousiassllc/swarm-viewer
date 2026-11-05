package molecules_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/molecules"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

var now = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func f(v float64) *float64 { return &v }

func TestUsageRow_Percent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		used float64
		want string // バーのクラスに含まれる色
	}{
		{"79.9", 79.9, "[&amp;::-webkit-progress-value]:bg-accent"},
		{"80", 80, "[&amp;::-webkit-progress-value]:bg-warning"},
		{"94.9", 94.9, "[&amp;::-webkit-progress-value]:bg-warning"},
		{"95", 95, "[&amp;::-webkit-progress-value]:bg-danger"},
	}
	for _, tt := range tests {
		slot := domain.Slot{Available: true, Used: f(tt.used), Unit: domain.UnitPercent, Status: "ok"}
		html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "5 時間枠", Slot: slot, Now: now}))
		webtest.MustContain(t, html, "<progress", tt.want, "5 時間枠")
		if tt.name == "79.9" {
			webtest.MustNotContain(t, html, "[&amp;::-webkit-progress-value]:bg-warning", "[&amp;::-webkit-progress-value]:bg-danger")
		}
	}
}

func TestUsageRow_Exhausted(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(10), Unit: domain.UnitPercent, Status: domain.SlotStatusExhausted}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "x", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "[&amp;::-webkit-progress-value]:bg-danger", "上限到達")
}

func TestUsageRow_USDHasAmountOnly(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(512.5), Unit: domain.UnitUSD, Status: "ok"}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "月間", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "$512.50")
	webtest.MustNotContain(t, html, "<progress", "[&amp;::-webkit-progress-value]:bg-warning", "[&amp;::-webkit-progress-value]:bg-danger", "%")
}

func TestUsageRow_USDExhaustedIsError(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(1), Unit: domain.UnitUSD, Status: domain.SlotStatusExhausted}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "月間", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "text-danger", "上限到達")
}

func TestUsageRow_UnavailableShowsDash(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "7 日枠", Slot: domain.Slot{}, Now: now}))
	webtest.MustContain(t, html, "—")
	webtest.MustNotContain(t, html, "<progress", "$", "%", "リセット")
}

func TestUsageRow_PercentWithoutUsedShowsDash(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Unit: domain.UnitPercent}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "x", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "—")
	webtest.MustNotContain(t, html, "<progress")
}

func TestUsageRow_ResetsAtRelative(t *testing.T) {
	t.Parallel()
	r := now.Add(3*time.Hour + 10*time.Minute)
	slot := domain.Slot{Available: true, Used: f(10), Unit: domain.UnitPercent, ResetsAt: &r}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "x", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "あと 3 時間", "リセット")
	slot.ResetsAt = nil
	html = webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "x", Slot: slot, Now: now}))
	webtest.MustNotContain(t, html, "リセット")
}

func TestHostRow(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "CPU", Percent: 95, Available: true}))
	webtest.MustContain(t, html, "[&amp;::-webkit-progress-value]:bg-danger", "95%")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "CPU", Percent: 10}))
	webtest.MustContain(t, html, "—")
	webtest.MustNotContain(t, html, "<progress")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "スワップ", Available: true, NoBar: true, Detail: "なし"}))
	webtest.MustContain(t, html, "なし")
	webtest.MustNotContain(t, html, "<progress")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "/", Percent: 50, Available: true, Detail: "10 GB 空き"}))
	webtest.MustContain(t, html, "10 GB 空き", "[&amp;::-webkit-progress-value]:bg-accent")
}

// elementChildren は n の直下の要素を返す。
func elementChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

// textOf は n 以下のテキストを連結し、空白の連続を 1 つにして前後を落とす。
func textOf(n *html.Node) string {
	return strings.Join(strings.Fields(rawText(n)), " ")
}

func rawText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(rawText(c))
	}
	return sb.String()
}

// hasElement は n 以下にタグ名 tag の要素があるか。
func hasElement(n *html.Node, tag string) bool {
	if n.Type == html.ElementNode && n.Data == tag {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasElement(c, tag) {
			return true
		}
	}
	return false
}

// どの行も 3 列のグリッド（ラベル・バー・値）で、バーの幅が右側の文字の長さで変わらない（Issue #10 B）。
func TestHostRow_ThreeColumnGrid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		props molecules.HostRowProps
		value string // 値の列（3 つ目）の文言
		bar   bool   // 2 つ目の列にバーがあるか
	}{
		{"CPU（補足なし）", molecules.HostRowProps{Label: "CPU", Percent: 1, Available: true}, "1%", true},
		{"ディスク", molecules.HostRowProps{Label: "/", Percent: 87, Available: true, Detail: "108.5 GB 空き"}, "87% 108.5 GB 空き", true},
		{"メモリ", molecules.HostRowProps{Label: "メモリ", Percent: 41, Available: true, Detail: "41.3 GB / 128 GB"}, "41% 41.3 GB / 128 GB", true},
		{"スワップなし", molecules.HostRowProps{Label: "スワップ", Available: true, NoBar: true, Detail: "なし"}, "なし", false},
		{"取れない", molecules.HostRowProps{Label: "メモリ"}, "—", false},
	}
	for _, tt := range tests {
		out := webtest.Render(t, molecules.HostRow(tt.props))
		webtest.MustNotContain(t, out, "flex-1", "shrink-0")
		if webtest.AllAttrKeys(t, out)["style"] {
			t.Errorf("%s: style 属性を使わない（CSP）:\n%s", tt.name, out)
		}

		root := webtest.Elements(t, out, "div")[0]
		class, _ := webtest.Attr(root, "class")
		for _, want := range []string{"grid", "grid-cols-[4.5rem_minmax(0,1fr)_12rem]", "text-sm"} {
			if !slices.Contains(strings.Fields(class), want) {
				t.Errorf("%s: 行の class に %q が無い: %q", tt.name, want, class)
			}
		}
		cols := elementChildren(root)
		if len(cols) != 3 {
			t.Fatalf("%s: 列は 3 つのはず: %d\n%s", tt.name, len(cols), out)
		}
		if got := textOf(cols[0]); got != tt.props.Label {
			t.Errorf("%s: ラベルの列 = %q", tt.name, got)
		}
		hasBar := hasElement(cols[1], "progress")
		if hasBar != tt.bar || (!tt.bar && textOf(cols[1]) != "") {
			t.Errorf("%s: バーの列が想定と違う（bar=%v）", tt.name, hasBar)
		}
		// 率と補足は値の列の 1 つの要素にまとまっている。
		if got := textOf(cols[2]); got != tt.value {
			t.Errorf("%s: 値の列 = %q, want %q", tt.name, got, tt.value)
		}
		if class, _ := webtest.Attr(cols[2], "class"); !strings.Contains(class, "truncate") || !strings.Contains(class, "text-right") {
			t.Errorf("%s: 値の列は右寄せ・truncate: %q", tt.name, class)
		}
	}

	// 値の列が tabular-nums・title 付き（はみ出したとき全文を読める）。
	out := webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "/", Percent: 87, Available: true, Detail: "108.5 GB 空き"}))
	webtest.MustContain(t, out, "tabular-nums", `title="87% 108.5 GB 空き"`)
}

// 補足（メモリの「1 GB / 4 GB」など）は読み取りたい値そのものなので --mute にしない（NFR-DSG-7）。
func TestHostRow_DetailIsInk(t *testing.T) {
	t.Parallel()
	out := webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "メモリ", Percent: 41, Available: true, Detail: "41.3 GB / 128 GB"}))
	for _, span := range webtest.Elements(t, out, "span") {
		if textOf(span) != "41.3 GB / 128 GB" {
			continue
		}
		class, _ := webtest.Attr(span, "class")
		if !slices.Contains(strings.Fields(class), "text-ink") || strings.Contains(class, "text-mute") {
			t.Errorf("補足は text-ink（text-mute にしない）: %q", class)
		}
		return
	}
	t.Fatalf("補足の span が無い:\n%s", out)
}

func TestSessionTab(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.SessionTab(molecules.SessionTabProps{ConnectionID: "01ABC", Session: "dev work", Selected: true}))
	webtest.MustContain(t, html, `hx-get="/connections/01ABC/sessions/dev%20work"`, `hx-target="#main"`, `aria-current="page"`, "dev work")
	html = webtest.Render(t, molecules.SessionTab(molecules.SessionTabProps{ConnectionID: "01ABC", Session: "0"}))
	webtest.MustNotContain(t, html, "aria-current")
}

func TestAttentionItem(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.AttentionItem(molecules.AttentionItemProps{Attention: domain.Attention{Kind: "asking", Pane: "T2", Detail: "どうしますか？"}}))
	webtest.MustContain(t, html, "答え待ち", "T2", "どうしますか？")

	// 未知の kind は文字列のまま。
	html = webtest.Render(t, molecules.AttentionItem(molecules.AttentionItemProps{Attention: domain.Attention{Kind: "brand_new_kind", Pane: "B1"}}))
	webtest.MustContain(t, html, "brand_new_kind", "B1")

	// report_waiting の detail（サーバー上のファイルパス）は出さない。
	html = webtest.Render(t, molecules.AttentionItem(molecules.AttentionItemProps{Attention: domain.Attention{Kind: "report_waiting", Pane: "B1", Detail: "/srv/secret/path.md"}}))
	webtest.MustContain(t, html, "報告待ち")
	webtest.MustNotContain(t, html, "/srv/secret/path.md")

	// auth: pane は空、expires_at を併記。
	exp := time.Date(2026, 11, 5, 9, 30, 0, 0, time.Local)
	html = webtest.Render(t, molecules.AttentionItem(molecules.AttentionItemProps{Attention: domain.Attention{Kind: "auth_expiring", Detail: "claude"}, AuthExpiresAt: exp}))
	webtest.MustContain(t, html, "ログイン期限間近", "claude", "期限 2026/11/05 09:30")
}
