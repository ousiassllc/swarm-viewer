package handler_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func setLastConnection(a *app, id string) {
	a.env.Store.Mu.Lock()
	a.env.Store.Cfg.LastConnectionID = id
	a.env.Store.Mu.Unlock()
}

func TestSessionPage(t *testing.T) {
	setup := func(t *testing.T) *app {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.AddConn("c2", "other", servicetest.URL2, true)
		a.env.Report.SetSessions("c1", "0", "1")
		a.env.Report.SetReport("c1", "0", servicetest.ReportOf(servicetest.Pane("%1", "T1", domain.StateWorking)))
		return a
	}

	t.Run("フルページ: タブ・接続切替・ポーリング属性", func(t *testing.T) {
		a := setup(t)
		if _, err := a.env.Conns.Tabs(t.Context(), "c1"); err != nil { // 直近のタブがある状態
			t.Fatal(err)
		}
		before := sessionsListCalls(a, "c1")
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "<html", `<main id="main"`, "接続: devbox",
			`hx-get="/connections/c1/sessions/1"`, // タブ
			`hx-get="/connections/c2"`,            // 接続切替（2 件以上）
			`hx-get="/connections/c1/sessions/0/snapshot"`,
			`hx-trigger="load, every 10s, refresh from:body"`,
			`hx-sync="this:drop"`,
			`id="snapshot-status"`)
		if strings.Contains(body, "[document") {
			t.Fatal("トリガーフィルタ式を出してはいけない")
		}
		// 画面の描画では取得しない（取得は領域の load が行う）。
		if n := a.env.Report.ReportCount(); n != 0 {
			t.Fatalf("report を %d 回呼んだ。初回描画では呼ばない", n)
		}
		if n := sessionsListCalls(a, "c1"); n != before {
			t.Fatalf("初回描画で sessions_list を呼んだ（%d 回 → %d 回）", before, n)
		}
	})
	t.Run("直近のタブが無ければ選択中の 1 件だけで描画し、取得しない", func(t *testing.T) {
		a := setup(t)
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, `id="session-tabs"`, `hx-get="/connections/c1/sessions/0"`, `hx-get="/connections/c1/sessions/0/snapshot"`)
		webtest.MustNotContain(t, body, `hx-get="/connections/c1/sessions/1"`)
		if n := sessionsListCalls(a, "c1"); n != 0 {
			t.Fatalf("初回描画で sessions_list を呼んだ（%d 回）", n)
		}
	})
	t.Run("フラグメント: レイアウトなし", func(t *testing.T) {
		a := setup(t)
		body := wantStatus(t, a.get("/connections/c1/sessions/0", hx()), http.StatusOK)
		webtest.MustContain(t, body, "接続: devbox", `hx-get="/connections/c1/sessions/0/snapshot"`)
		webtest.MustNotContain(t, body, "<html", "<body")
	})
	t.Run("画面更新間隔は設定をレンダリングする", func(t *testing.T) {
		a := setup(t)
		a.env.Store.Mu.Lock()
		a.env.Store.Cfg.ScreenPollSeconds = 30
		a.env.Store.Mu.Unlock()
		body := wantStatus(t, a.get("/connections/c1/sessions/0", hx()), http.StatusOK)
		webtest.MustContain(t, body, `hx-trigger="load, every 30s, refresh from:body"`)
	})
	t.Run("last_session と last_connection を記録する", func(t *testing.T) {
		a := setup(t)
		wantStatus(t, a.get("/connections/c1/sessions/1"), http.StatusOK)
		a.env.Store.Mu.Lock()
		defer a.env.Store.Mu.Unlock()
		if got := a.env.Store.Conns[0].LastSession; got != "1" {
			t.Fatalf("last_session = %q", got)
		}
		if got := a.env.Store.Cfg.LastConnectionID; got != "c1" {
			t.Fatalf("last_connection_id = %q", got)
		}
	})
	t.Run("許可外と分かっているセッションは先頭を描画する（ポーリング URL は正規のパス）", func(t *testing.T) {
		a := setup(t)
		if _, err := a.env.Conns.Tabs(t.Context(), "c1"); err != nil {
			t.Fatal(err)
		}
		w := a.get("/connections/c1/sessions/zzz")
		body := wantStatus(t, w, http.StatusOK)
		webtest.MustContain(t, body, `hx-get="/connections/c1/sessions/0/snapshot"`)
		webtest.MustNotContain(t, body, "/sessions/zzz")
		if loc := w.Header().Get("Location"); loc != "" {
			t.Fatalf("Location = %q", loc)
		}
		a.env.Store.Mu.Lock()
		defer a.env.Store.Mu.Unlock()
		if got := a.env.Store.Conns[0].LastSession; got != "0" {
			t.Fatalf("last_session = %q", got)
		}
	})
	t.Run("タブの取得が古ければ、新しいセッションを信じて描画する（snapshot が確かめる）", func(t *testing.T) {
		a := setup(t)
		if _, err := a.env.Conns.Tabs(t.Context(), "c1"); err != nil {
			t.Fatal(err)
		}
		a.env.Clock.Advance(time.Minute)
		body := wantStatus(t, a.get("/connections/c1/sessions/new"), http.StatusOK)
		webtest.MustContain(t, body, `hx-get="/connections/c1/sessions/new/snapshot"`, `hx-get="/connections/c1/sessions/0"`)
	})
	t.Run("セッション名は URL エンコードを復元する", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.Report.SetSessions("c1", "my session")
		body := wantStatus(t, a.get("/connections/c1/sessions/my%20session", hx()), http.StatusOK)
		webtest.MustContain(t, body, `hx-get="/connections/c1/sessions/my%20session/snapshot"`)
	})
	t.Run("許可セッション 0 件（取得済み）は引き直して専用ページ", func(t *testing.T) {
		a := setup(t)
		a.env.Report.SetSessions("c1")
		if _, err := a.env.Conns.Tabs(t.Context(), "c1"); err != nil {
			t.Fatal(err)
		}
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "許可されたセッションがありません")
	})
	t.Run("未知の接続は 404", func(t *testing.T) {
		a := setup(t)
		wantStatus(t, a.get("/connections/nope/sessions/0"), http.StatusNotFound)
	})
	t.Run("ホストに届かなくても待たずに 200 で前回値とバナー（FR-POLL-4）", func(t *testing.T) {
		a := setup(t)
		saveSnapshot(a, "c1", "0")
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "opacity-50", `role="alert"`, "前回取得", `hx-get="/connections/c1/sessions/0/snapshot"`)
		if n := sessionsListCalls(a, "c1") + a.env.Report.ReportCount(); n != 0 {
			t.Fatalf("初回描画でホストを %d 回呼んだ", n)
		}
	})
	t.Run("needs_repair ではポーリング属性を出さない", func(t *testing.T) {
		a := setup(t)
		a.env.Conns.MarkNeedsRepair("c1")
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnauthorized}
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "再ペアリング")
		webtest.MustNotContain(t, body, "hx-trigger", `hx-get="/connections/c1/sessions/0/snapshot"`)
	})
}
