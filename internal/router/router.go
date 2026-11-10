// Package router は Gin のエンジンを組み立てる（ミドルウェア・画面ルート・静的ファイル・OpenAPI）。
//
// 通常モード（New）と、DB が開けないときの recovery モード（NewRecovery）の 2 つを返す。
// ミドルウェアの順序は Recover → RequestLog → NoStore → CSP → CSRF（docs/components/overview.md §6.2）。
package router

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/web/handler"
	"github.com/ousiassllc/swarm-viewer/internal/web/middleware"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/updatehandler"
)

// OpenAPIPath は OpenAPI スペックの公開パス（`.json` を付けて公開する）。JSON オペレーションは v1 では 0 本
// （docs/architecture/overview.md §4.3）。
const OpenAPIPath = "/api/v1/openapi"

// Deps は通常モードのルーターの入力。
type Deps struct {
	Logger      *slog.Logger
	Settings    *service.SettingsService
	Connections *service.ConnectionService
	Snapshots   *service.SnapshotService
	Pairing     *service.PairingService
	Panes       *paneservice.PaneService
	Notifier    service.Notifier // 設定画面の通知許可状態（Permitted）にだけ使う
	Clock       service.Clock
	Updates     updatehandler.Service // 新しい版の確認と更新（Issue #109）。nil なら通知を出さない
	Static      fs.FS                 // static.FS()
}

// RecoveryReason は初期化導線を出す理由。
type RecoveryReason = pages.RecoveryReason

// RecoveryReason の値。
const (
	RecoveryCorrupt      = pages.RecoveryCorrupt
	RecoverySchemaTooNew = pages.RecoverySchemaTooNew
)

// RecoveryDeps は recovery モードのルーターの入力。
type RecoveryDeps struct {
	Logger *slog.Logger
	Reason RecoveryReason
	// Reset は保存データを退避して作り直す。POST /recovery/reset が呼ぶ。成功したらアプリが通常モードへ切り替える。
	Reset  func(ctx context.Context) error
	Static fs.FS
}

// New は通常モードのハンドラを返す。CSRF トークンはここで生成する（プロセスごと）。
func New(d Deps) (http.Handler, error) {
	token, err := middleware.NewCSRFToken()
	if err != nil {
		return nil, err
	}
	h := handler.New(handler.Deps{
		Logger:      d.Logger,
		Settings:    d.Settings,
		Connections: d.Connections,
		Snapshots:   d.Snapshots,
		Pairing:     d.Pairing,
		Panes:       d.Panes,
		Notifier:    d.Notifier,
		Clock:       d.Clock,
		Updates:     d.Updates,
		CSRFToken:   token,
	})
	e := newEngine(d.Logger, token)
	h.Register(e)
	e.NoRoute(h.NotFound)
	if err := registerStatic(e, d.Static); err != nil {
		return nil, err
	}
	registerOpenAPI(e)
	return e, nil
}

// NewRecovery は recovery モードのハンドラを返す（`/` の初期化導線と `POST /recovery/reset` だけ）。
func NewRecovery(d RecoveryDeps) (http.Handler, error) {
	if d.Reset == nil {
		return nil, errors.New("router: recovery reset function is required")
	}
	token, err := middleware.NewCSRFToken()
	if err != nil {
		return nil, err
	}
	h := handler.NewRecovery(handler.RecoveryDeps{Logger: d.Logger, Reason: d.Reason, Reset: d.Reset, CSRFToken: token})
	e := newEngine(d.Logger, token)
	h.Register(e)
	e.NoRoute(h.NotFound)
	if err := registerStatic(e, d.Static); err != nil {
		return nil, err
	}
	return e, nil
}

var releaseOnce sync.Once

// newEngine は共通のミドルウェアを載せたエンジンを返す。
func newEngine(logger *slog.Logger, csrfToken string) *gin.Engine {
	releaseOnce.Do(func() {
		// 既定の debug モードはルート登録を標準出力へ出す。GIN_MODE の指定があればそれを尊重する。
		if gin.Mode() == gin.DebugMode {
			gin.SetMode(gin.ReleaseMode)
		}
	})
	e := gin.New()
	// セッション名に `/` を含められるよう、エスケープ済みのパス（%2F）でルーティングし、パラメータは復号する。
	e.UseRawPath = true
	e.UnescapePathValues = true
	e.Use(
		middleware.Recover(logger),
		middleware.RequestLog(logger),
		middleware.NoStore(),
		middleware.CSP(),
		middleware.CSRF(csrfToken),
	)
	return e
}

// registerOpenAPI は Huma を Gin に載せ、`/api/v1/openapi.json` を公開する。
// 登録するオペレーションは 0 本。ドキュメント UI（`/docs`、外部アセットを読む）とスキーマ配信は無効にする。
func registerOpenAPI(e *gin.Engine) {
	cfg := huma.DefaultConfig("swarm-viewer", "1.0.0")
	cfg.OpenAPIPath = OpenAPIPath
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	humagin.New(e, cfg)
}
