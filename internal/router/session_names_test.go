package router_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// `/`・空白・`%` を含むセッション名は、paths.Session がエスケープした URL（%2F など）で、
// メイン画面にも snapshot にも届き、ルートのパラメータは復号される。
func TestRouterSessionNamesWithSpecialCharacters(t *testing.T) {
	console := newStubConsole(t)
	h, db, _ := newSmokeHandler(t)
	c := &client{t: t, h: h}
	hx := map[string]string{"HX-Request": "true"}

	page := c.expect(c.do(http.MethodGet, "/pairing", nil, nil), http.StatusOK)
	m := csrfRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("CSRF トークンが配られていない")
	}
	c.csrf = m[1] + m[2]

	pairStr := "swarm-viewer://pair?v=1&url=" + url.QueryEscape(console.URL+"/report") + "&token=" + smokeToken + "&name=stubbox"
	w := c.do(http.MethodPost, "/pairing", hx, url.Values{"pairing": {pairStr}})
	c.expect(w, http.StatusOK)
	w = c.do(http.MethodPost, w.Header().Get("HX-Redirect")+"/confirm", hx, nil)
	c.expect(w, http.StatusOK)
	cid := strings.TrimPrefix(w.Header().Get("HX-Redirect"), "/connections/")
	if cid == "" {
		t.Fatal("接続を保存できなかった")
	}

	for _, sid := range []string{"feature/x", "my session", "100%"} {
		sessionPath := paths.Session(cid, sid)
		if sid == "feature/x" && !strings.Contains(sessionPath, "feature%2Fx") {
			t.Fatalf("paths.Session = %q", sessionPath)
		}
		main := c.expect(c.do(http.MethodGet, sessionPath, nil, nil), http.StatusOK)
		mustContain(t, main, "<html", `hx-get="`+sessionPath+`/snapshot"`)

		snap := c.expect(c.do(http.MethodGet, paths.Snapshot(cid, sid), hx, nil), http.StatusOK)
		mustContain(t, snap, "pane-title-"+sid)

		if got, err := db.ConnectionByID(t.Context(), cid); err != nil || got.LastSession != sid {
			t.Fatalf("last_session = %q err=%v, want %q", got.LastSession, err, sid)
		}
	}
}
