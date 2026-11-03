package secret

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// 実際の OS 秘密領域は headless な Linux では試せないため、go-keyring のメモリ実装に差し替える（テスト内のみ）。

func TestStoreSetGetDelete(t *testing.T) {
	keyring.MockInit()
	s := NewStore()

	if err := s.Set("conn-1", "test-token-not-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("conn-1")
	if err != nil || got != "test-token-not-secret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	// 保存先は service "swarm-viewer"・キー "connection:{id}"。
	if raw, err := keyring.Get("swarm-viewer", "connection:conn-1"); err != nil || raw != "test-token-not-secret" {
		t.Fatalf("keyring entry = %q, %v", raw, err)
	}

	if err := s.Set("conn-1", "test-token-replaced"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if got, _ := s.Get("conn-1"); got != "test-token-replaced" {
		t.Fatalf("overwrite not applied: %q", got)
	}

	if err := s.Delete("conn-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("conn-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	keyring.MockInit()
	s := NewStore()
	for _, id := range []string{"missing", ""} {
		if _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) = %v, want ErrNotFound", id, err)
		}
	}
}

func TestStoreDeleteIsIdempotent(t *testing.T) {
	keyring.MockInit()
	s := NewStore()
	for range 2 {
		if err := s.Delete("never-saved"); err != nil {
			t.Fatalf("Delete missing = %v, want nil", err)
		}
	}
	if err := s.Delete(""); err != nil {
		t.Fatalf("Delete empty id = %v, want nil", err)
	}
}

func TestStoreKeysAreSeparatedPerConnection(t *testing.T) {
	keyring.MockInit()
	s := NewStore()
	_ = s.Set("a", "test-token-a")
	_ = s.Set("b", "test-token-b")
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get("b"); err != nil || got != "test-token-b" {
		t.Fatalf("b = %q, %v", got, err)
	}
}

func TestStoreSetRejectsInvalidArguments(t *testing.T) {
	keyring.MockInit()
	s := NewStore()
	tooLong := strings.Repeat("x", maxTokenBytes+1)
	for name, c := range map[string][2]string{
		"empty id":    {"", "test-token-not-secret"},
		"empty token": {"conn-1", ""},
		"too long":    {"conn-1", tooLong},
	} {
		err := s.Set(c[0], c[1])
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
		if err != nil && strings.Contains(err.Error(), c[1]) && c[1] != "" {
			t.Errorf("%s: error contains token", name)
		}
	}
	// 上限ちょうどは保存できる。
	if err := s.Set("conn-1", strings.Repeat("x", maxTokenBytes)); err != nil {
		t.Fatalf("Set at limit: %v", err)
	}
}

func TestStorePropagatesBackendErrors(t *testing.T) {
	backend := errors.New("keychain locked")
	keyring.MockInitWithError(backend)
	t.Cleanup(keyring.MockInit)
	s := NewStore()

	if err := s.Set("conn-1", "test-token-not-secret"); !errors.Is(err, backend) {
		t.Errorf("Set = %v, want backend error", err)
	}
	if _, err := s.Get("conn-1"); !errors.Is(err, backend) || errors.Is(err, ErrNotFound) {
		t.Errorf("Get = %v, want backend error (not ErrNotFound)", err)
	}
	if err := s.Delete("conn-1"); !errors.Is(err, backend) {
		t.Errorf("Delete = %v, want backend error", err)
	}
}
