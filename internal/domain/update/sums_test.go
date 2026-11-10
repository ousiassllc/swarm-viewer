package update

import (
	"errors"
	"strings"
	"testing"
)

var (
	hashA = strings.Repeat("a", 64)
	hashB = strings.Repeat("b0", 32)
)

func TestParseSumsAndLookup(t *testing.T) {
	body := hashA + "  swarm-viewer_v0.1.13_darwin_universal.zip\n" +
		"\n" +
		strings.ToUpper(hashB) + " *swarm-viewer_v0.1.13_windows_amd64_installer.exe\r\n"
	s, err := ParseSums([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Lookup("swarm-viewer_v0.1.13_darwin_universal.zip"); err != nil || got != hashA {
		t.Fatalf("lookup zip = %q, %v", got, err)
	}
	if got, err := s.Lookup("swarm-viewer_v0.1.13_windows_amd64_installer.exe"); err != nil || got != hashB {
		t.Fatalf("lookup exe = %q, %v（大文字は小文字にそろえる）", got, err)
	}
}

func TestParseSumsRejectsBadFormat(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"ハッシュが短い", "abc  file\n"},
		{"16 進でない", strings.Repeat("g", 64) + "  file\n"},
		{"区切りが 1 つのスペース", hashA + " file\n"},
		{"名前が無い", hashA + "  \n"},
		{"名前の前後に空白", hashA + "   file\n"},
		{"HTML", "<html>404</html>\n"},
		{"1 行だけ壊れている（部分的に信用しない）", hashA + "  ok\nbroken line\n"},
	} {
		if _, err := ParseSums([]byte(tc.body)); !errors.Is(err, ErrBadSums) {
			t.Errorf("%s: err = %v, want ErrBadSums", tc.name, err)
		}
	}
}

func TestSumsLookupMissingAndDuplicate(t *testing.T) {
	s, err := ParseSums([]byte(hashA + "  a.zip\n" + hashA + "  dup\n" + hashB + "  dup\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "dup", "a.zi", "A.zip"} {
		if _, err := s.Lookup(name); !errors.Is(err, ErrSumMissing) {
			t.Errorf("Lookup(%q) err = %v, want ErrSumMissing", name, err)
		}
	}
	empty, err := ParseSums(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Lookup("a.zip"); !errors.Is(err, ErrSumMissing) {
		t.Errorf("空の SHA256SUMS: err = %v", err)
	}
}
