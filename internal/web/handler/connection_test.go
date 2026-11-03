package handler_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func connectionsApp(t *testing.T) *app {
	t.Helper()
	a := newApp(t)
	a.env.AddConn("c1", "devbox", servicetest.URL1, true)
	a.env.AddConn("c2", "buildbox", servicetest.URL2, true)
	if err := a.env.Secrets.Set("c1", servicetest.Token); err != nil {
		t.Fatal(err)
	}
	if err := a.env.Secrets.Set("c2", servicetest.Token); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestConnectionsList(t *testing.T) {
	a := connectionsApp(t)
	setLastConnection(a, "c2")

	full := wantStatus(t, a.get("/connections"), http.StatusOK)
	webtest.MustContain(t, full, "<html", `id="connection-list"`, "devbox", "buildbox", "100.64.0.5", "表示中",
		`hx-delete="/connections/c1"`, `hx-confirm=`, `hx-get="/connections/c2"`)
	// URL 全体（path・ポート）は出さない。host のみ。
	webtest.MustNotContain(t, full, "/report", "48120", servicetest.Token)

	frag := wantStatus(t, a.get("/connections", hx()), http.StatusOK)
	webtest.MustContain(t, frag, `id="connection-list"`)
	webtest.MustNotContain(t, frag, "<html")
}

func TestConnectionsListEmpty(t *testing.T) {
	a := newApp(t)
	body := wantStatus(t, a.get("/connections", hx()), http.StatusOK)
	webtest.MustContain(t, body, "保存済みの接続はありません")
}

func TestDeleteConnection(t *testing.T) {
	a := connectionsApp(t)
	setLastConnection(a, "c1")

	body := wantStatus(t, a.do(http.MethodDelete, "/connections/c1", hx()), http.StatusOK)
	// 一覧全体（outerHTML 差し替え）とトースト OOB。
	webtest.MustContain(t, body, `id="connection-list"`, "buildbox", `id="toasts"`, `hx-swap-oob="true"`, "削除しました")
	webtest.MustNotContain(t, body, `hx-delete="/connections/c1"`)

	if _, ok := a.env.Store.Conn("c1"); ok {
		t.Fatal("接続が消えていない")
	}
	if _, ok := a.env.Secrets.Token("c1"); ok {
		t.Fatal("秘密領域の token が消えていない")
	}
	if _, ok := a.env.Secrets.Token("c2"); !ok {
		t.Fatal("他の接続の token を消した")
	}
	// 最後に使った接続を消したら、記録を空に戻す（次回は先頭）。
	a.env.Store.Mu.Lock()
	last := a.env.Store.Cfg.LastConnectionID
	a.env.Store.Mu.Unlock()
	if last != "" {
		t.Fatalf("last_connection_id = %q", last)
	}
	a.env.Report.SetSessions("c2", "0")
	webtest.MustContain(t, wantStatus(t, a.get("/"), http.StatusOK), "接続: buildbox")
}

func TestDeleteOtherConnectionKeepsLastConnection(t *testing.T) {
	a := connectionsApp(t)
	setLastConnection(a, "c2")
	wantStatus(t, a.do(http.MethodDelete, "/connections/c1", hx()), http.StatusOK)
	a.env.Store.Mu.Lock()
	defer a.env.Store.Mu.Unlock()
	if got := a.env.Store.Cfg.LastConnectionID; got != "c2" {
		t.Fatalf("last_connection_id = %q", got)
	}
}

func TestDeleteConnectionFailureShowsErrorToast(t *testing.T) {
	a := connectionsApp(t)
	a.env.Store.SetErr("DeleteConnection", errors.New("disk full"))
	body := wantStatus(t, a.do(http.MethodDelete, "/connections/c1", hx()), http.StatusOK)
	webtest.MustContain(t, body, "接続を削除できませんでした", "devbox")
	webtest.MustNotContain(t, body, "disk full")
}

func TestDeleteUnknownConnectionIs404(t *testing.T) {
	a := connectionsApp(t)
	wantStatus(t, a.do(http.MethodDelete, "/connections/nope", hx()), http.StatusNotFound)
}
