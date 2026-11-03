package applog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const (
	fakeToken = "test-token-not-secret"
	fakeURL   = "http://100.64.0.1:48120/report?token=" + fakeToken
)

// logJSON は l でログを 1 件出し、JSON オブジェクトにして返す。
func logJSON(t *testing.T, fn func(*slog.Logger)) (map[string]any, string) {
	t.Helper()
	var buf bytes.Buffer
	lg := New(&buf)
	lg.Level.Set(slog.LevelDebug)
	fn(lg.Logger)
	raw := buf.String()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &m); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, raw)
	}
	return m, raw
}

func TestReplaceAttr_FullyMaskedKeys(t *testing.T) {
	for _, key := range []string{"token", "Token", "authorization", "Authorization", "body", "activity", "note", "password", "payload", "pane"} {
		t.Run(key, func(t *testing.T) {
			m, raw := logJSON(t, func(l *slog.Logger) { l.Info("x", key, "secret-value-"+key) })
			if got := m[key]; got != Redacted {
				t.Fatalf("%s = %v, want %s", key, got, Redacted)
			}
			if strings.Contains(raw, "secret-value-") {
				t.Fatalf("value leaked: %s", raw)
			}
		})
	}
}

func TestReplaceAttr_MaskedRegardlessOfValueKind(t *testing.T) {
	m, raw := logJSON(t, func(l *slog.Logger) {
		l.Info("x", "token", 12345, "body", []byte("pane text"), "note", errors.New("secret note"))
	})
	for _, k := range []string{"token", "body", "note"} {
		if m[k] != Redacted {
			t.Fatalf("%s = %v, want redacted", k, m[k])
		}
	}
	if strings.Contains(raw, "12345") || strings.Contains(raw, "pane text") || strings.Contains(raw, "secret note") {
		t.Fatalf("value leaked: %s", raw)
	}
}

func TestReplaceAttr_URLKeysKeepHostOnly(t *testing.T) {
	m, raw := logJSON(t, func(l *slog.Logger) {
		l.Warn("probe failed", "url", fakeURL, "endpoint", "http://100.64.0.1:48120/report")
	})
	if m["url"] != "100.64.0.1:48120" {
		t.Fatalf("url = %v, want host only", m["url"])
	}
	if m["endpoint"] != "100.64.0.1:48120" {
		t.Fatalf("endpoint = %v, want host only", m["endpoint"])
	}
	if strings.Contains(raw, fakeToken) || strings.Contains(raw, "/report") {
		t.Fatalf("path or token leaked: %s", raw)
	}
}

func TestReplaceAttr_UnparsableURLIsRedacted(t *testing.T) {
	m, _ := logJSON(t, func(l *slog.Logger) { l.Info("x", "url", "not a url "+fakeToken, "link", 42) })
	if m["url"] != Redacted {
		t.Fatalf("url = %v, want redacted", m["url"])
	}
	if m["link"] != Redacted {
		t.Fatalf("link = %v, want redacted", m["link"])
	}
}

func TestReplaceAttr_URLInsideOtherValuesIsShrunk(t *testing.T) {
	err := fmt.Errorf("Get %q: dial tcp: refused", fakeURL)
	m, raw := logJSON(t, func(l *slog.Logger) {
		l.Error("fetch failed", "err", err, "detail", "see "+fakeURL+".")
	})
	if strings.Contains(raw, fakeToken) || strings.Contains(raw, "/report") {
		t.Fatalf("URL detail leaked: %s", raw)
	}
	if got, _ := m["err"].(string); !strings.Contains(got, "http://100.64.0.1:48120") {
		t.Fatalf("err lost the host: %q", got)
	}
	if got, _ := m["detail"].(string); !strings.HasSuffix(got, "http://100.64.0.1:48120.") {
		t.Fatalf("detail = %q, want trailing punctuation kept", got)
	}
}

func TestReplaceAttr_BearerValueIsRedacted(t *testing.T) {
	_, raw := logJSON(t, func(l *slog.Logger) {
		l.Warn("request failed", "header", "Authorization failed for Bearer "+fakeToken)
	})
	if strings.Contains(raw, fakeToken) {
		t.Fatalf("bearer token leaked: %s", raw)
	}
}

func TestReplaceAttr_NonSecretValuesPassThrough(t *testing.T) {
	m, _ := logJSON(t, func(l *slog.Logger) {
		l.Info("ok", "count", 3, "host", "100.64.0.1:48120", "ok", true, "kind", "timeout")
	})
	if m["count"] != float64(3) || m["host"] != "100.64.0.1:48120" || m["ok"] != true || m["kind"] != "timeout" {
		t.Fatalf("unexpected change: %v", m)
	}
}

func TestMessageIsScrubbed(t *testing.T) {
	m, raw := logJSON(t, func(l *slog.Logger) { l.Warn("bad link " + fakeURL) })
	if strings.Contains(raw, fakeToken) {
		t.Fatalf("message leaked: %s", raw)
	}
	if m["msg"] != "bad link http://100.64.0.1:48120" {
		t.Fatalf("msg = %v", m["msg"])
	}
}

func TestWithAttrsAndGroupsAreMasked(t *testing.T) {
	_, raw := logJSON(t, func(l *slog.Logger) {
		l.With("token", fakeToken).WithGroup("conn").With("url", fakeURL).Info("x", "body", "pane")
	})
	if strings.Contains(raw, fakeToken) || strings.Contains(raw, "/report") || strings.Contains(raw, "pane\"") {
		t.Fatalf("leaked through With/Group: %s", raw)
	}
}

func TestHostOnly(t *testing.T) {
	cases := map[string]string{
		"http://100.64.0.1:48120/report?token=x": "100.64.0.1:48120",
		"  http://[fd7a:115c:a1e0::1]:48120/a  ": "[fd7a:115c:a1e0::1]:48120",
		"swarm-viewer://pair?token=x":            "pair",
		"":                                       Redacted,
		"no scheme/path":                         Redacted,
		"http://":                                Redacted,
	}
	for in, want := range cases {
		if got := HostOnly(in); got != want {
			t.Errorf("HostOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
