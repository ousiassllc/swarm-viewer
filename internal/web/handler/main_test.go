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

func TestHome(t *testing.T) {
	t.Run("接続が 0 件なら /pairing", func(t *testing.T) {
		a := newApp(t)
		wantRedirect(t, a.get("/"), http.StatusFound, "/pairing")
	})
	t.Run("最後に使った接続へ", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.AddConn("c2", "two", servicetest.URL2, true)
		setLastConnection(a, "c2")
		wantRedirect(t, a.get("/"), http.StatusFound, "/connections/c2")
	})
	t.Run("記録が無い・消えた接続なら先頭", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.AddConn("c2", "two", servicetest.URL2, true)
		setLastConnection(a, "gone")
		wantRedirect(t, a.get("/"), http.StatusFound, "/connections/c1")
	})
}

func TestConnectionRedirect(t *testing.T) {
	t.Run("last_session へ", func(t *testing.T) {
		a := newApp(t)
		c := a.env.AddConn("c1", "one", servicetest.URL1, true)
		c.LastSession = "1"
		a.env.Store.Mu.Lock()
		a.env.Store.Conns[0] = c
		a.env.Store.Mu.Unlock()
		a.env.Report.SetSessions("c1", "0", "1")
		wantRedirect(t, a.get("/connections/c1"), http.StatusFound, "/connections/c1/sessions/1")
	})
	t.Run("許可外・未記録は先頭の許可セッションへ", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.Report.SetSessions("c1", "0", "1")
		wantRedirect(t, a.get("/connections/c1"), http.StatusFound, "/connections/c1/sessions/0")
	})
	t.Run("未知の cid は 404", func(t *testing.T) {
		a := newApp(t)
		body := wantStatus(t, a.get("/connections/nope"), http.StatusNotFound)
		webtest.MustContain(t, body, "ページが見つかりません")
		if !isFullPage(body) {
			t.Fatal("HX なしの 404 はフルページ")
		}
		if hxBody := wantStatus(t, a.get("/connections/nope", hx()), http.StatusNotFound); isFullPage(hxBody) {
			t.Fatal("HX の 404 はフラグメント")
		}
	})
	t.Run("許可セッション 0 件は専用ページ", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.Report.SetSessions("c1")
		full := wantStatus(t, a.get("/connections/c1"), http.StatusOK)
		webtest.MustContain(t, full, "許可されたセッションがありません", "devbox", "<html")
		frag := wantStatus(t, a.get("/connections/c1", hx()), http.StatusOK)
		webtest.MustContain(t, frag, "許可されたセッションがありません")
		webtest.MustNotContain(t, frag, "<html")
	})
	t.Run("401 は再ペアリングへ誘導", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "unauthorized"}
		body := wantStatus(t, a.get("/connections/c1"), http.StatusOK)
		webtest.MustContain(t, body, "再ペアリング", `href="/pairing"`)
		if !a.env.Conns.NeedsRepair("c1") {
			t.Fatal("needs_repair が立っていない")
		}
	})
	t.Run("不達でも前回のセッションがあれば表示先へ", func(t *testing.T) {
		a := newApp(t)
		c := a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		c.LastSession = "0"
		a.env.Store.Mu.Lock()
		a.env.Store.Conns[0] = c
		a.env.Store.Mu.Unlock()
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		wantRedirect(t, a.get("/connections/c1"), http.StatusFound, "/connections/c1/sessions/0")
	})
	t.Run("不達で前回も無ければ再試行の導線", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		body := wantStatus(t, a.get("/connections/c1", hx()), http.StatusOK)
		webtest.MustContain(t, body, "再試行", `hx-get="/connections/c1"`)
		webtest.MustNotContain(t, body, "<html")
	})
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
	t.Run("許可外のセッションは先頭へ 302（記録しない）", func(t *testing.T) {
		a := setup(t)
		wantRedirect(t, a.get("/connections/c1/sessions/zzz"), http.StatusFound, "/connections/c1/sessions/0")
		a.env.Store.Mu.Lock()
		defer a.env.Store.Mu.Unlock()
		if got := a.env.Store.Conns[0].LastSession; got != "" {
			t.Fatalf("last_session = %q", got)
		}
	})
	t.Run("セッション名は URL エンコードを復元する", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		a.env.Report.SetSessions("c1", "my session")
		body := wantStatus(t, a.get("/connections/c1/sessions/my%20session", hx()), http.StatusOK)
		webtest.MustContain(t, body, `hx-get="/connections/c1/sessions/my%20session/snapshot"`)
	})
	t.Run("許可セッション 0 件", func(t *testing.T) {
		a := setup(t)
		a.env.Report.SetSessions("c1")
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "許可されたセッションがありません")
	})
	t.Run("未知の接続は 404", func(t *testing.T) {
		a := setup(t)
		wantStatus(t, a.get("/connections/nope/sessions/0"), http.StatusNotFound)
	})
	t.Run("タブを取得できなくても 200 で前回値と失敗バナー", func(t *testing.T) {
		a := setup(t)
		a.env.Store.Mu.Lock()
		a.env.Store.Snaps[servicetest.SnapKey{Conn: "c1", Session: "0"}] = domain.Snapshot{
			Report:    servicetest.ReportOf(servicetest.Pane("%1", "T1", domain.StateWorking)),
			FetchedAt: a.env.Clock.Now().Add(-time.Minute),
		}
		a.env.Store.Mu.Unlock()
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		body := wantStatus(t, a.get("/connections/c1/sessions/0"), http.StatusOK)
		webtest.MustContain(t, body, "opacity-50", `data-error-kind="`, "前回取得", `hx-get="/connections/c1/sessions/0/snapshot"`)
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
