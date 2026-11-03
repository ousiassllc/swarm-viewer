package deeplink

import "testing"

const pairLink = "swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.1%3A48120%2Freport&token=test-token-not-secret"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain", pairLink, pairLink, true},
		{"double quoted", `"` + pairLink + `"`, pairLink, true},
		{"single quoted", `'` + pairLink + `'`, pairLink, true},
		{"smart quoted", "“" + pairLink + "”", pairLink, true},
		{"surrounding spaces", "  " + pairLink + "\t", pairLink, true},
		{"spaces inside quotes", `" ` + pairLink + ` "`, pairLink, true},
		{"upper-case scheme", "SWARM-VIEWER://pair?v=1", "swarm-viewer://pair?v=1", true},
		{"mixed-case scheme", "Swarm-Viewer://pair?v=1", "swarm-viewer://pair?v=1", true},
		{"trailing slash host kept for parser", "swarm-viewer://pair/?v=1", "swarm-viewer://pair/?v=1", true},
		{"empty", "", "", false},
		{"scheme only", "swarm-viewer://", "", false},
		{"other scheme", "http://100.64.0.1:48120/report", "", false},
		{"file scheme", "file:///etc/passwd", "", false},
		{"javascript", "javascript:alert(1)", "", false},
		{"prefix lookalike", "xswarm-viewer://pair", "", false},
		{"no slashes", "swarm-viewer:pair?v=1", "", false},
		{"executable path", `C:\Program Files\swarm-viewer\swarm-viewer.exe`, "", false},
		{"flag", "--swarm-viewer://pair", "", false},
		{"newline injected", "swarm-viewer://pair?v=1\nx=1", "", false},
		{"NUL injected", "swarm-viewer://pair?v=1\x00", "", false},
		{"only quotes", `""`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Normalize(c.in)
			if ok != c.ok || got != c.want {
				t.Fatalf("Normalize(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{"windows cold start", []string{`C:\app\swarm-viewer.exe`, pairLink}, pairLink, true},
		{"second instance with working dir flags", []string{"swarm-viewer", "--x", `"` + pairLink + `"`}, pairLink, true},
		{"first link wins", []string{"a", "swarm-viewer://pair?v=1&a=1", "swarm-viewer://pair?v=1&a=2"}, "swarm-viewer://pair?v=1&a=1", true},
		{"skips other schemes", []string{"a", "https://example.com", pairLink}, pairLink, true},
		{"no args", nil, "", false},
		{"only exe", []string{"swarm-viewer"}, "", false},
		{"only other scheme", []string{"swarm-viewer", "https://example.com"}, "", false},
		{"malformed", []string{"swarm-viewer", "swarm-viewer://"}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Extract(c.args)
			if ok != c.ok || got != c.want {
				t.Fatalf("Extract = (%q, %v), want (%q, %v)", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestHostOfNeverReturnsTokenOrQuery(t *testing.T) {
	cases := map[string]string{
		pairLink:                          "pair",
		"swarm-viewer://pair/?token=abc":  "pair",
		"SWARM-VIEWER://pair#frag":        "pair",
		"swarm-viewer://":                 "",
		"swarm-viewer://evil?token=zzzzz": "evil",
		"pair?token=abc":                  "pair",
	}
	for in, want := range cases {
		if got := HostOf(in); got != want {
			t.Errorf("HostOf(%q) = %q, want %q", in, got, want)
		}
	}
}
