package report

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// blockingServer は release を閉じるまで応答しない。cleanup の順序（release → Close）に注意。
func blockingServer(t *testing.T) (*httptest.Server, *requestLog, chan struct{}) {
	t.Helper()
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	ts, reqs := newRawServer(t, func(_ http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) }) // ts.Close より先に走る（LIFO）
	return ts, reqs, started
}

func TestTimeoutIsUnreachable(t *testing.T) {
	ts, _, started := blockingServer(t)
	c, logs := newTestClient(t)
	c.timeout = 100 * time.Millisecond

	done := make(chan error, 1)
	go func() { _, err := c.Sessions(t.Context(), testConn(ts)); done <- err }()
	<-started // サーバーが応答を保留したことを確認してから、deadline の経過を待つ
	err := waitFor(t, done)
	_ = assertFetchError(t, err, domain.ErrUnreachable)
	assertNoLeak(t, err, logs, ts)
}

func TestCallerCancellationIsUnreachable(t *testing.T) {
	ts, _, started := blockingServer(t)
	c, _ := newTestClient(t)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { _, err := c.Report(ctx, testConn(ts), "0"); done <- err }()
	<-started
	cancel()
	_ = assertFetchError(t, waitFor(t, done), domain.ErrUnreachable)
}

func TestDefaultTimeoutIsTenSeconds(t *testing.T) {
	if requestTimeout != 10*time.Second {
		t.Fatalf("requestTimeout = %v", requestTimeout)
	}
	c, _ := newTestClient(t)
	if c.timeout != 10*time.Second {
		t.Fatalf("client timeout = %v", c.timeout)
	}
}

// waitFor は結果を最大 10 秒待つ（固定 sleep ではなく、上限付きの待ち）。
func waitFor(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("call did not return")
		return nil
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	target, targetReqs := newRawServer(t, func(http.ResponseWriter, *http.Request) {})
	ts, reqs := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+domain.ReportPath, http.StatusTemporaryRedirect)
	})
	c, logs := newTestClient(t)
	_, err := c.Sessions(t.Context(), testConn(ts))
	fe := assertFetchError(t, err, domain.ErrProtocol)
	if fe.Code != "http_307" {
		t.Errorf("code = %q, want http_307 (CheckRedirect must return the 3xx as-is)", fe.Code)
	}
	if n := targetReqs.count(); n != 0 {
		t.Fatalf("redirect target received %d requests (token would leak)", n)
	}
	if reqs.count() != 1 {
		t.Errorf("origin requests = %d, want 1", reqs.count())
	}
	assertNoLeak(t, err, logs, ts)
}

func TestMissingTokenIsUnauthorizedWithoutRequest(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, logs := newTestClient(t)
	conn := testConn(ts)
	conn.ID = "no-such-connection"

	_, err := c.Sessions(t.Context(), conn)
	fe := assertFetchError(t, err, domain.ErrUnauthorized)
	if fe.Code != "token_missing" {
		t.Errorf("code = %q", fe.Code)
	}
	if reqs.count() != 0 {
		t.Errorf("requests = %d, want 0 (no Authorization-less request)", reqs.count())
	}
	assertNoLeak(t, err, logs, ts)
	if !strings.Contains(logs.String(), "connection_id="+conn.ID) {
		t.Errorf("log lacks connection id: %s", logs)
	}
}

func TestSecretStoreFailureIsUnreachable(t *testing.T) {
	ts, reqs := newStatelessServer(t, stubOptions{jsonResponse: true, tools: okTools()})
	c, logs := newTestClient(t)
	backend := errors.New("keychain locked " + testToken)
	keyring.MockInitWithError(backend)
	t.Cleanup(keyring.MockInit)

	_, err := c.Report(t.Context(), testConn(ts), "0")
	fe := assertFetchError(t, err, domain.ErrUnreachable)
	if fe.Code != "secret_store" {
		t.Errorf("code = %q", fe.Code)
	}
	if reqs.count() != 0 {
		t.Errorf("requests = %d", reqs.count())
	}
	assertNoLeak(t, err, logs, ts)
}
