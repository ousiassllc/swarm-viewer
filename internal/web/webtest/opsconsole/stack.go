package opsconsole

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/repository/report"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/handler"
	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
)

// CSRF は Stack のハンドラが受け付ける CSRF トークン（テスト用の固定値）。
const CSRF = "csrf-token-not-secret"

// ConnID は Stack が作る接続の ID。
const ConnID = "c1"

func init() { gin.SetMode(gin.TestMode) }

// Stack は偽 console に本物のクライアント（repository/report）・service・ハンドラをつないだ一式。
// 保存先・秘密領域・時計だけがメモリのフェイク（servicetest）。ログは Logs に集める（DEBUG 以上）。
type Stack struct {
	Console *Console
	Handler http.Handler
	Logs    *servicetest.LogBuffer
	Conn    domain.Connection
	// store・secrets は AddConnection が使う（メモリのフェイク）。
	store   *servicetest.Store
	secrets *servicetest.Secrets
	// Svc は同じ service 一式（ペアリング下書きの作成や、router の組み立て＝手動確認に使う）。
	Svc Services
}

// Services は Stack が配線した service 一式。
type Services struct {
	Logger      *slog.Logger
	Settings    *service.SettingsService
	Connections *service.ConnectionService
	Snapshots   *service.SnapshotService
	Pairing     *service.PairingService
	Panes       *paneservice.PaneService
	Notifier    service.Notifier
	Clock       service.Clock
}

// NewStack は偽 console を起動し、接続先 path（"/report" か "/ops"）の接続 1 件（ID c1・名前 stub）を登録した一式を作る。
func NewStack(t *testing.T, path string) *Stack {
	t.Helper()
	return newStack(New(t), path)
}

func newStack(console *Console, path string) *Stack {
	rec := &servicetest.Recorder{}
	clock, store, secrets := servicetest.NewClock(), servicetest.NewStore(rec), servicetest.NewSecrets(rec)
	logs := &servicetest.LogBuffer{}
	log := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := report.NewClient(secrets, log)
	settings := service.NewSettingsService(store)
	conns := service.NewConnectionService(client, store, secrets, clock, log)
	notif := service.NewNotificationService(client, store, &servicetest.Notifier{}, clock, settings, conns, log)
	snaps := service.NewSnapshotService(client, store, notif, conns, clock, log)
	pairing := service.NewPairingService(client, store, secrets, &servicetest.IDs{}, clock, conns, log)
	panes := paneservice.New(client, conns, log)

	conn := domain.Connection{ID: ConnID, Name: "stub", URL: console.URL(path), CreatedAt: clock.Now(), UpdatedAt: clock.Now(), BaselinedAt: clock.Now()}
	store.Mu.Lock()
	store.Conns = append(store.Conns, conn)
	store.Mu.Unlock()
	_ = secrets.Set(ConnID, Token)

	notifier := &servicetest.Notifier{}
	h := handler.New(handler.Deps{
		Logger: log, Settings: settings, Connections: conns, Snapshots: snaps, Pairing: pairing, Panes: panes,
		Notifier: notifier, Clock: clock, CSRFToken: CSRF,
	})
	e := gin.New()
	e.Use(middleware.Recover(log), middleware.RequestLog(log), middleware.NoStore(), middleware.CSP(), middleware.CSRF(CSRF))
	h.Register(e)
	e.NoRoute(h.NotFound)
	return &Stack{
		Console: console, Handler: e, Logs: logs, Conn: conn, store: store, secrets: secrets,
		Svc: Services{
			Logger: log, Settings: settings, Connections: conns, Snapshots: snaps, Pairing: pairing, Panes: panes,
			Notifier: notifier, Clock: clock,
		},
	}
}

// AddConnection は同じ偽 console の別の種類（path が "/report" か "/ops"）の接続を追加する。
func (s *Stack) AddConnection(id, name, path string) domain.Connection {
	now := s.Svc.Clock.Now()
	c := domain.Connection{ID: id, Name: name, URL: s.Console.URL(path), CreatedAt: now, UpdatedAt: now, BaselinedAt: now}
	s.store.Mu.Lock()
	s.store.Conns = append(s.store.Conns, c)
	s.store.Mu.Unlock()
	_ = s.secrets.Set(id, Token)
	return c
}

// Do はリクエストを 1 回送る（CSRF ヘッダ付き）。hx が true なら HTMX のリクエスト（フラグメント応答）。
func (s *Stack) Do(method, target string, form url.Values, hx bool) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, target, body)
	r.Header.Set(middleware.CSRFHeaderName, CSRF)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if hx {
		r.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, r)
	return w
}

// Get は HTMX の GET。
func (s *Stack) Get(target string) *httptest.ResponseRecorder {
	return s.Do(http.MethodGet, target, nil, true)
}

// Post は HTMX の POST（form 値付き）。
func (s *Stack) Post(target string, form url.Values) *httptest.ResponseRecorder {
	if form == nil {
		form = url.Values{}
	}
	return s.Do(http.MethodPost, target, form, true)
}

// PanePath は接続 c1・セッション 0 のペイン詳細のパス（suffix は "/live" など）。
func PanePath(pos, suffix string) string {
	return "/connections/" + ConnID + "/sessions/0/panes/" + url.PathEscape(pos) + suffix
}
