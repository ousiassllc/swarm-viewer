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

func TestPairingProbeFailureSavesNothingAndKeepsDraft(t *testing.T) {
	e := NewEnv(t)
	e.Report.ProbeErr = &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "unauthorized"}
	id := mustParse(t, e, PairingStr)
	ctx := context.Background()
	e.Clock.Advance(time.Minute)

	_, err := e.Pairing.Confirm(ctx, id)

	var fe *domain.FetchError
	if !errors.As(err, &fe) || fe.Kind != domain.ErrUnauthorized {
		t.Fatalf("err = %v, want unauthorized FetchError", err)
	}
	if e.Store.ConnCount() != 0 {
		t.Fatal("connection was saved despite probe failure")
	}
	if _, ok := e.Secrets.Token("id-1"); ok {
		t.Fatal("token was saved despite probe failure")
	}
	for _, c := range e.Rec.List() {
		if strings.HasPrefix(c, "store.Save") || strings.HasPrefix(c, "secret.") {
			t.Fatalf("unexpected write call %q", c)
		}
	}
	// 下書きは残り、再試行できる。期限は延長されない。
	if _, err := e.Pairing.Draft(id); err != nil {
		t.Fatalf("draft should remain: %v", err)
	}
	e.Report.ProbeErr = nil
	if _, err := e.Pairing.Confirm(ctx, id); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestPairingProbeFailureDoesNotExtendExpiry(t *testing.T) {
	e := NewEnv(t)
	e.Report.ProbeErr = ErrInjected
	id := mustParse(t, e, PairingStr)
	e.Clock.Advance(service.DraftTTL - time.Second)
	if _, err := e.Pairing.Confirm(context.Background(), id); err == nil {
		t.Fatal("want probe failure")
	}
	e.Clock.Advance(2 * time.Second)
	if _, err := e.Pairing.Draft(id); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Draft = %v, want expired", err)
	}
}

func TestPairingProbeFailureIsClassifiedUnreachable(t *testing.T) {
	e := NewEnv(t)
	e.Report.ProbeErr = ErrInjected
	id := mustParse(t, e, PairingStr)
	_, err := e.Pairing.Confirm(context.Background(), id)
	var fe *domain.FetchError
	if !errors.As(err, &fe) || fe.Kind != domain.ErrUnreachable {
		t.Fatalf("err = %v", err)
	}
}

func TestPairingSecretFailureRollsBackNewRow(t *testing.T) {
	e := NewEnv(t)
	e.Secrets.ErrSet = ErrInjected
	id := mustParse(t, e, PairingStr)

	_, err := e.Pairing.Confirm(context.Background(), id)

	if err == nil {
		t.Fatal("want error")
	}
	if e.Store.ConnCount() != 0 {
		t.Fatal("newly created row must be deleted when the secret store fails")
	}
	if strings.Contains(err.Error(), Token) || strings.Contains(e.Logs.String(), Token) {
		t.Fatal("token leaked into error or log")
	}
	if _, err := e.Pairing.Draft(id); err != nil {
		t.Fatalf("draft should remain for retry: %v", err)
	}
}

func TestPairingSecretFailureKeepsExistingRow(t *testing.T) {
	e := NewEnv(t)
	e.AddConn("c1", "old", URL1, true)
	e.Secrets.ErrSet = ErrInjected
	id := mustParse(t, e, PairingStr)

	if _, err := e.Pairing.Confirm(context.Background(), id); err == nil {
		t.Fatal("want error")
	}
	if _, ok := e.Store.Conn("c1"); !ok {
		t.Fatal("existing connection must not be deleted by a failed re-pairing")
	}
}

func TestPairingRepairingKeepsIDAndBaselinedAt(t *testing.T) {
	e := NewEnv(t)
	old := e.AddConn("c1", "old-name", URL1, true)
	e.Secrets.Tokens["c1"] = "old-token-not-secret"
	e.Conns.MarkNeedsRepair("c1")
	e.Clock.Advance(time.Hour)
	id := mustParse(t, e, PairingStr)

	conn, err := e.Pairing.Confirm(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if conn.ID != "c1" || e.Store.ConnCount() != 1 {
		t.Fatalf("re-pairing must replace, not add: conn=%+v count=%d", conn, e.Store.ConnCount())
	}
	if !conn.BaselinedAt.Equal(old.BaselinedAt) {
		t.Fatalf("baselined_at changed: %v -> %v", old.BaselinedAt, conn.BaselinedAt)
	}
	if !conn.CreatedAt.Equal(old.CreatedAt) || !conn.UpdatedAt.Equal(e.Clock.Now()) {
		t.Fatalf("created/updated = %v/%v", conn.CreatedAt, conn.UpdatedAt)
	}
	if tok, _ := e.Secrets.Token("c1"); tok != Token {
		t.Fatal("token was not replaced")
	}
	if e.Conns.NeedsRepair("c1") {
		t.Fatal("re-pairing must clear needs_repair")
	}
}

func TestPairingConcurrentConfirmCreatesOneConnection(t *testing.T) {
	e := NewEnv(t)
	id := mustParse(t, e, PairingStr)
	done := make(chan error, 4)
	for range 4 {
		go func() { _, err := e.Pairing.Confirm(context.Background(), id); done <- err }()
	}
	ok := 0
	for range 4 {
		if err := Recv(t, done, "confirm"); err == nil {
			ok++
		} else if !errors.Is(err, service.ErrDraftExpired) {
			t.Fatalf("err = %v", err)
		}
	}
	if ok != 1 || e.Store.ConnCount() != 1 {
		t.Fatalf("ok=%d connections=%d, want exactly one", ok, e.Store.ConnCount())
	}
}
