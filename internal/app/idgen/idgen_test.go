package idgen

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestNewID_IsValidULID(t *testing.T) {
	id := ULID{}.NewID()
	if len(id) != 26 {
		t.Fatalf("len = %d, want 26: %q", len(id), id)
	}
	if _, err := ulid.ParseStrict(id); err != nil {
		t.Fatalf("not a valid ULID: %v", err)
	}
}

func TestNewID_UsesInjectedTimeAndEntropy(t *testing.T) {
	at := time.UnixMilli(1_700_000_000_000)
	g := ULID{Now: func() time.Time { return at }, Entropy: bytes.NewReader(make([]byte, 10))}
	id := g.NewID()
	parsed, err := ulid.ParseStrict(id)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Time() != ulid.Timestamp(at) {
		t.Fatalf("time = %d, want %d", parsed.Time(), ulid.Timestamp(at))
	}
	if !strings.HasSuffix(id, "0000000000000000") {
		t.Fatalf("zero entropy should give a zero random part: %q", id)
	}
}

func TestNewID_UniqueAndTimeOrdered(t *testing.T) {
	seen := map[string]struct{}{}
	for range 1000 {
		id := ULID{}.NewID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
	t0 := time.UnixMilli(1_700_000_000_000)
	a := ULID{Now: func() time.Time { return t0 }}.NewID()
	b := ULID{Now: func() time.Time { return t0.Add(time.Second) }}.NewID()
	if !(a < b) {
		t.Fatalf("IDs from a later time must sort after: %q vs %q", a, b)
	}
}
