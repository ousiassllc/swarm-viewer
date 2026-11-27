package vectors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

func TestFetchError_NeedsRepair(t *testing.T) {
	t.Parallel()
	for kind, want := range map[domain.ErrKind]bool{
		domain.ErrUnauthorized: true, domain.ErrForbiddenScope: true,
		domain.ErrUnreachable: false, domain.ErrProtocol: false, domain.ErrSessionForbidden: false, domain.ErrOperation: false,
	} {
		if got := (&domain.FetchError{Kind: kind}).NeedsRepair(); got != want {
			t.Errorf("%v: NeedsRepair = %v, want %v", kind, got, want)
		}
	}
	var nilErr *domain.FetchError
	if nilErr.NeedsRepair() {
		t.Error("nil は false")
	}
}

func TestAsFetchError(t *testing.T) {
	t.Parallel()
	fe := &domain.FetchError{Kind: domain.ErrUnauthorized}
	if got, ok := domain.AsFetchError(fmt.Errorf("wrap: %w", fe)); !ok || got != fe {
		t.Errorf("domain.AsFetchError(wrapped) = %v, %v", got, ok)
	}
	if got, ok := domain.AsFetchError(errors.New("other")); ok || got != nil {
		t.Errorf("domain.AsFetchError(other) = %v, %v", got, ok)
	}
}
