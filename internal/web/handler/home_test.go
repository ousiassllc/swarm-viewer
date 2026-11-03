package handler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

// sessionsListCalls は sessions_list の呼び出し回数。
func sessionsListCalls(a *app, cid string) int {
	n := 0
	for _, e := range a.env.Rec.List() {
		if e == "report.Sessions:"+cid {
			n++
		}
	}
	return n
}

func setLastSession(a *app, session string) {
	a.env.Store.Mu.Lock()
	a.env.Store.Conns[0].LastSession = session
	a.env.Store.Mu.Unlock()
}

func saveSnapshot(a *app, cid, session string) {
	a.env.Store.Mu.Lock()
	a.env.Store.Snaps[servicetest.SnapKey{Conn: cid, Session: session}] = domain.Snapshot{
		Report:    servicetest.ReportOf(servicetest.Pane("%1", "T1", domain.StateWorking)),
		FetchedAt: a.env.Clock.Now().Add(-time.Minute),
	}
	a.env.Store.Mu.Unlock()
}

// 3xx を使わず、`/` が解決した画面をその場に描画する（WebKit の WebView は http.Redirect を扱えない）。
func TestHome(t *testing.T) {
	t.Run("接続が 0 件ならペアリング画面をその場に描画", func(t *testing.T) {
		a := newApp(t)
		w := a.get("/")
		webtest.MustContain(t, wantStatus(t, w, http.StatusOK), "<html", `id="pairing-form"`)
		if loc := w.Header().Get("Location"); loc != "" {
			t.Fatalf("Location = %q", loc)
		}
	})
	t.Run("最後に使った接続のメイン画面", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.AddConn("c2", "two", servicetest.URL2, true)
		a.env.Report.SetSessions("c2", "0")
		setLastConnection(a, "c2")
		w := a.get("/")
		webtest.MustContain(t, wantStatus(t, w, http.StatusOK), "<html", "接続: two")
		if loc := w.Header().Get("Location"); loc != "" {
			t.Fatalf("Location = %q", loc)
		}
	})
	t.Run("記録が無い・消えた接続なら先頭", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.AddConn("c2", "two", servicetest.URL2, true)
		a.env.Report.SetSessions("c1", "0")
		setLastConnection(a, "gone")
		webtest.MustContain(t, wantStatus(t, a.get("/"), http.StatusOK), "接続: one")
	})
}

func TestConnectionPage(t *testing.T) {
	t.Run("last_session をその場に描画（ポーリング URL は正規のパス）", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		setLastSession(a, "1")
		w := a.get("/connections/c1")
		webtest.MustContain(t, wantStatus(t, w, http.StatusOK), "<html", "接続: one", `hx-get="/connections/c1/sessions/1/snapshot"`)
		if loc := w.Header().Get("Location"); loc != "" {
			t.Fatalf("Location = %q", loc)
		}
	})
	t.Run("直近のタブに last_session が無ければ先頭の許可セッション", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.Report.SetSessions("c1", "0", "1")
		setLastSession(a, "gone")
		if _, err := a.env.Conns.Tabs(t.Context(), "c1"); err != nil {
			t.Fatal(err)
		}
		body := wantStatus(t, a.get("/connections/c1"), http.StatusOK)
		webtest.MustContain(t, body, `hx-get="/connections/c1/sessions/0/snapshot"`)
		webtest.MustNotContain(t, body, "/sessions/gone")
	})
	t.Run("初回（保存値もキャッシュも無い）は取得して先頭の許可セッション", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "one", servicetest.URL1, true)
		a.env.Report.SetSessions("c1", "0", "1")
		webtest.MustContain(t, wantStatus(t, a.get("/connections/c1"), http.StatusOK), `hx-get="/connections/c1/sessions/0/snapshot"`)
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
	t.Run("前回のセッションがあれば、ホストに触れずに前回値で描画する（FR-POLL-4・NFR-PERF-1）", func(t *testing.T) {
		a := newApp(t)
		a.env.AddConn("c1", "devbox", servicetest.URL1, true)
		setLastSession(a, "0")
		saveSnapshot(a, "c1", "0")
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		body := wantStatus(t, a.get("/connections/c1"), http.StatusOK)
		webtest.MustContain(t, body, "opacity-50", "前回取得", `hx-get="/connections/c1/sessions/0/snapshot"`)
		if n := sessionsListCalls(a, "c1") + a.env.Report.ReportCount(); n != 0 {
			t.Fatalf("初回描画でホストを %d 回呼んだ", n)
		}
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
