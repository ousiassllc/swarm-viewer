package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

const resetFailedMessage = "保存データを初期化できませんでした。アプリを再起動して、もう一度お試しください"

// RecoveryDeps は recovery モードのハンドラの入力。store・service を持たない。
type RecoveryDeps struct {
	Logger *slog.Logger
	Reason pages.RecoveryReason
	// Reset は保存データを退避して作り直す。成功したらアプリが通常モードへ切り替える。
	Reset     func(ctx context.Context) error
	CSRFToken string
}

// Recovery は DB が開けないときの最小ルート（`/` の初期化導線と `POST /recovery/reset`）。
type Recovery struct {
	base
	reason pages.RecoveryReason
	reset  func(ctx context.Context) error
}

// NewRecovery は Recovery を作る。
func NewRecovery(d RecoveryDeps) *Recovery {
	return &Recovery{base: base{log: d.Logger, csrf: d.CSRFToken}, reason: d.Reason, reset: d.Reset}
}

// Register は recovery モードのルートを登録する。
func (r *Recovery) Register(g gin.IRoutes) {
	g.GET("/", r.page)
	g.POST(paths.RecoveryReset, r.resetData)
}

// NotFound は recovery モードの未定義ルート。ページ遷移は `/` の初期化導線へ戻し、それ以外は 404。
func (r *Recovery) NotFound(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, paths.Home)
		return
	}
	r.notFound(c)
}

func (r *Recovery) page(c *gin.Context) {
	p := pages.RecoveryProps{Reason: r.reason}
	r.view(c, "保存データを開けません", func(s layout.BaseProps) templ.Component { return pages.RecoveryPage(s, p) }, pages.RecoveryContent(p))
}

// resetData は `POST /recovery/reset`。成功したら通常モードの `/` へ（HX-Redirect でページごと読み直す）。
// 失敗は固定文言を添えた初期化導線のフラグメント。
func (r *Recovery) resetData(c *gin.Context) {
	if err := r.reset(c.Request.Context()); err != nil {
		r.log.Error("recovery reset failed", "err", err)
		r.render(c, http.StatusOK, pages.RecoveryContent(pages.RecoveryProps{Reason: r.reason, Error: resetFailedMessage}))
		return
	}
	if isHX(c) {
		c.Header(htmxRedirect, paths.Home)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, paths.Home)
}
