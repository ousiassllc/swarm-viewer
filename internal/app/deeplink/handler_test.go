package deeplink

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

type fakeParser struct {
	mu  sync.Mutex
	got []string
	id  service.DraftID
	err error
}

func (f *fakeParser) Parse(raw string) (service.DraftID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, raw)
	return f.id, f.err
}

type fakeNav struct {
	mu    sync.Mutex
	paths []string
}

func (n *fakeNav) Open(p string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.paths = append(n.paths, p)
}

func newHandler(p *fakeParser, n *fakeNav, buf *bytes.Buffer) *Handler {
	return NewHandler(p, n, slog.New(slog.NewTextHandler(buf, nil)))
}

func TestHandle_SuccessNavigatesToDraftOnly(t *testing.T) {
	p, n, buf := &fakeParser{id: "DRAFT-ID_1"}, &fakeNav{}, &bytes.Buffer{}
	newHandler(p, n, buf).Handle(`"` + pairLink + `"`)

	if len(p.got) != 1 || p.got[0] != pairLink {
		t.Fatalf("Parse got %v, want the normalized link", p.got)
	}
	if len(n.paths) != 1 || n.paths[0] != paths.Draft("DRAFT-ID_1") {
		t.Fatalf("navigated to %v, want the draft route", n.paths)
	}
	if strings.Contains(n.paths[0], "token") || strings.Contains(n.paths[0], "100.64") {
		t.Fatalf("navigation path must not carry link content: %q", n.paths[0])
	}
}

func TestHandle_ParseErrorGoesToPasteWithoutLeaking(t *testing.T) {
	p, n, buf := &fakeParser{err: errors.New("pairing: invalid")}, &fakeNav{}, &bytes.Buffer{}
	newHandler(p, n, buf).Handle(pairLink)

	if len(n.paths) != 1 || n.paths[0] != paths.PairingInvalidLink {
		t.Fatalf("navigated to %v, want the paste page with the generic error flag", n.paths)
	}
	logged := buf.String()
	if strings.Contains(logged, "test-token-not-secret") || strings.Contains(logged, "100.64.0.1") || strings.Contains(logged, "v=1") {
		t.Fatalf("log leaks link content: %s", logged)
	}
	if !strings.Contains(logged, "host=pair") {
		t.Fatalf("log should name only the link host: %s", logged)
	}
}

func TestHandle_NonLinkIsDroppedWithoutParseOrNavigation(t *testing.T) {
	for _, raw := range []string{"", "https://example.com/?token=test-token-not-secret", "swarm-viewer://", "swarm-viewer://pair\nx"} {
		p, n, buf := &fakeParser{id: "x"}, &fakeNav{}, &bytes.Buffer{}
		newHandler(p, n, buf).Handle(raw)
		if len(p.got) != 0 || len(n.paths) != 0 {
			t.Errorf("Handle(%q): parse=%v nav=%v, want nothing", raw, p.got, n.paths)
		}
		if strings.Contains(buf.String(), "test-token-not-secret") {
			t.Errorf("Handle(%q) leaked into the log: %s", raw, buf.String())
		}
	}
}

func TestDispatcher_QueuesUntilBound(t *testing.T) {
	var d Dispatcher
	d.Submit("a")
	d.Submit("b")
	var got []string
	d.Bind(func(s string) { got = append(got, s) })
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("flushed %v, want a,b", got)
	}
	d.Submit("c")
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("after bind: %v", got)
	}
}

func TestDispatcher_PendingIsBounded(t *testing.T) {
	var d Dispatcher
	for _, s := range []string{"1", "2", "3", "4", "5", "6"} {
		d.Submit(s)
	}
	var got []string
	d.Bind(func(s string) { got = append(got, s) })
	if strings.Join(got, ",") != "3,4,5,6" {
		t.Fatalf("kept %v, want the newest %d", got, maxPending)
	}
}

func TestDispatcher_ConcurrentSubmitAndBind(t *testing.T) {
	var d Dispatcher
	var mu sync.Mutex
	count := 0
	fn := func(string) { mu.Lock(); count++; mu.Unlock() }

	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() { defer wg.Done(); d.Submit("x") }()
	}
	wg.Add(1)
	go func() { defer wg.Done(); d.Bind(fn) }()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Fatalf("delivered %d links, want exactly 3 (none lost, none duplicated)", count)
	}
}
