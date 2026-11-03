package servicetest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

func TestConnectionDeleteRemovesRowAndSecret(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Secrets.Tokens["c1"] = Token

	if err := e.Conns.Delete(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.Store.Conn("c1"); ok {
		t.Fatal("connection still stored")
	}
	if _, ok := e.Secrets.Token("c1"); ok {
		t.Fatal("token still stored")
	}
}

// 秘密領域の削除に失敗したら接続を残し、利用者が再試行できる（token だけが孤立して残らない）。
func TestConnectionDeleteSecretFailureKeepsConnectionAndAllowsRetry(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Secrets.Tokens["c1"] = Token
	e.Secrets.ErrDelete = ErrInjected

	if err := e.Conns.Delete(context.Background(), "c1"); !errors.Is(err, ErrInjected) {
		t.Fatalf("err = %v, want the secret error", err)
	}
	if _, ok := e.Store.Conn("c1"); !ok {
		t.Fatal("connection must stay when the secret could not be deleted")
	}
	if logs := e.Logs.String(); strings.Contains(logs, Token) {
		t.Fatal("token leaked into log")
	}

	e.Secrets.ErrDelete = nil
	if err := e.Conns.Delete(context.Background(), "c1"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, ok := e.Store.Conn("c1"); ok {
		t.Fatal("connection still stored after retry")
	}
	if _, ok := e.Secrets.Token("c1"); ok {
		t.Fatal("token still stored after retry")
	}
}

// 行の削除に失敗した場合 token は既に消えているので、接続は needs_repair になり、再削除で片付けられる。
func TestConnectionDeleteStoreFailureLeavesNeedsRepairAndAllowsRetry(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Secrets.Tokens["c1"] = Token
	e.Store.SetErr("DeleteConnection", ErrInjected)

	if err := e.Conns.Delete(context.Background(), "c1"); !errors.Is(err, ErrInjected) {
		t.Fatalf("err = %v", err)
	}
	if _, ok := e.Store.Conn("c1"); !ok {
		t.Fatal("connection must stay when the row delete failed")
	}
	if _, ok := e.Secrets.Token("c1"); ok {
		t.Fatal("token must not linger once the delete was attempted")
	}
	if !e.Conns.NeedsRepair("c1") {
		t.Fatal("a connection without its token must be shown as needs_repair")
	}

	e.Store.SetErr("DeleteConnection", nil)
	if err := e.Conns.Delete(context.Background(), "c1"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, ok := e.Store.Conn("c1"); ok {
		t.Fatal("connection still stored after retry")
	}
	if e.Conns.NeedsRepair("c1") {
		t.Fatal("needs_repair must be released after deletion")
	}
}

func TestConnectionList(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "a", URL1, true)
	e.AddConn("c2", "b", URL2, false)
	got, err := e.Conns.List(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %v, %v", got, err)
	}
	e.Store.SetErr("Connections", ErrInjected)
	if _, err := e.Conns.List(context.Background()); !errors.Is(err, ErrInjected) {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectionGetNotFound(t *testing.T) {
	e := NewEnv(t)
	if _, err := e.Conns.Get(context.Background(), "missing"); !errors.Is(err, service.ErrConnectionNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := e.Conns.Tabs(context.Background(), "missing"); !errors.Is(err, service.ErrConnectionNotFound) {
		t.Fatalf("Tabs err = %v", err)
	}
}

func TestConnectionTabsFiltersAllowedAndCaches(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Report.Mu.Lock()
	e.Report.SessionList["c1"] = []domain.Session{{ID: "0", Allowed: true}, {ID: "stub", Allowed: false}, {ID: "1", Allowed: true}, {ID: "", Allowed: true}}
	e.Report.Mu.Unlock()

	tabs, err := e.Conns.Tabs(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 2 || tabs[0].ID != "0" || tabs[1].ID != "1" {
		t.Fatalf("tabs = %+v", tabs)
	}
	cached, at, ok := e.Conns.CachedTabs("c1")
	if !ok || len(cached) != 2 || !at.Equal(e.Clock.Now()) {
		t.Fatalf("cache = %+v %v %v", cached, at, ok)
	}
	// 呼び出しごとに取得する（キャッシュを返さない）。
	if _, err := e.Conns.Tabs(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range e.Rec.List() {
		if c == "report.Sessions:c1" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("sessions_list called %d times, want 2", n)
	}
}

func TestConnectionTabsAuthErrorsSetNeedsRepair(t *testing.T) {
	cases := map[string]struct {
		err    error
		repair bool
	}{
		"unauthorized":    {&domain.FetchError{Kind: domain.ErrUnauthorized, Code: "unauthorized"}, true},
		"forbidden scope": {&domain.FetchError{Kind: domain.ErrForbiddenScope, Code: "forbidden_scope"}, true},
		"unreachable":     {&domain.FetchError{Kind: domain.ErrUnreachable}, false},
		"host error":      {&domain.FetchError{Kind: domain.ErrHostError, Code: "internal"}, false},
		"plain error":     {ErrInjected, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEnv(t)
			e.AddConn("c1", "devbox", URL1, true)
			e.Report.SessionsErr["c1"] = tc.err

			_, err := e.Conns.Tabs(context.Background(), "c1")

			var fe *domain.FetchError
			if !errors.As(err, &fe) {
				t.Fatalf("err = %v, want *domain.FetchError", err)
			}
			if got := e.Conns.NeedsRepair("c1"); got != tc.repair {
				t.Fatalf("NeedsRepair = %v, want %v", got, tc.repair)
			}
		})
	}
}

func TestConnectionMarkRepaired(t *testing.T) {
	e := NewEnv(t)
	e.Conns.MarkNeedsRepair("c1")
	if !e.Conns.NeedsRepair("c1") {
		t.Fatal("want needs_repair")
	}
	e.Conns.MarkRepaired("c1")
	if e.Conns.NeedsRepair("c1") {
		t.Fatal("want repaired")
	}
}

func TestConnectionResolveSession(t *testing.T) {
	cases := map[string]struct {
		sessions  []string
		requested string
		want      string
		wantErr   error
	}{
		"requested allowed":   {[]string{"0", "1"}, "1", "1", nil},
		"requested missing":   {[]string{"0", "1"}, "gone", "0", nil},
		"requested empty":     {[]string{"0", "1"}, "", "0", nil},
		"single":              {[]string{"0"}, "0", "0", nil},
		"no allowed sessions": {nil, "0", "", service.ErrNoSessions},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEnv(t)
			e.AddConn("c1", "devbox", URL1, true)
			e.Report.SetSessions("c1", tc.sessions...)

			got, err := e.Conns.ResolveSession(context.Background(), "c1", tc.requested)

			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("got %q, %v; want %q, %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestConnectionResolveSessionIgnoresNotAllowedRequested(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Report.Mu.Lock()
	e.Report.SessionList["c1"] = []domain.Session{{ID: "stub", Allowed: false}, {ID: "0", Allowed: true}}
	e.Report.Mu.Unlock()

	got, err := e.Conns.ResolveSession(context.Background(), "c1", "stub")
	if err != nil || got != "0" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestConnectionResolveSessionOnlyStubsIsNoSessions(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Report.Mu.Lock()
	e.Report.SessionList["c1"] = []domain.Session{{ID: "stub", Allowed: false}}
	e.Report.Mu.Unlock()
	if _, err := e.Conns.ResolveSession(context.Background(), "c1", "stub"); !errors.Is(err, service.ErrNoSessions) {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectionResolveSessionFetchErrorSetsRepair(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnauthorized}
	_, err := e.Conns.ResolveSession(context.Background(), "c1", "0")
	var fe *domain.FetchError
	if !errors.As(err, &fe) || !e.Conns.NeedsRepair("c1") {
		t.Fatalf("err=%v repair=%v", err, e.Conns.NeedsRepair("c1"))
	}
}

func TestConnectionRememberSessionAndLastConnection(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	ctx := context.Background()
	if err := e.Conns.RememberSession(ctx, "c1", "1"); err != nil {
		t.Fatal(err)
	}
	if c, _ := e.Store.Conn("c1"); c.LastSession != "1" {
		t.Fatalf("LastSession = %q", c.LastSession)
	}
	if err := e.Conns.SetLastConnection(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Settings.Get(ctx); st.LastConnectionID != "c1" {
		t.Fatalf("LastConnectionID = %q", st.LastConnectionID)
	}
}

func TestConnectionLogsDoNotContainURL(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "devbox", URL1, true)
	e.Report.SessionsErr["c1"] = &domain.FetchError{Kind: domain.ErrUnreachable}
	_, _ = e.Conns.Tabs(context.Background(), "c1")
	logs := e.Logs.String()
	if strings.Contains(logs, URL1) || strings.Contains(logs, "/report") {
		t.Fatalf("log contains the full URL: %q", logs)
	}
	if !strings.Contains(logs, "100.64.0.5") {
		t.Fatalf("log should identify the host: %q", logs)
	}
}
