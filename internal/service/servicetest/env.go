// Package servicetest は service パッケージのテスト用フェイクと、それらを配線したテスト環境を提供する。
// テストからだけ import する（本番コードは import しない）。
package servicetest

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

// テストで使う固定値。token は明らかなダミー。
const (
	URL1       = "http://100.64.0.5:48120/report"
	URL2       = "http://100.64.0.6:48120/report"
	Token      = "test-token-not-secret"
	PairingStr = "swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=" + Token + "&name=devbox&sessions=0%2C1"

	// WaitDeadline は条件待ちの上限。固定時間の待ちには使わない。
	WaitDeadline = 5 * time.Second
)

// LogBuffer はログの取り込み先。並行書き込みに耐える。
type LogBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write は io.Writer を満たす。
func (s *LogBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *LogBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// Env は service 一式とフェイクを配線したテスト環境。
type Env struct {
	Rec      *Recorder
	Clock    *Clock
	Store    *Store
	Report   *Report
	Secrets  *Secrets
	Notifier *Notifier
	Logs     *LogBuffer

	Settings *service.SettingsService
	Conns    *service.ConnectionService
	Notif    *service.NotificationService
	Snap     *service.SnapshotService
	Pairing  *service.PairingService
}

// NewEnv はフェイクで配線した service 一式を作る。
func NewEnv(t *testing.T) *Env {
	t.Helper()
	rec := &Recorder{}
	e := &Env{
		Rec:      rec,
		Clock:    NewClock(),
		Store:    NewStore(rec),
		Report:   NewReport(rec),
		Secrets:  NewSecrets(rec),
		Notifier: &Notifier{},
		Logs:     &LogBuffer{},
	}
	log := slog.New(slog.NewTextHandler(e.Logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.Settings = service.NewSettingsService(e.Store)
	e.Conns = service.NewConnectionService(e.Report, e.Store, e.Secrets, e.Clock, log)
	e.Notif = service.NewNotificationService(e.Report, e.Store, e.Notifier, e.Clock, e.Settings, e.Conns, log)
	e.Snap = service.NewSnapshotService(e.Report, e.Store, e.Notif, e.Conns, e.Clock, log)
	e.Pairing = service.NewPairingService(e.Report, e.Store, e.Secrets, &IDs{}, e.Clock, e.Conns, log)
	return e
}

// AddConn は保存済みの接続を作る。baselined=false は baselined_at が NULL の接続。
func (e *Env) AddConn(id, name, url string, baselined bool) domain.Connection {
	c := domain.Connection{ID: id, Name: name, URL: url, CreatedAt: e.Clock.Now(), UpdatedAt: e.Clock.Now()}
	if baselined {
		c.BaselinedAt = e.Clock.Now()
	}
	e.Store.Mu.Lock()
	e.Store.Conns = append(e.Store.Conns, c)
	e.Store.Mu.Unlock()
	return c
}

// Pane はテスト用のペイン。
func Pane(id, pos string, state domain.PaneState) domain.Pane {
	return domain.Pane{ID: id, Pos: pos, State: state}
}

// ReportOf はペインだけを持つ report。
func ReportOf(panes ...domain.Pane) domain.Report { return domain.Report{Panes: panes} }

// Recv は ch から 1 件受け取る。上限時間内に来なければテストを失敗させる。
func Recv[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(WaitDeadline):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}
