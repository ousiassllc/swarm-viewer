package paths_test

import (
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

func TestPaths(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		paths.Connection("01A"):          "/connections/01A",
		paths.Session("01A", "dev work"): "/connections/01A/sessions/dev%20work",
		paths.Session("01A", "a/b"):      "/connections/01A/sessions/a%2Fb",
		paths.Snapshot("01A", "0"):       "/connections/01A/sessions/0/snapshot",
		paths.Draft("abc-_"):             "/pairing/drafts/abc-_",
		paths.DraftConfirm("abc"):        "/pairing/drafts/abc/confirm",
		paths.DraftDiscard("abc"):        "/pairing/drafts/abc/discard",
		paths.Connection("../etc"):       "/connections/..%2Fetc",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
