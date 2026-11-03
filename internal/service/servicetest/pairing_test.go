package servicetest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

func mustParse(t *testing.T, e *Env, raw string) service.DraftID {
	t.Helper()
	id, err := e.Pairing.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return id
}

func TestPairingHappyPath(t *testing.T) {
	e := NewEnv(t)
	ctx := context.Background()

	id := mustParse(t, e, PairingStr)
	view, err := e.Pairing.Draft(id)
	if err != nil {
		t.Fatal(err)
	}
	if view.Name != "devbox" || view.Host != "100.64.0.5" || len(view.SessionsHint) != 2 {
		t.Fatalf("view = %+v", view)
	}
	if !view.ExpiresAt.Equal(e.Clock.Now().Add(service.DraftTTL)) {
		t.Fatalf("ExpiresAt = %v", view.ExpiresAt)
	}

	conn, err := e.Pairing.Confirm(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if conn.ID == "" || conn.Name != "devbox" || conn.URL != URL1 {
		t.Fatalf("conn = %+v", conn)
	}
	if !conn.BaselinedAt.IsZero() {
		t.Fatal("new connection must have baselined_at = NULL")
	}
	saved, ok := e.Store.Conn(conn.ID)
	if !ok || saved != conn {
		t.Fatalf("saved = %+v ok=%v", saved, ok)
	}
	if tok, _ := e.Secrets.Token(conn.ID); tok != Token {
		t.Fatal("token was not stored in the secret store")
	}
	if len(e.Report.ProbeCalls) != 1 || e.Report.ProbeCalls[0] != (ProbeCall{URL1, Token}) {
		t.Fatalf("probe = %+v", e.Report.ProbeCalls)
	}
	// 疎通確認 → SQLite → 秘密領域の順。
	calls := e.Rec.List()
	want := []string{"report.Probe", "store.ConnectionByURL", "store.SaveConnection", "secret.Set:" + conn.ID}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	// 確定後は下書きが消える。
	if _, err := e.Pairing.Draft(id); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Draft after confirm: %v", err)
	}
	if _, err := e.Pairing.Confirm(ctx, id); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("second Confirm: %v", err)
	}
}

func TestPairingParseInvalid(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want error
	}{
		"not pairing":   {"https://example.com", domain.ErrPairingFormat},
		"empty":         {"", domain.ErrPairingFormat},
		"bad version":   {"swarm-viewer://pair?v=2&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=" + Token, domain.ErrPairingVersion},
		"outside range": {"swarm-viewer://pair?v=1&url=http%3A%2F%2F8.8.8.8%3A48120%2Freport&token=" + Token, domain.ErrInvalidURL},
		"no token":      {"swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport", domain.ErrPairingToken},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEnv(t)
			old := mustParse(t, e, PairingStr)

			_, err := e.Pairing.Parse(tc.raw)

			var ve *service.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *service.ValidationError", err)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want wraps %v", err, tc.want)
			}
			if strings.Contains(err.Error(), Token) {
				t.Fatal("error text must not contain the token")
			}
			// 失敗した Parse は既存の下書きを壊さない。
			if _, err := e.Pairing.Draft(old); err != nil {
				t.Fatalf("existing draft was dropped: %v", err)
			}
		})
	}
}

func TestPairingDraftExpiry(t *testing.T) {
	cases := map[string]struct {
		advance time.Duration
		expired bool
	}{
		"just before": {service.DraftTTL - time.Nanosecond, false},
		"at expiry":   {service.DraftTTL, true},
		"after":       {service.DraftTTL + time.Second, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEnv(t)
			id := mustParse(t, e, PairingStr)
			e.Clock.Advance(tc.advance)

			_, derr := e.Pairing.Draft(id)
			_, cerr := e.Pairing.Confirm(context.Background(), id)

			if tc.expired {
				if !errors.Is(derr, service.ErrDraftExpired) || !errors.Is(cerr, service.ErrDraftExpired) {
					t.Fatalf("Draft=%v Confirm=%v, want service.ErrDraftExpired", derr, cerr)
				}
				if e.Store.ConnCount() != 0 || len(e.Report.ProbeCalls) != 0 {
					t.Fatal("expired draft must not probe or save")
				}
			} else if derr != nil || cerr != nil {
				t.Fatalf("Draft=%v Confirm=%v", derr, cerr)
			}
		})
	}
}

func TestPairingUnknownDraft(t *testing.T) {
	e := NewEnv(t)
	if _, err := e.Pairing.Draft("nope"); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Draft: %v", err)
	}
	if _, err := e.Pairing.Confirm(context.Background(), "nope"); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Confirm: %v", err)
	}
	e.Pairing.Discard("nope") // 何も起きない
}

func TestPairingOnlyOneDraft(t *testing.T) {
	e := NewEnv(t)
	first := mustParse(t, e, PairingStr)
	second := mustParse(t, e, strings.Replace(PairingStr, "devbox", "other", 1))

	if first == second {
		t.Fatal("draft ids must differ")
	}
	if _, err := e.Pairing.Draft(first); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("old draft should be discarded: %v", err)
	}
	if v, err := e.Pairing.Draft(second); err != nil || v.Name != "other" {
		t.Fatalf("new draft: %+v %v", v, err)
	}
}

func TestPairingDiscard(t *testing.T) {
	e := NewEnv(t)
	id := mustParse(t, e, PairingStr)
	other := mustParse(t, e, PairingStr)

	e.Pairing.Discard(id) // 古い ID の破棄は現在の下書きに影響しない
	if _, err := e.Pairing.Draft(other); err != nil {
		t.Fatalf("current draft must survive: %v", err)
	}
	e.Pairing.Discard(other)
	if _, err := e.Pairing.Draft(other); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Draft after Discard: %v", err)
	}
}
