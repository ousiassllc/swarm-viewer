package pages_test

import (
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

var shell = layout.BaseProps{Title: "t", CSRFToken: "csrf-test-token"} //nolint:gosec // テスト用の偽の値

func mainProps() pages.MainProps {
	conn := domain.Connection{ID: "01A", Name: "devbox", URL: "http://100.64.0.5:48120/report"}
	return pages.MainProps{
		Connection:  conn,
		Connections: []domain.Connection{conn, {ID: "01B", Name: "other"}},
		Sessions:    []domain.Session{{ID: "0", Allowed: true}, {ID: "1", Allowed: true}},
		Selected:    "1",
		Snapshot: organisms.SnapshotViewProps{
			Snapshot: domain.Snapshot{
				FetchedAt: time.Date(2026, 11, 3, 9, 0, 0, 0, time.Local),
				Report:    domain.Report{Panes: []domain.Pane{{Pos: "T1", State: domain.StateIdle, Title: "p"}}},
			},
			PollSeconds: 30,
		},
	}
}

func TestMainPage(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.MainPage(shell, mainProps()))
	webtest.MustContain(t, html,
		"接続: devbox", `hx-get="/connections/01B"`, // 接続が複数なので切替を出す
		`hx-get="/connections/01A/sessions/1"`, `hx-target="#main"`, // 更新ボタン・タブ
		`hx-get="/settings"`, `hx-get="/connections"`,
		`id="snapshot-status"`, `id="snapshot"`,
		`hx-get="/connections/01A/sessions/1/snapshot"`,
		`hx-trigger="load, every 30s, refresh from:body"`,
		"opacity-50", "前回取得 09:00", // 初回描画は直前値をグレーアウト（FR-POLL-4）
		"csrf-test-token",
	)
	webtest.MustNotContain(t, html, "100.64.0.5", "48120")
}

func TestMainContent_SingleConnectionHasNoSwitcher(t *testing.T) {
	t.Parallel()
	p := mainProps()
	p.Connections = p.Connections[:1]
	html := webtest.Render(t, pages.MainContent(p))
	webtest.MustNotContain(t, html, "接続の切替", "<html", "csrf-token")
}

func TestMainContent_RepairHasNoPolling(t *testing.T) {
	t.Parallel()
	p := mainProps()
	p.Snapshot.Repair = true
	html := webtest.Render(t, pages.MainContent(p))
	webtest.MustNotContain(t, html, "hx-trigger", "/snapshot\"", "hx-sync")
	webtest.MustContain(t, html, `href="/pairing"`)
}

func TestPairingPage(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.PairingPage(shell, pages.PairingProps{HasConnections: true}))
	webtest.MustContain(t, html, `id="pairing-form"`, `hx-post="/pairing"`, "保存済みの接続", "<html")
	webtest.MustNotContain(t, webtest.Render(t, pages.PairingContent(pages.PairingProps{})), "保存済みの接続")
}

func TestPairingDraftPage_NoTokenAndRoutes(t *testing.T) {
	t.Parallel()
	p := pages.DraftProps{DraftID: "dr_AFT-1", Name: "devbox", Host: "100.64.0.5", SessionsHint: []string{"0", "1"}, ExpiresText: "あと 4 分"}
	html := webtest.Render(t, pages.PairingDraftPage(shell, p))
	webtest.MustContain(t, html,
		"devbox", "100.64.0.5", "0, 1", "あと 4 分",
		`hx-post="/pairing/drafts/dr_AFT-1/confirm"`, `hx-post="/pairing/drafts/dr_AFT-1/discard"`,
		"接続を保存", "破棄",
	)
	webtest.MustNotContain(t, html, "48120", "/report", "swarm-viewer://", "Bearer")
	webtest.MustContain(t, webtest.Render(t, pages.PairingDraftContent(pages.DraftProps{DraftID: "d"})), "申告なし")
}

func TestPairingDraft_DraftIDIsEscapedInPath(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.PairingDraftContent(pages.DraftProps{DraftID: `a/b"c`}))
	webtest.MustNotContain(t, html, `a/b"c`)
	webtest.MustContain(t, html, "/pairing/drafts/a%2Fb%22c/confirm")
}

func TestPairingDraft_ConfirmError(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.PairingDraftContent(pages.DraftProps{DraftID: "d", Error: "ホストに届きません。Tailscale 接続を確認してください"}))
	webtest.MustContain(t, html, `role="alert"`, "ホストに届きません", "confirm")
}

func TestDraftExpiredPage(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.DraftExpiredPage(shell))
	webtest.MustContain(t, html, "もう一度ペアリングしてください", `href="/pairing"`)
}

func TestConnectionsAndNoSessions(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.ConnectionsPage(shell, pages.ConnectionsProps{List: organisms.ConnectionListProps{Connections: []domain.Connection{{ID: "01A", Name: "n"}}}}))
	webtest.MustContain(t, html, "接続管理", `hx-delete="/connections/01A"`, `href="/pairing"`)
	html = webtest.Render(t, pages.NoSessionsPage(shell, "devbox"))
	webtest.MustContain(t, html, "許可されたセッションがありません", "devbox")
	webtest.MustNotContain(t, webtest.Render(t, pages.NoSessionsContent("")), "接続「")
}

func TestSettingsPage(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, pages.SettingsPage(shell, pages.SettingsProps{Form: organisms.SettingsFormProps{Settings: domain.DefaultSettings()}}))
	webtest.MustContain(t, html, `id="settings-form"`, `hx-put="/settings"`, "デバッグログ", "閉じる")
}

func TestRecoveryPage(t *testing.T) {
	t.Parallel()
	for reason, want := range map[pages.RecoveryReason]string{
		pages.RecoveryCorrupt:      "壊れている可能性",
		pages.RecoverySchemaTooNew: "より新しい版",
		"whatever":                 "壊れている可能性",
	} {
		html := webtest.Render(t, pages.RecoveryPage(shell, pages.RecoveryProps{Reason: reason}))
		webtest.MustContain(t, html, want, `hx-post="/recovery/reset"`, `hx-confirm=`, "保存データを初期化する", "秘密領域に残ります")
	}
	webtest.MustContain(t, webtest.Render(t, pages.RecoveryContent(pages.RecoveryProps{Error: "初期化に失敗しました"})), "初期化に失敗しました")
}

// どの画面にも、token・Authorization・接続 URL の全体が出ない。
func TestAllPages_NeverContainTokenLikeStrings(t *testing.T) {
	t.Parallel()
	const fakeToken = "test-token-not-secret"
	conn := domain.Connection{ID: "01A", Name: "devbox", URL: "http://100.64.0.5:48120/report"}
	comps := map[string]templ.Component{
		"main":     pages.MainPage(shell, mainProps()),
		"pairing":  pages.PairingPage(shell, pages.PairingProps{Form: organisms.PairingFormProps{Error: "e"}}),
		"draft":    pages.PairingDraftPage(shell, pages.DraftProps{DraftID: "d", Name: "n", Host: "100.64.0.5"}),
		"expired":  pages.DraftExpiredPage(shell),
		"conns":    pages.ConnectionsPage(shell, pages.ConnectionsProps{List: organisms.ConnectionListProps{Connections: []domain.Connection{conn}}}),
		"nosess":   pages.NoSessionsPage(shell, "n"),
		"settings": pages.SettingsPage(shell, pages.SettingsProps{}),
		"recovery": pages.RecoveryPage(shell, pages.RecoveryProps{}),
	}
	for name, c := range comps {
		html := webtest.Render(t, c)
		for _, banned := range []string{fakeToken, "Authorization", "Bearer", "token=", "http://100.64.0.5"} {
			if strings.Contains(html, banned) {
				t.Errorf("%s: %q が含まれている", name, banned)
			}
		}
	}
}
