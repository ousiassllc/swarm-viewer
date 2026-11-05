package organisms_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

// アラートの種類を見分けるアイコン（Iconoir の warning-triangle・warning-circle・info-circle の path の一部）。
const (
	warningIcon = "M20.0429 21H3.95705"
	dangerIcon  = "M12 7V13"
	infoIcon    = "M12 11.5V16.5"
)

func fp(v float64) *float64 { return &v }

func usageWith(status string, five domain.Slot) domain.Usage {
	return domain.Usage{
		Status: status, Reason: "timeout",
		Claude: []domain.AgentAccount{{Account: "main", FiveHour: five, SevenDay: domain.Slot{}}},
		Codex:  []domain.AgentAccount{{Account: "cx", FiveHour: five}},
		Cursor: []domain.CursorAccount{{Account: "cur", Window: strp("monthly"), Models: domain.Slot{Available: true, Used: fp(20), Unit: domain.UnitUSD}}},
	}
}

func strp(s string) *string { return &s }

func TestUsagePanel_ToneBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		slot   domain.Slot
		want   string
		absent []string
	}{
		{"79.9", domain.Slot{Available: true, Used: fp(79.9), Unit: domain.UnitPercent}, "[&amp;::-webkit-progress-value]:bg-accent", []string{"[&amp;::-webkit-progress-value]:bg-warning", "[&amp;::-webkit-progress-value]:bg-danger"}},
		{"80", domain.Slot{Available: true, Used: fp(80), Unit: domain.UnitPercent}, "[&amp;::-webkit-progress-value]:bg-warning", []string{"[&amp;::-webkit-progress-value]:bg-danger"}},
		{"94.9", domain.Slot{Available: true, Used: fp(94.9), Unit: domain.UnitPercent}, "[&amp;::-webkit-progress-value]:bg-warning", []string{"[&amp;::-webkit-progress-value]:bg-danger"}},
		{"95", domain.Slot{Available: true, Used: fp(95), Unit: domain.UnitPercent}, "[&amp;::-webkit-progress-value]:bg-danger", []string{"[&amp;::-webkit-progress-value]:bg-warning"}},
		{"exhausted", domain.Slot{Available: true, Used: fp(1), Unit: domain.UnitPercent, Status: domain.SlotStatusExhausted}, "[&amp;::-webkit-progress-value]:bg-danger", []string{"[&amp;::-webkit-progress-value]:bg-warning"}},
	}
	for _, tt := range tests {
		html := webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: usageWith(domain.UsageOK, tt.slot), Now: now}))
		webtest.MustContain(t, html, tt.want)
		webtest.MustNotContain(t, html, tt.absent...)
	}
}

func TestUsagePanel_StatusBranches(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: fp(42), Unit: domain.UnitPercent}
	fetched := time.Date(2026, 11, 3, 11, 0, 0, 0, time.Local)
	u := usageWith(domain.UsageOK, slot)
	u.FetchedAt = &fetched

	// ok: 通常表示 + 最終取得。グレーなし。
	html := webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
	webtest.MustContain(t, html, "Claude · main", "Codex · cx", "Cursor · cur", "月間", "$20.00", "42%", "最終取得 11/03 11:00", "5 時間枠", "7 日枠", "—")
	webtest.MustNotContain(t, html, "opacity-50", "古いデータ", "<details")
	// アカウント名（h3）から本文までは 8px（ガイド §3 見出し→本文）。
	webtest.MustContain(t, html, `<div class="flex flex-col gap-2"><h3 class="text-sm text-ink">Claude · main</h3>`, `<div class="flex flex-col gap-2"><h3 class="text-sm text-ink">Cursor · cur</h3>`)

	// stale: グレーアウト + 古いデータ + fetched_at + reason は折りたたみ。
	u.Status = domain.UsageStale
	html = webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
	webtest.MustContain(t, html, "opacity-50", "古いデータ", "最終取得 11/03 11:00", "<details", "timeout", "42%")
	// 取得状態は状態色のアラート（アイコン＋文言。ガイド C4）。stale は注意。
	webtest.MustContain(t, html, `role="alert"`, "data-alert", warningIcon)
	webtest.MustNotContain(t, html, `<p class="text-sm text-ink">古いデータ`)

	// unavailable・pending・disabled・未知: 数字を出さない。unavailable・未知はエラー、pending・disabled はお知らせ（role="status"）。
	for _, tt := range []struct{ status, label, role, icon string }{
		{domain.UsageUnavailable, "取得できません", `role="alert"`, dangerIcon},
		{domain.UsagePending, "取得中", `role="status"`, infoIcon},
		{domain.UsageDisabled, "利用上限の取得は無効です", `role="status"`, infoIcon},
		{"brand-new", "取得できません", `role="alert"`, dangerIcon},
		{"", "取得できません", `role="alert"`, dangerIcon},
	} {
		u.Status = tt.status
		html = webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
		webtest.MustContain(t, html, tt.label, "data-alert", tt.role, tt.icon)
		webtest.MustNotContain(t, html, "42%", "<progress", "$20.00", "Claude", "最終取得", `<p class="text-sm text-ink">`+tt.label)
	}

	// reason は unavailable のときだけ折りたたみ（pending・disabled では出さない）。
	u.Status = domain.UsageUnavailable
	webtest.MustContain(t, webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now})), "<details", "timeout")
	u.Status = domain.UsagePending
	webtest.MustNotContain(t, webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now})), "<details")
}

func TestUsagePanel_BothSlotsUnavailableIsOneLine(t *testing.T) {
	t.Parallel()
	render := func(a domain.AgentAccount) string {
		u := domain.Usage{Status: domain.UsageOK, Claude: []domain.AgentAccount{a}}
		return webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
	}

	// 2 枠とも取れない（reason なし・古い swarm-console）: 「取得できません」1 行。「—」・枠ラベルは出ない。
	html := render(domain.AgentAccount{Account: "account-1"})
	webtest.MustContain(t, html, "Claude · account-1", "取得できません", "data-alert", `role="alert"`, warningIcon)
	webtest.MustNotContain(t, html, "—", "5 時間枠", "7 日枠", "ログイン切れ")
	if n := strings.Count(html, "取得できません"); n != 1 {
		t.Errorf("「取得できません」は 1 回のはず: %d 回\n%s", n, html)
	}

	// reason があれば理由を添える。
	for reason, want := range map[string]string{
		"without_usage": "ログイン切れの可能性",
		"no_email":      "アカウントを特定できません（ログイン切れの可能性）",
	} {
		html = render(domain.AgentAccount{Account: "account-1", Reason: strp(reason)})
		webtest.MustContain(t, html, "取得できません", want)
		webtest.MustNotContain(t, html, "—", "5 時間枠")
	}

	// 知らない reason は理由なし（「取得できません」だけ）。
	html = render(domain.AgentAccount{Account: "account-1", Reason: strp("brand-new")})
	webtest.MustContain(t, html, "取得できません")
	webtest.MustNotContain(t, html, "brand-new", "ログイン切れ", "—")

	// 片方だけ取れない: これまでどおり 2 行（取れない枠は「—」）。reason があっても理由は出さない。
	ok := domain.Slot{Available: true, Used: fp(30), Unit: domain.UnitPercent}
	html = render(domain.AgentAccount{Account: "account-1", FiveHour: ok, Reason: strp("without_usage")})
	webtest.MustContain(t, html, "5 時間枠", "7 日枠", "30%", "—")
	webtest.MustNotContain(t, html, "ログイン切れ")
	if strings.Contains(html, ">取得できません<") {
		t.Errorf("片方だけ取れないアカウントに 1 行表示を出さない:\n%s", html)
	}
}

func TestUsagePanel_CodexBothSlotsUnavailable(t *testing.T) {
	t.Parallel()
	u := domain.Usage{Status: domain.UsageOK, Codex: []domain.AgentAccount{{Account: "cx", Reason: strp("no_email")}}}
	html := webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
	webtest.MustContain(t, html, "Codex · cx", "取得できません", "アカウントを特定できません")
	webtest.MustNotContain(t, html, "—", "5 時間枠")
}

func TestUsagePanel_StaleFlagGraysOut(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: domain.Usage{Status: domain.UsageOK}, Stale: true, Now: now}))
	webtest.MustContain(t, html, "opacity-50")
	webtest.MustNotContain(t, html, "古いデータ")
}

func TestUsagePanel_CursorNullWindow(t *testing.T) {
	t.Parallel()
	u := domain.Usage{Status: domain.UsageOK, Cursor: []domain.CursorAccount{{Account: "c", Models: domain.Slot{}}}}
	html := webtest.Render(t, organisms.UsagePanel(organisms.UsagePanelProps{Usage: u, Now: now}))
	webtest.MustContain(t, html, "モデル", "—")
}

func TestHostPanel(t *testing.T) {
	t.Parallel()
	h := domain.Host{
		CPU:  domain.CPU{Available: true, UsedPercent: 96},
		Mem:  domain.Mem{Available: true, TotalGB: 128, UsedGB: 41.3, UsedPercent: 32.3},
		Swap: domain.Swap{Available: true, TotalGB: 0},
		Disk: []domain.Disk{
			{Mount: "/", Available: true, UsedPercent: 81, FreeGB: 12.34},
			{Mount: "/data", Available: false},
		},
	}
	html := webtest.Render(t, organisms.HostPanel(organisms.HostPanelProps{Host: h}))
	webtest.MustContain(t, html, "CPU", "96%", "[&amp;::-webkit-progress-value]:bg-danger", "メモリ", "32.3%", "41.3 GB / 128 GB", "スワップ", "なし", "12.3 GB 空き", "[&amp;::-webkit-progress-value]:bg-warning", "/data", "—")
	webtest.MustNotContain(t, html, "load")
	// 表示順: CPU → メモリ → スワップ → ディスク。
	cpu, mem, swap, disk := strings.Index(html, "CPU"), strings.Index(html, "メモリ"), strings.Index(html, "スワップ"), strings.Index(html, "/data")
	if cpu >= mem || mem >= swap || swap >= disk {
		t.Errorf("行の順が CPU → メモリ → スワップ → ディスクでない:\n%s", html)
	}

	h.Swap = domain.Swap{Available: true, TotalGB: 4, UsedGB: 1, UsedPercent: 25}
	webtest.MustContain(t, webtest.Render(t, organisms.HostPanel(organisms.HostPanelProps{Host: h})), "1 GB / 4 GB", "25%")
	h.Swap = domain.Swap{}
	webtest.MustContain(t, webtest.Render(t, organisms.HostPanel(organisms.HostPanelProps{Host: h})), "スワップ")
}

func TestHostPanel_Mem(t *testing.T) {
	t.Parallel()
	render := func(m domain.Mem) string {
		h := domain.Host{CPU: domain.CPU{Available: true}, Mem: m, Swap: domain.Swap{Available: true, TotalGB: 4, UsedGB: 1, UsedPercent: 25}}
		return webtest.Render(t, organisms.HostPanel(organisms.HostPanelProps{Host: h}))
	}

	// しきい値色は CPU などと同じ（80 / 95）。
	webtest.MustContain(t, render(domain.Mem{Available: true, TotalGB: 100, UsedGB: 79.9, UsedPercent: 79.9}), "79.9%", "79.9 GB / 100 GB", "[&amp;::-webkit-progress-value]:bg-accent")
	webtest.MustContain(t, render(domain.Mem{Available: true, TotalGB: 100, UsedGB: 80, UsedPercent: 80}), "[&amp;::-webkit-progress-value]:bg-warning")
	webtest.MustContain(t, render(domain.Mem{Available: true, TotalGB: 100, UsedGB: 95, UsedPercent: 95}), "[&amp;::-webkit-progress-value]:bg-danger")

	// host.mem が無い（古い swarm-console）・available: false は「メモリ」の行を「—」で出す。ほかの行は崩れない。
	for _, m := range []domain.Mem{{}, {Available: false, TotalGB: 8, UsedGB: 4, UsedPercent: 50}} {
		html := render(m)
		webtest.MustContain(t, html, "メモリ", "—", "スワップ", "1 GB / 4 GB", "25%")
		webtest.MustNotContain(t, html, "8 GB", "50%")
	}
}

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

func TestSessionTabs(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SessionTabs(organisms.SessionTabsProps{
		ConnectionID: "c1", Selected: "1",
		Sessions: []domain.Session{{ID: "0", Allowed: true}, {ID: "1", Allowed: true}, {ID: "hidden", Allowed: false}},
	}))
	webtest.MustContain(t, html, `hx-get="/connections/c1/sessions/0"`, `hx-get="/connections/c1/sessions/1"`)
	webtest.MustNotContain(t, html, "hidden")
	if strings.Count(html, "aria-current") != 1 {
		t.Error("選択中は 1 つだけ")
	}
	// セッションが 1 つだけでもタブを出す。
	one := webtest.Render(t, organisms.SessionTabs(organisms.SessionTabsProps{ConnectionID: "c1", Selected: "0", Sessions: []domain.Session{{ID: "0", Allowed: true}}}))
	webtest.MustContain(t, one, "/connections/c1/sessions/0")
}
