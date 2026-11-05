package organisms_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

var now = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func sampleSnapshot() domain.Snapshot {
	return domain.Snapshot{
		FetchedAt: time.Date(2026, 11, 3, 11, 58, 0, 0, time.Local),
		Report: domain.Report{
			Layout: domain.Layout{Rows: 2, Cols: []int{2, 1}},
			Panes: []domain.Pane{
				{Pos: "T1", ID: "%1", State: domain.StateWorking, Title: "alpha"},
				{Pos: "T2", ID: "%2", State: domain.StateAsking, Title: "beta"},
				{Pos: "B1", ID: "%3", State: domain.StateIdle, Title: "gamma"},
			},
			Attention: []domain.Attention{{Kind: "asking", Pane: "T2", Detail: "q?"}},
			Usage:     domain.Usage{Status: domain.UsageOK},
			Host:      domain.Host{CPU: domain.CPU{Available: true, UsedPercent: 12}},
		},
	}
}

func baseProps() organisms.SnapshotViewProps {
	return organisms.SnapshotViewProps{
		Snapshot: sampleSnapshot(), Fresh: true, PollSeconds: 10,
		ConnectionID: "01CONN", Session: "0", Now: now,
	}
}

func TestSnapshotView_PollingAttributes(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SnapshotView(baseProps()))
	webtest.MustContain(t,
		html,
		`hx-get="/connections/01CONN/sessions/0/snapshot"`,
		`hx-trigger="every 10s, refresh from:body"`,
		`hx-sync="this:drop"`,
		`hx-swap="outerHTML"`,
	)
	// 置き換えた要素が load で再取得し続けない。フィルタ式 [...] も使わない。
	trigger := hxTrigger(t, html)
	if strings.Contains(trigger, "load") || strings.ContainsAny(trigger, "[]") {
		t.Errorf("hx-trigger = %q", trigger)
	}
}

func TestSnapshotView_InitialAddsLoad(t *testing.T) {
	t.Parallel()
	p := baseProps()
	p.Initial = true
	webtest.MustContain(t, webtest.Render(t, organisms.SnapshotRegion(p)), `hx-trigger="load, every 10s, refresh from:body"`)
}

func TestSnapshotView_PollSecondsValidated(t *testing.T) {
	t.Parallel()
	for _, n := range []int{5, 10, 30, 60} {
		p := baseProps()
		p.PollSeconds = n
		webtest.MustContain(t, webtest.Render(t, organisms.SnapshotView(p)), `hx-trigger="every `+itoa(n)+`s, refresh from:body"`)
	}
	for _, n := range []int{0, -1, 7, 3600} {
		p := baseProps()
		p.PollSeconds = n
		webtest.MustContain(t, webtest.Render(t, organisms.SnapshotView(p)), `hx-trigger="every 10s, refresh from:body"`)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// hxTrigger は #snapshot 要素の hx-trigger 属性値。
func hxTrigger(t *testing.T, html string) string {
	t.Helper()
	for _, d := range webtest.Elements(t, html, "div") {
		if id, _ := webtest.Attr(d, "id"); id == "snapshot" {
			v, _ := webtest.Attr(d, "hx-trigger")
			return v
		}
	}
	t.Fatal("#snapshot が無い")
	return ""
}

func TestSnapshotView_RepairHasNoPollingAttributes(t *testing.T) {
	t.Parallel()
	p := baseProps()
	p.Repair = true
	html := webtest.Render(t, organisms.SnapshotView(p))
	webtest.MustNotContain(t, html, "hx-get=\"/connections", "hx-trigger", "hx-sync", "every ")
	// Initial でも出さない。
	p.Initial = true
	html = webtest.Render(t, organisms.SnapshotRegion(p))
	webtest.MustNotContain(t, html, "hx-get", "hx-trigger", "load")
}

func TestSnapshotView_UnauthorizedStopsPollingAndOffersRepair(t *testing.T) {
	t.Parallel()
	for _, kind := range []domain.ErrKind{domain.ErrUnauthorized, domain.ErrForbiddenScope} {
		p := baseProps()
		p.Fresh = false
		p.Err = &domain.FetchError{Kind: kind}
		html := webtest.Render(t, organisms.SnapshotView(p))
		webtest.MustNotContain(t, html, "hx-trigger", "hx-sync", "次回再試行")
		webtest.MustContain(t, html, `href="/pairing"`, "再ペアリング", "前回取得 11:58")
	}
}

func TestSnapshotView_StaleIsGrayedWithBanner(t *testing.T) {
	t.Parallel()
	p := baseProps()
	p.Fresh = false
	p.Err = &domain.FetchError{Kind: domain.ErrUnreachable, Code: "timeout"}
	html := webtest.Render(t, organisms.SnapshotView(p))
	webtest.MustContain(t, html,
		`id="snapshot" class="flex flex-col gap-4 opacity-50 grayscale"`,
		`id="snapshot-status" hx-swap-oob="true"`,
		// 失敗の種別は見出し（SemiBold）、前回取得・次回再試行は本文（ガイド §4 のインラインアラート）。
		`<p class="font-semibold">ホストに届きません。Tailscale 接続を確認してください</p>`,
		"前回取得 11:58", "次回再試行 10 秒後", "timeout",
		`hx-trigger="every 10s, refresh from:body"`, // 到達不可は自動再試行する
	)
	// グレーアウトは領域の 1 回だけ。カードや利用上限で重ねると opacity が掛け算になる（ガイド C5）。
	if n := strings.Count(html, "opacity-50"); n != 1 {
		t.Errorf("opacity-50 は領域の 1 か所だけのはず（%d 回）:\n%s", n, html)
	}
}

func TestSnapshotView_FreshHasNoGrayNoBannerContent(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SnapshotView(baseProps()))
	webtest.MustNotContain(t, html, "opacity-50", "grayscale", `role="alert"`)
	// OOB の空バナーは出す（前回のバナーを消すため）。
	webtest.MustContain(t, html, `id="snapshot-status" hx-swap-oob="true"`)
}

func TestSnapshotView_BannerNeverShowsErrorMessageOrDetails(t *testing.T) {
	t.Parallel()
	p := baseProps()
	p.Fresh = false
	p.Err = &domain.FetchError{Kind: domain.ErrHostError, Code: "tmux_error"}
	html := webtest.Render(t, organisms.SnapshotView(p))
	webtest.MustContain(t, html, "ホスト側でエラーが発生しました", "tmux_error")
}

func TestSnapshotView_EmptyState(t *testing.T) {
	t.Parallel()
	p := organisms.SnapshotViewProps{Fresh: false, Err: &domain.FetchError{Kind: domain.ErrUnreachable}, PollSeconds: 10, ConnectionID: "c", Session: "0", Now: now}
	html := webtest.Render(t, organisms.SnapshotView(p))
	webtest.MustContain(t, html, "表示できるデータがありません", "ホストに届きません", "border-dashed")
	webtest.MustNotContain(t, html, "<progress", "<article")
}

func TestSnapshotView_InitialWithoutErrorShowsPreviousNotice(t *testing.T) {
	t.Parallel()
	p := baseProps()
	p.Fresh = false
	p.Initial = true
	html := webtest.Render(t, organisms.SnapshotStatus(p))
	webtest.MustContain(t, html, `id="snapshot-status"`, "前回の状態を表示中", "前回取得 11:58")
	webtest.MustNotContain(t, html, "hx-swap-oob")
	// 直前値が無い初回はエラーではないので、バナーは出さず領域に「読み込み中…」だけを出す。
	p.Snapshot = domain.Snapshot{}
	status := webtest.Render(t, organisms.SnapshotStatus(p))
	webtest.MustContain(t, status, `id="snapshot-status"`)
	webtest.MustNotContain(t, status, `role="alert"`, "読み込み中…")
	region := webtest.Render(t, organisms.SnapshotRegion(p))
	webtest.MustContain(t, region, "読み込み中…")
	webtest.MustNotContain(t, region, "表示できるデータがありません")
	// 読み込み中は空状態ではなくスピナー（ガイド S4）。
	webtest.MustContain(t, region, `role="status"`, "htmx-indicator", "animate-spin")
	webtest.MustNotContain(t, region, "border-dashed")
	// ポーリング応答（SnapshotView）でも OOB のバナーは空になる。
	oob := webtest.Render(t, organisms.SnapshotView(p))
	webtest.MustContain(t, oob, `hx-swap-oob="true"`, "読み込み中…")
	webtest.MustNotContain(t, oob, `role="alert"`, "前回の状態を表示中")
}

func TestSnapshotView_ContainsAllPanels(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SnapshotView(baseProps()))
	webtest.MustContain(t, html, "alpha", "beta", "gamma", "要対応", "利用上限", "ホスト", "CPU", `data-layout="grid"`)
	// asking の T2 は強調表示。
	if !strings.Contains(html, `ring-warning/40" data-pane-pos="T2"`) {
		t.Errorf("asking の T2 が強調されていない")
	}
}

func TestSnapshotView_NoSecrets(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.SnapshotView(baseProps()))
	webtest.MustNotContain(t, html, "token", "Bearer", "Authorization")
}
