package notice

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

func TestBuild_Asking(t *testing.T) {
	m := Build("N1", service.Notification{
		ConnectionID: "c1", ConnectionName: "devbox", Session: "swarm", Pos: "1.2",
		State: domain.StateAsking, Detail: "続行しますか?",
	})
	if m.ID != "N1" || m.Title != "devbox" {
		t.Fatalf("id/title = %q/%q", m.ID, m.Title)
	}
	if m.Body != "swarm · 1.2 · 答え待ち\n続行しますか?" {
		t.Fatalf("body = %q", m.Body)
	}
	if m.Data[DataConnectionID] != "c1" || m.Data[DataSession] != "swarm" {
		t.Fatalf("data = %v", m.Data)
	}
}

func TestBuild_StalledHasNoDetailLine(t *testing.T) {
	m := Build("N2", service.Notification{ConnectionID: "c1", ConnectionName: "devbox", Session: "s", Pos: "0.0", State: domain.StateStalled})
	if m.Body != "s · 0.0 · 止まってるかも" {
		t.Fatalf("body = %q", m.Body)
	}
}

func TestBuild_NeverCarriesTokenOrURL(t *testing.T) {
	m := Build("N3", service.Notification{ConnectionID: "c1", ConnectionName: "devbox", Session: "s", Pos: "0.0", State: domain.StateAsking})
	for k, v := range m.Data {
		if s, _ := v.(string); strings.Contains(s, "://") || strings.Contains(strings.ToLower(k), "token") {
			t.Fatalf("data %s=%v must not carry URL/token", k, v)
		}
	}
}

func TestBuild_SanitizesAndTruncatesHostSuppliedText(t *testing.T) {
	long := strings.Repeat("あ", 500)
	m := Build("N4", service.Notification{
		ConnectionID: "c1", ConnectionName: "dev\nbox\x07", Session: "s", Pos: "0.0",
		State: domain.StateAsking, Detail: long,
	})
	if m.Title != "dev box" {
		t.Fatalf("title = %q, want control chars folded", m.Title)
	}
	detail := strings.SplitN(m.Body, "\n", 2)[1]
	if got := len([]rune(detail)); got != maxDetailRunes+1 { // 切り詰め + 「…」
		t.Fatalf("detail runes = %d", got)
	}
}

func TestBuild_EmptyNameFallsBack(t *testing.T) {
	if m := Build("N5", service.Notification{State: domain.StateAsking}); m.Title != "swarm-viewer" {
		t.Fatalf("title = %q", m.Title)
	}
}

func TestTarget(t *testing.T) {
	path, ok := Target(map[string]any{DataConnectionID: "c1", DataSession: "my session"})
	if !ok || path != "/connections/c1/sessions/my%20session" {
		t.Fatalf("Target = (%q, %v)", path, ok)
	}
	// 通知 → クリックの往復で Build の Data から同じ遷移先に戻る。
	m := Build("N6", service.Notification{ConnectionID: "c9", ConnectionName: "n", Session: "sess", Pos: "1", State: domain.StateAsking})
	if p, ok := Target(m.Data); !ok || p != "/connections/c9/sessions/sess" {
		t.Fatalf("round trip = (%q, %v)", p, ok)
	}
}

func TestTarget_RejectsBadPayloads(t *testing.T) {
	for _, d := range []map[string]any{
		nil,
		{},
		{DataConnectionID: "c1"},
		{DataSession: "s"},
		{DataConnectionID: "", DataSession: "s"},
		{DataConnectionID: "c1", DataSession: ""},
		{DataConnectionID: 1, DataSession: "s"},
		{DataConnectionID: "c1", DataSession: []string{"s"}},
	} {
		if p, ok := Target(d); ok {
			t.Errorf("Target(%v) = %q, want rejection", d, p)
		}
	}
}
