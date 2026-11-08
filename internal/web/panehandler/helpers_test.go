package panehandler_test

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

const (
	pathReport = "/report"
	pathOps    = "/ops"
)

// opsOnlyTools は操作の 6 ツール。見るだけの接続では 1 回も呼ばれてはいけない。
var opsOnlyTools = []string{"ask_pick", "send_text", "send_escape", "pane_start", "pane_close", "dir_list"}

func body(t *testing.T, w *httptest.ResponseRecorder, status int) string {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d\n%s", w.Code, status, w.Body.String())
	}
	return w.Body.String()
}

func mustContain(t *testing.T, html string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(html, w) {
			t.Errorf("%q が含まれない:\n%s", w, html)
		}
	}
}

func mustNotContain(t *testing.T, html string, banned ...string) {
	t.Helper()
	for _, b := range banned {
		if strings.Contains(html, b) {
			t.Errorf("%q が含まれている:\n%s", b, html)
		}
	}
}

// live は pos のペインの本文（/live）を取得して返す。
func live(t *testing.T, s *opsconsole.Stack, pos string) string {
	t.Helper()
	return body(t, s.Get(opsconsole.PanePath(pos, "/live")), 200)
}

func form(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

// toolNames は記録した tools/call のツール名（順序のまま）。
func toolNames(reqs []opsconsole.Request) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.Tool
	}
	return out
}
