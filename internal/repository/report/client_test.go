package report

import (
	"errors"
	"slices"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// 成功系は stateless サーバー（POST のみ・GET/DELETE は 405）に対して、単発 JSON 応答と SSE 応答の両方で通す。
func eachResponseMode(t *testing.T, f func(t *testing.T, jsonResponse bool)) {
	t.Helper()
	for name, jsonResponse := range map[string]bool{"json": true, "sse": false} {
		t.Run(name, func(t *testing.T) { f(t, jsonResponse) })
	}
}

func rpcMethods(l *requestLog) []string {
	var out []string
	for _, r := range l.all() {
		out = append(out, r.HTTPMethod+" "+r.RPCMethod)
	}
	return out
}

func TestSessionsReturnsAllowedSessionsInOrder(t *testing.T) {
	eachResponseMode(t, func(t *testing.T, jsonResponse bool) {
		ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: jsonResponse, tools: okTools()})
		c, _ := newTestClient(t)

		got, err := c.Sessions(t.Context(), testConn(ts))
		if err != nil {
			t.Fatalf("Sessions: %v", err)
		}
		want := []domain.Session{{ID: "0", Allowed: true}, {ID: "2", Allowed: true}}
		if !slices.Equal(got, want) {
			t.Fatalf("sessions = %+v, want %+v", got, want)
		}
		// GET（常駐 SSE）・DELETE・tools/list・server/discover は送らない。
		wantWire := []string{"POST initialize", "POST notifications/initialized", "POST tools/call"}
		if gotWire := rpcMethods(reqs); !slices.Equal(gotWire, wantWire) {
			t.Fatalf("wire = %v, want %v", gotWire, wantWire)
		}
	})
}

func TestSessionsEmptyAllowedList(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{
		toolSessionsList: {text: `{"sessions":[{"id":"1","allowed":false}]}`},
	}})
	c, _ := newTestClient(t)
	got, err := c.Sessions(t.Context(), testConn(ts))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Sessions = %v, %v; want empty non-nil, nil", got, err)
	}
}

func TestReportSendsFormatJSONAndExplicitSession(t *testing.T) {
	eachResponseMode(t, func(t *testing.T, jsonResponse bool) {
		ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: jsonResponse, tools: okTools()})
		c, _ := newTestClient(t)

		rep, err := c.Report(t.Context(), testConn(ts), "0")
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		if len(rep.Panes) != 2 || rep.Panes[0].ID != "%1" || rep.Panes[0].State != domain.StateAsking {
			t.Fatalf("report not parsed: %+v", rep.Panes)
		}
		var call *recordedRequest
		for _, r := range reqs.all() {
			if r.RPCMethod == "tools/call" {
				call = &r
			}
		}
		if call == nil || call.Tool != "report" {
			t.Fatalf("no report call: %+v", reqs.all())
		}
		if len(call.Args) != 2 || call.Args["format"] != "json" || call.Args["session"] != "0" {
			t.Fatalf("report args = %v, want exactly format=json, session=0", call.Args)
		}
	})
}

func TestSessionsSendsEmptyArguments(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	if _, err := c.Sessions(t.Context(), testConn(ts)); err != nil {
		t.Fatal(err)
	}
	for _, r := range reqs.all() {
		if r.RPCMethod == "tools/call" && (r.Tool != "sessions_list" || len(r.Args) != 0) {
			t.Fatalf("sessions_list call = %+v, want empty args", r)
		}
	}
}

func TestReportWithoutSessionDoesNotCallTheNetwork(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	_, err := c.Report(t.Context(), testConn(ts), "")
	_ = assertFetchError(t, err, domain.ErrSessionRequired)
	if n := reqs.count(); n != 0 {
		t.Fatalf("%d requests reached the server", n)
	}
}

func TestReportInvalidPayloadIsProtocolError(t *testing.T) {
	for name, text := range map[string]string{"not json": "<html>", "array": `[1]`} {
		t.Run(name, func(t *testing.T) {
			ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{
				toolReport: {text: text},
			}})
			c, _ := newTestClient(t)
			_, err := c.Report(t.Context(), testConn(ts), "0")
			_ = assertFetchError(t, err, domain.ErrProtocol)
		})
	}
}

func TestProbeReturnsAllowedSessionsOnly(t *testing.T) {
	eachResponseMode(t, func(t *testing.T, jsonResponse bool) {
		ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: jsonResponse, tools: okTools()})
		c, _ := newTestClient(t)

		got, err := c.Probe(t.Context(), reportURL(ts), testToken)
		if err != nil {
			t.Fatalf("Probe: %v", err)
		}
		ids := make([]string, 0, len(got))
		for _, s := range got {
			if !s.Allowed {
				t.Errorf("disallowed session returned: %+v", s)
			}
			ids = append(ids, s.ID)
		}
		if !slices.Equal(ids, []string{"0", "2"}) {
			t.Fatalf("ids = %v", ids)
		}
		for _, r := range reqs.all() {
			if r.RPCMethod == "tools/call" && r.Tool != "sessions_list" {
				t.Fatalf("Probe called %q", r.Tool)
			}
		}
	})
}

func TestProbeDoesNotUseSecretStore(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c := NewClient(failingTokens{}, nil)
	if _, err := c.Probe(t.Context(), reportURL(ts), testToken); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if reqs.count() == 0 {
		t.Fatal("no request sent")
	}
}

func TestBearerHeaderOnEveryRequest(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, _ := newTestClient(t)
	if _, err := c.Report(t.Context(), testConn(ts), "0"); err != nil {
		t.Fatal(err)
	}
	all := reqs.all()
	if len(all) == 0 {
		t.Fatal("no requests")
	}
	for _, r := range all {
		if r.Auth != "Bearer "+testToken {
			t.Errorf("%s %s: Authorization = %q", r.HTTPMethod, r.RPCMethod, r.Auth)
		}
	}
}

func TestClientIsReusableAcrossCalls(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: false, tools: okTools()})
	c, _ := newTestClient(t)
	for range 3 {
		if _, err := c.Sessions(t.Context(), testConn(ts)); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Report(t.Context(), testConn(ts), "0"); err != nil {
			t.Fatal(err)
		}
	}
}

// failingTokens は呼ばれたらテストを失敗させる TokenStore。
type failingTokens struct{}

func (failingTokens) Get(string) (string, error) { return "", errors.New("must not be called") }

func assertFetchError(t *testing.T, err error, kind domain.ErrKind) *domain.FetchError {
	t.Helper()
	var fe *domain.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v (%T), want *domain.FetchError", err, err)
	}
	if fe.Kind != kind {
		t.Fatalf("kind = %v (code %q), want %v", fe.Kind, fe.Code, kind)
	}
	return fe
}
