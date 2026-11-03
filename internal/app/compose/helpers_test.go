package compose

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	_ "modernc.org/sqlite" // テストで user_version を書き換える

	"github.com/ousiassllc/swarm-viewer/internal/app/applog"
	"github.com/ousiassllc/swarm-viewer/internal/app/swap"
	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

var csrfRe = regexp.MustCompile(`X-CSRF-Token&#34;:&#34;([^&]+)&#34;|X-CSRF-Token":"([^"]+)"`)

type env struct {
	rt      *Runtime
	handler *swap.Handler
	dbPath  string
	clock   *servicetest.Clock
	logging *applog.Logging
	logBuf  *bytes.Buffer
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		handler: &swap.Handler{},
		dbPath:  filepath.Join(t.TempDir(), "viewer.db"),
		clock:   servicetest.NewClock(),
		logBuf:  &bytes.Buffer{},
	}
	e.logging = applog.New(e.logBuf)
	rec := &servicetest.Recorder{}
	e.rt = New(Config{
		DBPath:   e.dbPath,
		Logging:  e.logging,
		Notifier: &servicetest.Notifier{},
		Clock:    e.clock,
		IDs:      &servicetest.IDs{},
		Secrets:  servicetest.NewSecrets(rec),
		Report:   servicetest.NewReport(rec),
		Static:   fstest.MapFS{"js/app.js": {Data: []byte("//")}},
		Handler:  e.handler,
	})
	t.Cleanup(e.rt.Close)
	return e
}

func (e *env) get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func (e *env) csrf(t *testing.T, path string) string {
	t.Helper()
	m := csrfRe.FindStringSubmatch(e.get(path).Body.String())
	if m == nil {
		t.Fatalf("no CSRF token in %s", path)
	}
	return m[1] + m[2]
}

func (e *env) send(method, path, token string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// waitFor は cond が真になるまで上限付きで待つ。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// tickerStarted は通知スケジューラがフェイク Clock に ticker を作ったか。
func tickerStarted(c *servicetest.Clock) bool { return c.TickerCount() > 0 }

func corruptDB(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("this is not a sqlite database, just junk bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func bumpSchema(t *testing.T, path string) {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
}
