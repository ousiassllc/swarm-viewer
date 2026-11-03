package report

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// assertNoLeak は token・応答本文のメッセージ・URL がエラーとログに出ていないことを確かめる。
func assertNoLeak(t *testing.T, err error, logs *bytes.Buffer, ts *httptest.Server) {
	t.Helper()
	forbidden := []string{testToken, secretMessage, "Bearer", "Authorization", ts.URL, strings.TrimPrefix(ts.URL, "http://")}
	text := logs.String()
	if err != nil {
		text += "\n" + err.Error()
	}
	for _, f := range forbidden {
		if strings.Contains(text, f) {
			t.Errorf("output leaks %q:\n%s", f, text)
		}
	}
}

func TestHTTPErrorsAreClassified(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		kind   domain.ErrKind
	}{
		{"401", http.StatusUnauthorized, "unauthorized", domain.ErrUnauthorized},
		{"403", http.StatusForbidden, "forbidden_scope", domain.ErrForbiddenScope},
		{"404", http.StatusNotFound, "not_found", domain.ErrProtocol},
		{"405", http.StatusMethodNotAllowed, "method_not_allowed", domain.ErrProtocol},
		{"413", http.StatusRequestEntityTooLarge, "too_large", domain.ErrProtocol},
		{"500", http.StatusInternalServerError, "weird-Code!", domain.ErrProtocol},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeHTTPError(w, tc.status, tc.code)
			})
			c, logs := newTestClient(t)

			for name, call := range map[string]func() error{
				"Sessions": func() error { _, err := c.Sessions(t.Context(), testConn(ts)); return err },
				"Report":   func() error { _, err := c.Report(t.Context(), testConn(ts), "0"); return err },
				"Probe":    func() error { _, err := c.Probe(t.Context(), reportURL(ts), testToken); return err },
			} {
				err := call()
				fe := assertFetchError(t, err, tc.kind)
				if tc.kind != domain.ErrProtocol || tc.status != http.StatusInternalServerError {
					if fe.Code != tc.code {
						t.Errorf("%s: code = %q, want %q", name, fe.Code, tc.code)
					}
				} else if fe.Code != "http_500" { // 不正なコードは捨ててステータスだけ残す
					t.Errorf("%s: code = %q, want http_500", name, fe.Code)
				}
				assertNoLeak(t, err, logs, ts)
			}
		})
	}
}

func TestHTTPErrorWithoutJSONBody(t *testing.T) {
	ts, _ := newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, secretMessage, http.StatusMethodNotAllowed)
	})
	c, logs := newTestClient(t)
	_, err := c.Sessions(t.Context(), testConn(ts))
	fe := assertFetchError(t, err, domain.ErrProtocol)
	if fe.Code != "http_405" {
		t.Errorf("code = %q", fe.Code)
	}
	assertNoLeak(t, err, logs, ts)
}

func TestUnauthorizedFromServerWithWrongToken(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c := NewClient(tokensOf{testConnID: "test-token-wrong"}, nil)
	_, err := c.Sessions(t.Context(), testConn(ts))
	_ = assertFetchError(t, err, domain.ErrUnauthorized)
	if reqs.count() != 1 { // initialize だけで止まる
		t.Errorf("requests = %d, want 1", reqs.count())
	}
}

type tokensOf map[string]string

func (m tokensOf) Get(id string) (string, error) {
	if v, ok := m[id]; ok {
		return v, nil
	}
	return "", errors.New("unknown connection")
}

func TestToolErrorCodes(t *testing.T) {
	cases := []struct {
		code string
		kind domain.ErrKind
	}{
		{"session_required", domain.ErrSessionRequired},
		{"session_forbidden", domain.ErrSessionForbidden},
		{"invalid_args", domain.ErrSessionMissing},
		{"tmux_error", domain.ErrHostError},
		{"internal", domain.ErrHostError},
		{"brand_new_code", domain.ErrProtocol},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			eachResponseMode(t, func(t *testing.T, jsonResponse bool) {
				ts, _ := newStatelessServer(t, stubOptions{jsonResponse: jsonResponse, tools: map[string]toolReply{
					toolReport:       toolError(tc.code),
					toolSessionsList: toolError(tc.code),
				}})
				c, logs := newTestClient(t)

				_, err := c.Report(t.Context(), testConn(ts), "0")
				fe := assertFetchError(t, err, tc.kind)
				if fe.Code != tc.code {
					t.Errorf("code = %q, want %q", fe.Code, tc.code)
				}
				assertNoLeak(t, err, logs, ts)
				if !strings.Contains(logs.String(), "kind="+tc.kind.String()) {
					t.Errorf("log lacks kind: %s", logs)
				}

				_, err = c.Sessions(t.Context(), testConn(ts))
				_ = assertFetchError(t, err, tc.kind)
			})
		})
	}
}

func TestToolErrorWithUnparseableBodyIsProtocol(t *testing.T) {
	for name, text := range map[string]string{
		"not json":    secretMessage,
		"no code":     `{"error":{"message":"` + secretMessage + `"}}`,
		"empty":       ``,
		"bad code":    `{"error":{"code":"Has Space!"}}`,
		"object code": `{"error":{"code":{"x":1}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{
				toolReport: {isError: true, text: text},
			}})
			c, logs := newTestClient(t)
			_, err := c.Report(t.Context(), testConn(ts), "0")
			_ = assertFetchError(t, err, domain.ErrProtocol)
			assertNoLeak(t, err, logs, ts)
		})
	}
}

func TestSuccessResultWithoutTextIsProtocol(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: map[string]toolReply{
		toolSessionsList: {text: ""},
	}})
	c, _ := newTestClient(t)
	_, err := c.Sessions(t.Context(), testConn(ts))
	_ = assertFetchError(t, err, domain.ErrProtocol)
}

func TestNonMCPResponseIsProtocol(t *testing.T) {
	ts, _ := newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"` + secretMessage + `"}`))
	})
	c, logs := newTestClient(t)
	_, err := c.Sessions(t.Context(), testConn(ts))
	_ = assertFetchError(t, err, domain.ErrProtocol)
	assertNoLeak(t, err, logs, ts)
}

func TestConnectionRefusedIsUnreachable(t *testing.T) {
	ts, _ := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	conn := testConn(ts)
	ts.Close()
	c, logs := newTestClient(t)
	_, err := c.Sessions(t.Context(), conn)
	_ = assertFetchError(t, err, domain.ErrUnreachable)
	assertNoLeak(t, err, logs, ts)
}
