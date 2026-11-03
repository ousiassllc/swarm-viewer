package handler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

const snapshotURL = "/connections/c1/sessions/0/snapshot"

func snapshotApp(t *testing.T) *app {
	t.Helper()
	a := newApp(t)
	a.env.AddConn("c1", "devbox", servicetest.URL1, true)
	a.env.Report.SetSessions("c1", "0", "1")
	pane := servicetest.Pane("%1", "T1", domain.StateAsking)
	pane.Title = "fix-the-build"
	a.env.Report.SetReport("c1", "0", servicetest.ReportOf(pane))
	return a
}

func TestSnapshotFresh(t *testing.T) {
	a := snapshotApp(t)
	body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, body, "fix-the-build",
		`id="snapshot"`,
		`hx-get="`+snapshotURL+`"`,
		`hx-trigger="every 10s, refresh from:body"`, // 置き換えた要素に load を含めない
		`hx-sync="this:drop"`,
		`id="snapshot-status"`, `hx-swap-oob="true"`)
	webtest.MustNotContain(t, body, "opacity-50", `role="alert"`, "<html")
	if n := a.env.Report.ReportCount(); n != 1 {
		t.Fatalf("report の呼び出し = %d, want 1", n)
	}
	// 取得結果は保存される（FR-POLL-4）。
	a.env.Store.Mu.Lock()
	_, saved := a.env.Store.Snaps[servicetest.SnapKey{Conn: "c1", Session: "0"}]
	a.env.Store.Mu.Unlock()
	if !saved {
		t.Fatal("snapshot が保存されていない")
	}
}

func TestSnapshotPollSecondsFollowSettings(t *testing.T) {
	a := snapshotApp(t)
	a.env.Store.Mu.Lock()
	a.env.Store.Cfg.ScreenPollSeconds = 60
	a.env.Store.Mu.Unlock()
	body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, body, `hx-trigger="every 60s, refresh from:body"`)
}

func TestSnapshotFetchFailureIsStill200(t *testing.T) {
	cases := []struct {
		name string
		err  *domain.FetchError
	}{
		{"不達", &domain.FetchError{Kind: domain.ErrUnreachable}},
		{"互換なし", &domain.FetchError{Kind: domain.ErrProtocol, Code: "http_404"}},
		{"ホスト側エラー", &domain.FetchError{Kind: domain.ErrHostError, Code: "tmux_error"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := snapshotApp(t)
			// 先に成功させて直前値を持たせる。
			wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
			a.env.Clock.Advance(time.Minute)
			a.env.Report.SetReportErr("c1", "0", tc.err)

			body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
			webtest.MustContain(t, body,
				"fix-the-build", // 直前値は残す
				"opacity-50",    // グレーアウト
				`role="alert"`, `hx-swap-oob="true"`,
				"前回取得", "次回再試行 10 秒後",
				`hx-trigger="every 10s, refresh from:body"`) // ポーリングは続ける
		})
	}
}

func TestSnapshotFailureWithoutPreviousIsEmptyState(t *testing.T) {
	a := snapshotApp(t)
	a.env.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: domain.ErrUnreachable})
	body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, body, "表示できるデータがありません", `role="alert"`)
}

func TestSnapshotUnauthorizedStopsPolling(t *testing.T) {
	for _, kind := range []domain.ErrKind{domain.ErrUnauthorized, domain.ErrForbiddenScope} {
		a := snapshotApp(t)
		a.env.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: kind})
		body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
		webtest.MustContain(t, body, "再ペアリング", `href="/pairing"`, `role="alert"`)
		webtest.MustNotContain(t, body, "hx-trigger", `hx-get="`+snapshotURL+`"`)
		if !a.env.Conns.NeedsRepair("c1") {
			t.Fatal("needs_repair が立っていない")
		}
		// 以降は取得せずに（導線なしの）同じ状態を返す。
		before := a.env.Report.ReportCount()
		again := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
		webtest.MustNotContain(t, again, "hx-trigger")
		if a.env.Report.ReportCount() != before {
			t.Fatal("needs_repair の接続を取得した")
		}
	}
}

const goneURL = "/connections/c1/sessions/gone/snapshot"

var sessionErrKinds = []domain.ErrKind{domain.ErrSessionForbidden, domain.ErrSessionMissing, domain.ErrSessionRequired}

// 許可セッションに無いと分かったら HX-Redirect（3xx にしない）で先頭の許可セッションへ。
func TestSnapshotSessionErrorsRedirectToFirstAllowedSession(t *testing.T) {
	for _, kind := range sessionErrKinds {
		a := snapshotApp(t)
		a.env.Report.SetReportErr("c1", "gone", &domain.FetchError{Kind: kind})
		wantHXRedirect(t, a.get(goneURL, hx()), "/connections/c1/sessions/0")
	}
}

func TestSnapshotSessionErrorWithNoAllowedSessionsRedirectsToConnection(t *testing.T) {
	a := snapshotApp(t)
	a.env.Report.SetSessions("c1")
	a.env.Report.SetReportErr("c1", "gone", &domain.FetchError{Kind: domain.ErrSessionMissing})
	wantHXRedirect(t, a.get(goneURL, hx()), "/connections/c1")
}

// 移動しても同じ失敗を繰り返すとき（タブにあるセッション・タブを取得できない）は移動せず、バナーを出す。
func TestSnapshotSessionErrorDoesNotLoopRedirect(t *testing.T) {
	t.Run("セッションはタブにある", func(t *testing.T) {
		a := snapshotApp(t)
		a.env.Report.SetReportErr("c1", "0", &domain.FetchError{Kind: domain.ErrSessionForbidden})
		w := a.get(snapshotURL, hx())
		body := wantStatus(t, w, http.StatusOK)
		if w.Header().Get("HX-Redirect") != "" {
			t.Fatal("HX-Redirect を返した")
		}
		webtest.MustContain(t, body, `role="alert"`, `hx-trigger="every 10s, refresh from:body"`)
	})
	t.Run("タブを取得できない", func(t *testing.T) {
		a := snapshotApp(t)
		a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
		a.env.Report.SetReportErr("c1", "gone", &domain.FetchError{Kind: domain.ErrSessionMissing})
		w := a.get(goneURL, hx())
		body := wantStatus(t, w, http.StatusOK)
		if w.Header().Get("HX-Redirect") != "" {
			t.Fatal("HX-Redirect を返した")
		}
		webtest.MustContain(t, body, `role="alert"`)
	})
}

// 初回描画は保存値のタブ（1 件）で出すため、最初の snapshot 応答がタブバーを OOB で差し替える。
func TestSnapshotRefreshesTabsByOOB(t *testing.T) {
	a := snapshotApp(t)
	body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, body, `id="session-tabs"`, `hx-swap-oob="true"`,
		`hx-get="/connections/c1/sessions/0"`, `hx-get="/connections/c1/sessions/1"`)
	if n := sessionsListCalls(a, "c1"); n != 1 {
		t.Fatalf("sessions_list = %d, want 1", n)
	}

	// 直近の取得が新しいあいだは引き直さず、タブバーも返さない（ポーリングのたびに sessions_list を呼ばない）。
	a.env.Clock.Advance(10 * time.Second)
	again := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustNotContain(t, again, `id="session-tabs"`)
	if n := sessionsListCalls(a, "c1"); n != 1 {
		t.Fatalf("sessions_list = %d, want 1", n)
	}

	// 古くなったら引き直す。
	a.env.Report.SetSessions("c1", "0", "1", "2")
	a.env.Clock.Advance(time.Minute)
	later := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, later, `id="session-tabs"`, `hx-get="/connections/c1/sessions/2"`)
}

func TestSnapshotTabsFailureStillRendersRegion(t *testing.T) {
	a := snapshotApp(t)
	a.env.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
	body := wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	webtest.MustContain(t, body, "fix-the-build", `id="snapshot"`)
	webtest.MustNotContain(t, body, `id="session-tabs"`)
}

func TestSnapshotNeedsRepairDoesNotFetchTabs(t *testing.T) {
	a := snapshotApp(t)
	a.env.Conns.MarkNeedsRepair("c1")
	wantStatus(t, a.get(snapshotURL, hx()), http.StatusOK)
	if n := sessionsListCalls(a, "c1"); n != 0 {
		t.Fatalf("sessions_list = %d, want 0", n)
	}
}

func TestSnapshotUnknownConnectionIs404(t *testing.T) {
	a := newApp(t)
	wantStatus(t, a.get(snapshotURL, hx()), http.StatusNotFound)
}
