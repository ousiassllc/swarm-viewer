package molecules_test

import (
	"testing"
	"time"

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
		{"79.9", 79.9, "bg-sky-500"},
		{"80", 80, "bg-amber-500"},
		{"94.9", 94.9, "bg-amber-500"},
		{"95", 95, "bg-red-500"},
	}
	for _, tt := range tests {
		slot := domain.Slot{Available: true, Used: f(tt.used), Unit: domain.UnitPercent, Status: "ok"}
		html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "5 時間枠", Slot: slot, Now: now}))
		webtest.MustContain(t, html, "<progress", tt.want, "5 時間枠")
		if tt.name == "79.9" {
			webtest.MustNotContain(t, html, "bg-amber-500", "bg-red-500")
		}
	}
}

func TestUsageRow_Exhausted(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(10), Unit: domain.UnitPercent, Status: domain.SlotStatusExhausted}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "x", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "bg-red-500", "上限到達")
}

func TestUsageRow_USDHasAmountOnly(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(512.5), Unit: domain.UnitUSD, Status: "ok"}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "月間", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "$512.50")
	webtest.MustNotContain(t, html, "<progress", "bg-amber-500", "bg-red-500", "%")
}

func TestUsageRow_USDExhaustedIsError(t *testing.T) {
	t.Parallel()
	slot := domain.Slot{Available: true, Used: f(1), Unit: domain.UnitUSD, Status: domain.SlotStatusExhausted}
	html := webtest.Render(t, molecules.UsageRow(molecules.UsageRowProps{Label: "月間", Slot: slot, Now: now}))
	webtest.MustContain(t, html, "text-red-400", "上限到達")
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
	webtest.MustContain(t, html, "bg-red-500", "95%")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "CPU", Percent: 10}))
	webtest.MustContain(t, html, "—")
	webtest.MustNotContain(t, html, "<progress")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "スワップ", Available: true, NoBar: true, Detail: "なし"}))
	webtest.MustContain(t, html, "なし")
	webtest.MustNotContain(t, html, "<progress")
	html = webtest.Render(t, molecules.HostRow(molecules.HostRowProps{Label: "/", Percent: 50, Available: true, Detail: "10 GB 空き"}))
	webtest.MustContain(t, html, "10 GB 空き", "bg-sky-500")
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
	webtest.MustContain(t, html, "ログイン期限間近", "claude", "期限 11/05 09:30")
}
