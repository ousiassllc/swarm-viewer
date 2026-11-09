// Package handler は画面ルートの Gin ハンドラ（docs/api/endpoints.md §4、docs/components/overview.md §6.1）。
//
// service と domain にだけ依存し、repository・app は import しない。応答は Templ のコンポーネントだけで作り、
// 入力値・token・URL・swarm-console の応答本文をログにも画面にも出さない（NFR-SEC-7・10、NFR-LOG-3）。
package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/panehandler"
)

const (
	appTitle      = "swarm-viewer"
	htmxRequest   = "HX-Request"
	htmxRedirect  = "HX-Redirect"
	htmlMediaType = "text/html; charset=utf-8"
)

// Deps はハンドラが使う service 一式（コンストラクタ注入）。
type Deps struct {
	Logger      *slog.Logger
	Settings    *service.SettingsService
	Connections *service.ConnectionService
	Snapshots   *service.SnapshotService
	Pairing     *service.PairingService
	Panes       *paneservice.PaneService
	// Notifier は設定画面の OS 通知許可状態（Permitted）にだけ使う。
	Notifier service.Notifier
	Clock    service.Clock
	// CSRFToken はレイアウトが配る CSRF トークン（middleware.CSRF に渡すものと同じ）。
	CSRFToken string
}

// Handler は通常モードの画面ルート。
type Handler struct {
	base
	settings *service.SettingsService
	conns    *service.ConnectionService
	snaps    *service.SnapshotService
	pairing  *service.PairingService
	panes    *panehandler.PaneHandler
	notifier service.Notifier
	clock    service.Clock
}

// New は Handler を作る。
func New(d Deps) *Handler {
	b := base{log: d.Logger, csrf: d.CSRFToken}
	return &Handler{
		base:     b,
		settings: d.Settings,
		conns:    d.Connections,
		snaps:    d.Snapshots,
		pairing:  d.Pairing,
		panes: panehandler.New(panehandler.Deps{
			Logger: d.Logger, Resp: responder{b}, Connections: d.Connections, Snapshots: d.Snapshots, Panes: d.Panes,
		}),
		notifier: d.Notifier,
		clock:    d.Clock,
	}
}

// Register は画面ルートを登録する（静的ファイル・OpenAPI・NoRoute は router が持つ）。
func (h *Handler) Register(r gin.IRoutes) {
	r.GET("/", h.home)
	h.registerMain(r)
	h.registerConnections(r)
	h.panes.Register(r)
	h.registerPairing(r)
	h.registerSettings(r)
}

// NotFound は未定義ルートの応答（404）。
func (h *Handler) NotFound(c *gin.Context) { h.notFound(c) }

// base はレンダリングと共通のエラー応答。通常モードと recovery モードで共有する。
type base struct {
	log  *slog.Logger
	csrf string
}

func isHX(c *gin.Context) bool { return c.GetHeader(htmxRequest) == "true" }

func (b base) shell(title string) layout.BaseProps {
	return layout.BaseProps{Title: title + " - " + appTitle, CSRFToken: b.csrf}
}

// render はコンポーネントを描画してから書き出す（描画に失敗したら中途半端な本文を返さない）。
func (b base) render(c *gin.Context, status int, comp templ.Component) {
	var buf bytes.Buffer
	if err := comp.Render(c.Request.Context(), &buf); err != nil {
		b.log.Error("render failed", "route", c.FullPath(), "err", err)
		c.Data(http.StatusInternalServerError, htmlMediaType, nil)
		return
	}
	c.Data(status, htmlMediaType, buf.Bytes())
}

// view はページルートの応答。HX-Request ならフラグメント、でなければレイアウト込みのフルページ。
func (b base) view(c *gin.Context, title string, page func(layout.BaseProps) templ.Component, content templ.Component) {
	b.viewStatus(c, http.StatusOK, title, page, content)
}

func (b base) viewStatus(c *gin.Context, status int, title string, page func(layout.BaseProps) templ.Component, content templ.Component) {
	if isHX(c) {
		b.render(c, status, content)
		return
	}
	b.render(c, status, page(b.shell(title)))
}

// fail は固定文言のエラー応答。アクションルート（GET 以外）は常にフラグメント、
// ページルートは HX-Request の有無でフラグメント / フルページを分ける。
func (b base) fail(c *gin.Context, status int, message string) {
	comp := atoms.Alert(atoms.AlertProps{Kind: atoms.AlertDanger, Message: message})
	if c.Request.Method == http.MethodGet && !isHX(c) {
		comp = inLayout(b.shell(appTitle), comp)
	}
	b.render(c, status, comp)
}

func (b base) notFound(c *gin.Context) {
	b.fail(c, http.StatusNotFound, "ページが見つかりません")
}

// internal は想定外の失敗。原因はログへ（service のエラーは固定文言で、token・URL を含まない）。
func (b base) internal(c *gin.Context, what string, err error) {
	b.log.Error("handler failed", "route", c.FullPath(), "what", what, "err", err)
	b.fail(c, http.StatusInternalServerError, "内部エラーが発生しました。もう一度お試しください")
}

// inLayout は child をレイアウトの <main> に入れたフルページにする。
func inLayout(shell layout.BaseProps, child templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return layout.Base(shell).Render(templ.WithChildren(ctx, child), w)
	})
}

// fetchError は err が取得失敗（*domain.FetchError）ならそれを返す。
func fetchError(err error) (*domain.FetchError, bool) {
	var fe *domain.FetchError
	if errors.As(err, &fe) {
		return fe, true
	}
	return nil, false
}
