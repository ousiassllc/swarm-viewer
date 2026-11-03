package router_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/ousiassllc/swarm-viewer/internal/repository/report"
	"github.com/ousiassllc/swarm-viewer/internal/repository/secret"
	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/router"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/static"
)

var csrfRe = regexp.MustCompile(`X-CSRF-Token&#34;:&#34;([^&]+)&#34;|X-CSRF-Token":"([^"]+)"`)

func TestSmokeEndToEnd(t *testing.T) {
	ctx := t.Context()
	console := newStubConsole(t)
	h, db, secrets := newSmokeHandler(t)
	c := &client{t: t, h: h}
	hx := map[string]string{"HX-Request": "true"}

	// 静的ファイルと OpenAPI（オペレーション 0 本）。
	css := c.do(http.MethodGet, "/static/css/app.css", nil, nil)
	c.expect(css, http.StatusOK)
	c.expect(c.do(http.MethodGet, "/static/nope.js", nil, nil), http.StatusNotFound)
	spec := c.expect(c.do(http.MethodGet, "/api/v1/openapi.json", nil, nil), http.StatusOK)
	var doc struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal([]byte(spec), &doc); err != nil || len(doc.Paths) != 0 {
		t.Fatalf("openapi paths = %v err=%v", doc.Paths, err)
	}

	// 接続 0 件: / はリダイレクトせず（WebKit の WebView 対策）ペアリング画面をその場に描画する。
	// レイアウトが配る CSRF トークンを読む。
	w := c.do(http.MethodGet, "/", nil, nil)
	home := c.expect(w, http.StatusOK)
	mustContain(t, home, "<html", `id="pairing-form"`)
	c.noLocation(w)
	page := c.expect(c.do(http.MethodGet, "/pairing", nil, nil), http.StatusOK)
	m := csrfRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("CSRF トークンがレイアウトに無い:\n%s", page)
	}
	c.csrf = m[1] + m[2]

	// CSRF が無い状態変更は 403。
	noToken := &client{t: t, h: h}
	noToken.expect(noToken.do(http.MethodPost, "/pairing", hx, url.Values{"pairing": {"x"}}), http.StatusForbidden)

	// 貼り付け → 下書き（loopback は任意ポートを許可）。
	pairStr := "swarm-viewer://pair?v=1&url=" + url.QueryEscape(console.URL+"/report") +
		"&token=" + smokeToken + "&name=stubbox&sessions=0%2Cwork"
	w = c.do(http.MethodPost, "/pairing", hx, url.Values{"pairing": {pairStr}})
	c.expect(w, http.StatusOK)
	c.noLocation(w)
	draftPath := w.Header().Get("HX-Redirect")
	if !strings.HasPrefix(draftPath, "/pairing/drafts/") {
		t.Fatalf("Location = %q", draftPath)
	}
	draft := c.expect(c.do(http.MethodGet, draftPath, nil, nil), http.StatusOK)
	mustContain(t, draft, "stubbox", "127.0.0.1", "0, work", draftPath+"/confirm")
	mustNotContain(t, draft, smokeToken)

	// 確定 → HX-Redirect /connections/{cid}。
	w = c.do(http.MethodPost, draftPath+"/confirm", hx, nil)
	c.expect(w, http.StatusOK)
	c.noLocation(w)
	connPath := w.Header().Get("HX-Redirect")
	cid := strings.TrimPrefix(connPath, "/connections/")
	if cid == "" || cid == connPath {
		t.Fatalf("Location = %q", connPath)
	}
	if tok, err := secrets.Get(cid); err != nil || tok != smokeToken {
		t.Fatalf("秘密領域の token が保存されていない: %v", err)
	}

	// / も接続も、リダイレクトせずに先頭の許可セッションのメイン画面（フルページ）をその場に描画する。
	sessionPath := connPath + "/sessions/0"
	w = c.do(http.MethodGet, "/", nil, nil)
	main0 := c.expect(w, http.StatusOK)
	c.noLocation(w)
	w = c.do(http.MethodGet, connPath, nil, nil)
	c.expect(w, http.StatusOK)
	c.noLocation(w)
	main0Conn := c.expect(c.do(http.MethodGet, connPath, nil, nil), http.StatusOK)
	mustContain(t, main0Conn, `hx-get="`+sessionPath+`/snapshot"`)
	mustContain(t, main0, "<html", "stubbox", `hx-get="`+sessionPath+`/snapshot"`, "every 10s",
		`hx-get="`+connPath+`/sessions/work"`) // 許可セッションだけがタブになる
	mustNotContain(t, main0, "hidden")

	// スナップショット: ペイン・利用上限・ホスト負荷。
	snap0 := c.expect(c.do(http.MethodGet, sessionPath+"/snapshot", hx, nil), http.StatusOK)
	mustContain(t, snap0, "pane-title-0", "second-pane", "acct-a", "42", "CPU", "37")
	mustNotContain(t, snap0, "opacity-50", "<html")
	saved, ok, err := db.Snapshot(ctx, cid, "0")
	if err != nil || !ok || len(saved.Report.Panes) != 2 {
		t.Fatalf("snapshot が保存されていない: ok=%v err=%v", ok, err)
	}

	// セッション切替: メイン画面のフラグメント → snapshot。last_session が記録される。
	workPath := connPath + "/sessions/work"
	frag := c.expect(c.do(http.MethodGet, workPath, hx, nil), http.StatusOK)
	mustContain(t, frag, `hx-get="`+workPath+`/snapshot"`)
	mustNotContain(t, frag, "<html")
	snapWork := c.expect(c.do(http.MethodGet, workPath+"/snapshot", hx, nil), http.StatusOK)
	mustContain(t, snapWork, "pane-title-work")
	if got, err := db.ConnectionByID(ctx, cid); err != nil || got.LastSession != "work" {
		t.Fatalf("last_session = %q err=%v", got.LastSession, err)
	}
	if st, err := db.Settings(ctx); err != nil || st.LastConnectionID != cid {
		t.Fatalf("last_connection_id = %q err=%v", st.LastConnectionID, err)
	}
	// 接続を選び直すと last_session をその場に描画する（ポーリング URL は正規のパス）。
	w = c.do(http.MethodGet, connPath, nil, nil)
	again := c.expect(w, http.StatusOK)
	c.noLocation(w)
	mustContain(t, again, `hx-get="`+workPath+`/snapshot"`)

	// 設定の保存。
	body := c.expect(c.do(http.MethodPut, "/settings", hx, url.Values{
		"screen_poll_seconds": {"30"}, "notify_poll_minutes": {"5"}, "notifications_enabled": {"on"},
	}), http.StatusOK)
	mustContain(t, body, "設定を保存しました")
	if st, err := db.Settings(ctx); err != nil || st.ScreenPollSeconds != 30 || st.NotifyPollMinutes != 5 || st.DebugLog {
		t.Fatalf("settings = %+v err=%v", st, err)
	}
	bad := c.expect(c.do(http.MethodPut, "/settings", hx, url.Values{
		"screen_poll_seconds": {"7"}, "notify_poll_minutes": {"5"},
	}), http.StatusUnprocessableEntity)
	mustContain(t, bad, "設定の値が正しくありません")
	mustContain(t, c.expect(c.do(http.MethodGet, workPath+"/snapshot", hx, nil), http.StatusOK), "every 30s")

	// 接続の削除: 一覧・SQLite・秘密領域・スナップショット。
	list := c.expect(c.do(http.MethodGet, "/connections", hx, nil), http.StatusOK)
	mustContain(t, list, "stubbox", "127.0.0.1", `hx-delete="`+connPath+`"`)
	mustNotContain(t, list, "/report")
	del := c.expect(c.do(http.MethodDelete, connPath, hx, nil), http.StatusOK)
	mustContain(t, del, "削除しました", "保存済みの接続はありません")
	if _, err := secrets.Get(cid); err == nil {
		t.Fatal("秘密領域の token が残っている")
	}
	if _, ok, err := db.Snapshot(ctx, cid, "0"); err != nil || ok {
		t.Fatalf("snapshot が残っている: ok=%v err=%v", ok, err)
	}
	if _, ok, err := db.Snapshot(ctx, cid, "work"); err != nil || ok {
		t.Fatalf("snapshot(work) が残っている: ok=%v err=%v", ok, err)
	}
	if got, err := db.Connections(ctx); err != nil || len(got) != 0 {
		t.Fatalf("connections = %v err=%v", got, err)
	}
	w = c.do(http.MethodGet, "/", nil, nil)
	mustContain(t, c.expect(w, http.StatusOK), `id="pairing-form"`)
	c.noLocation(w)

	// どの応答にも token を含めない。
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, b := range c.bodies {
		if strings.Contains(b, smokeToken) {
			t.Errorf("応答 #%d に token が含まれている", i)
		}
	}
}

// newSmokeHandler は本物の service・SQLite・秘密領域（keyring のモック）でルーターを組む。
func newSmokeHandler(t *testing.T) (http.Handler, *store.Store, *secret.Store) {
	t.Helper()
	keyring.MockInit()
	db, err := store.Open(filepath.Join(t.TempDir(), "viewer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	secrets := secret.NewStore()
	log := slog.New(slog.DiscardHandler)
	clock := service.SystemClock{}
	reportClient := report.NewClient(secrets, log)

	settings := service.NewSettingsService(db)
	conns := service.NewConnectionService(reportClient, db, secrets, clock, log)
	notif := service.NewNotificationService(reportClient, db, grantedNotifier{}, clock, settings, conns, log)
	snaps := service.NewSnapshotService(reportClient, db, notif, conns, clock, log)
	pairing := service.NewPairingService(reportClient, db, secrets, &seqIDs{}, clock, conns, log)

	h, err := router.New(router.Deps{
		Logger: log, Settings: settings, Connections: conns, Snapshots: snaps, Pairing: pairing,
		Notifier: grantedNotifier{}, Clock: clock, Static: static.FS(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, db, secrets
}
