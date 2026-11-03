package compose

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/app/applog"
	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/router"
)

func TestBeforeStartHandlerIsUnavailable(t *testing.T) {
	e := newEnv(t)
	if rec := e.get("/"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503 before Start", rec.Code)
	}
}

func TestNormalStart_ServesAppAndRunsSchedulerOnlyAfterReady(t *testing.T) {
	e := newEnv(t)
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec := e.get("/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/pairing" {
		t.Fatalf("GET / = %d %q, want 302 /pairing (no connections)", rec.Code, rec.Header().Get("Location"))
	}
	// Wails の準備前（Ready 前）はスケジューラを起動しない。
	if tickerStarted(e.clock) {
		t.Fatal("scheduler started before Ready")
	}
	e.rt.Ready()
	waitFor(t, "scheduler ticker", func() bool { return tickerStarted(e.clock) })
	e.rt.Ready() // 二重の Ready でスケジューラを二重に起動しない
	e.rt.Close()
}

func TestNormalStart_ParseDeepLinkCreatesDraft(t *testing.T) {
	e := newEnv(t)
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	link := "swarm-viewer://pair?v=1&url=" + url.QueryEscape("http://100.64.0.1:48120/report") + "&token=test-token-not-secret"
	id, err := e.rt.Parse(link)
	if err != nil || id == "" {
		t.Fatalf("Parse = %q, %v", id, err)
	}
	rec := e.get("/pairing/drafts/" + string(id))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "test-token-not-secret") {
		t.Fatalf("draft page = %d (token must not be rendered)", rec.Code)
	}
}

func TestStartInRecoveryMode(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*testing.T, string)
		reason router.RecoveryReason
	}{
		{"corrupt", corruptDB, router.RecoveryCorrupt},
		{"schema too new", bumpSchema, router.RecoverySchemaTooNew},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.break_(t, e.dbPath)
			before, err := os.ReadFile(e.dbPath)
			if err != nil {
				t.Fatal(err)
			}

			if err := e.rt.Start(t.Context()); err != nil {
				t.Fatalf("Start must not fail in recovery mode: %v", err)
			}
			rec := e.get("/")
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/recovery/reset") {
				t.Fatalf("GET / = %d, want the recovery page with the reset action", rec.Code)
			}
			// 画面は理由ごとに案内が変わる（router.RecoveryReason が届いている）。
			if got := rec.Body.String(); c.reason == router.RecoverySchemaTooNew && !strings.Contains(got, "新し") {
				t.Fatalf("schema-too-new guidance missing:\n%s", got)
			}
			// recovery モードでは通知スケジューラを起動せず、ディープリンクも受けない。
			e.rt.Ready() // recovery モードでは store が無いので、ここで同期的に何も起動しない
			if tickerStarted(e.clock) {
				t.Fatal("scheduler must not run in recovery mode")
			}
			if _, err := e.rt.Parse("swarm-viewer://pair?v=1"); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("Parse in recovery mode = %v, want ErrUnavailable", err)
			}
			// 起動しただけでは DB に触らない。
			if after, _ := os.ReadFile(e.dbPath); !bytes.Equal(before, after) {
				t.Fatal("database was modified by starting in recovery mode")
			}
		})
	}
}

func TestRecoveryResetSwitchesToNormalMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		brk  func(*testing.T, string)
	}{{"corrupt", corruptDB}, {"schema too new", bumpSchema}} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			tc.brk(t, e.dbPath)
			if err := e.rt.Start(t.Context()); err != nil {
				t.Fatal(err)
			}
			e.rt.Ready()
			token := e.csrf(t, "/")

			rec := e.send(http.MethodPost, "/recovery/reset", token, nil)
			if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/" {
				t.Fatalf("reset = %d HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
			}
			// 通常モードに切り替わった: / は /pairing へ、設定を保存でき、スケジューラが動く。
			if rec := e.get("/"); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/pairing" {
				t.Fatalf("after reset GET / = %d %q", rec.Code, rec.Header().Get("Location"))
			}
			waitFor(t, "scheduler after reset", func() bool { return tickerStarted(e.clock) })

			// 古い DB は削除せず退避されている（新しい版のアプリで読み直せる）。
			backups, _ := filepath.Glob(e.dbPath + ".bak-*")
			if len(backups) != 1 {
				t.Fatalf("backups = %v, want exactly 1", backups)
			}
			// 通常モードの再 reset は DB を退避しない（二重送信で動いている DB を消さない）。
			if err := e.rt.reset(context.Background()); err != nil {
				t.Fatalf("second reset = %v", err)
			}
			if again, _ := filepath.Glob(e.dbPath + ".bak-*"); len(again) != 1 {
				t.Fatalf("backups after second reset = %v", again)
			}
		})
	}
}

func TestSettingsDebugLogTogglesLevel(t *testing.T) {
	e := newEnv(t)
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if e.logging.Level.Level() != applog.LevelFor(false) {
		t.Fatalf("initial level = %v, want warn", e.logging.Level.Level())
	}
	token := e.csrf(t, "/pairing")
	form := url.Values{"screen_poll_seconds": {"10"}, "notify_poll_minutes": {"10"}, "notifications_enabled": {"on"}, "debug_log": {"on"}}
	if rec := e.send(http.MethodPut, "/settings", token, form); rec.Code != http.StatusOK {
		t.Fatalf("PUT /settings = %d", rec.Code)
	}
	waitFor(t, "debug level on", func() bool { return e.logging.Level.Level() == applog.LevelFor(true) })

	form.Del("debug_log")
	if rec := e.send(http.MethodPut, "/settings", token, form); rec.Code != http.StatusOK {
		t.Fatalf("PUT /settings = %d", rec.Code)
	}
	waitFor(t, "debug level off", func() bool { return e.logging.Level.Level() == applog.LevelFor(false) })
}

func TestCloseIsIdempotentAndStopsWorkers(t *testing.T) {
	e := newEnv(t)
	if err := e.rt.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.rt.Ready()
	waitFor(t, "scheduler", func() bool { return tickerStarted(e.clock) })
	e.rt.Close()
	e.rt.Close()
	if !e.clock.Ticker(0).Stopped() {
		t.Fatal("scheduler ticker must be stopped after Close")
	}
}

func TestClassify(t *testing.T) {
	if r, ok := Classify(store.ErrSchemaTooNew); !ok || r != router.RecoverySchemaTooNew {
		t.Fatalf("schema too new = %v, %v", r, ok)
	}
	if r, ok := Classify(store.ErrCorrupt); !ok || r != router.RecoveryCorrupt {
		t.Fatalf("corrupt = %v, %v", r, ok)
	}
	if _, ok := Classify(context.Canceled); ok {
		t.Fatal("unrelated errors must not enter recovery mode")
	}
}
