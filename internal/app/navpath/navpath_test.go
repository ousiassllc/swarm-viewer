package navpath

import "testing"

func TestClean(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"/", "/", true},
		{"/pairing/drafts/abc", "/pairing/drafts/abc", true},
		{"/connections/c1/sessions/my%20session", "/connections/c1/sessions/my%20session", true},
		{"/pairing?link=invalid", "/pairing?link=invalid", true},
		{"", "/", false},
		{"pairing", "/", false},
		{"//evil.example/x", "/", false},
		{"https://evil.example/", "/", false},
		{"javascript:alert(1)", "/", false},
		{"/\\evil.example", "/", false},
		{"/a\nb", "/", false},
		{"/a#frag", "/", false},
		{"/%zz", "/", false},
	}
	for _, c := range cases {
		got, ok := Clean(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Clean(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
