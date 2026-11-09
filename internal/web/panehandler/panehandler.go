// Package panehandler はペイン詳細と操作（選択・送信・中断・終了・新しいセッション）の画面ルート
// （docs/api/endpoints.md §4.2・§4.6、Issue #97）。linterly の 1 ディレクトリの行数上限のため handler とは別のパッケージ。
//
// service と domain にだけ依存し、repository・app は import しない。応答は Templ のコンポーネントだけで作り、
// 送信本文・pane_tail・ask_card の本文・swarm-console の応答本文をログにも固定文言の外にも出さない（NFR-SEC-10・11）。
// 操作系のルートは、接続の種類が操作でなければ swarm-console に通信せず 403 を返す。
package panehandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms/panedetail"
)

// Responder は応答の書き出し（handler パッケージの共通の応答処理を借りる）。
type Responder interface {
	// Render はコンポーネントを描画してから書き出す。
	Render(c *gin.Context, status int, comp templ.Component)
	// View はページルートの応答。HX-Request ならフラグメント、でなければレイアウト込みのフルページ。
	View(c *gin.Context, title string, page func(layout.BaseProps) templ.Component, content templ.Component)
	// Fail は固定文言のエラー応答。
	Fail(c *gin.Context, status int, message string)
	// NotFound は 404。
	NotFound(c *gin.Context)
	// Internal は想定外の失敗（原因はログへ、画面には固定文言）。
	Internal(c *gin.Context, what string, err error)
}

// Deps は PaneHandler が使う service 一式（コンストラクタ注入）。
type Deps struct {
	Logger      *slog.Logger
	Resp        Responder
	Connections *service.ConnectionService
	Snapshots   *service.SnapshotService
	Panes       *paneservice.PaneService
}

// PaneHandler はペイン詳細と操作のルート。
type PaneHandler struct {
	log   *slog.Logger
	resp  Responder
	conns *service.ConnectionService
	snaps *service.SnapshotService
	panes *paneservice.PaneService
}

// New は PaneHandler を作る。
func New(d Deps) *PaneHandler {
	return &PaneHandler{log: d.Logger, resp: d.Resp, conns: d.Connections, snaps: d.Snapshots, panes: d.Panes}
}

// Register はペイン詳細・操作のルートを登録する。P は `/connections/:cid/sessions/:sid/panes/:pos`。
func (h *PaneHandler) Register(r gin.IRoutes) {
	const p = "/connections/:cid/sessions/:sid/panes/:pos"
	r.GET(p, h.detail)
	r.GET(p+"/live", h.live)
	r.POST(p+"/pick", h.pick)
	r.POST(p+"/send", h.send)
	r.POST(p+"/escape", h.escape)
	r.POST(p+"/close", h.closePane)
	r.GET(p+"/start", h.startPage)
	r.GET(p+"/start/dirs", h.startDirs)
	r.GET(p+"/start/skills", h.startSkills)
	r.POST(p+"/start", h.startSubmit)
}

// target は 1 リクエストの対象（接続・セッション・ペイン位置）。
type target struct {
	conn domain.Connection
	ref  panedetail.Ref
}

// loadTarget は {cid}・{sid}・{pos} から対象を決める。接続が無ければ 404、他の失敗は 500 を返して false。
func (h *PaneHandler) loadTarget(c *gin.Context) (target, bool) {
	conn, err := h.conns.Get(c.Request.Context(), c.Param("cid"))
	switch {
	case err == nil:
		return target{conn: conn, ref: panedetail.Ref{ConnectionID: conn.ID, Session: c.Param("sid"), Pos: c.Param("pos")}}, true
	case errors.Is(err, service.ErrConnectionNotFound):
		h.resp.NotFound(c)
	default:
		h.resp.Internal(c, "load connection", err)
	}
	return target{}, false
}

// loadOpsTarget は loadTarget に加え、接続の種類が操作であることを確かめる。見るだけなら、通信せずに 403 と固定文言を返して false。
func (h *PaneHandler) loadOpsTarget(c *gin.Context) (target, bool) {
	t, ok := h.loadTarget(c)
	if !ok {
		return target{}, false
	}
	if t.conn.Kind() != domain.KindOps {
		h.log.Warn("operation route refused on a view-only connection", "connection_id", t.conn.ID, "route", c.FullPath())
		h.resp.Fail(c, http.StatusForbidden, format.ViewOnlyMessage)
		return target{}, false
	}
	return t, true
}

// linesParam は lines（30・100・200 のどれか。それ以外・無指定は 30）。
func linesParam(raw string) int {
	switch raw {
	case "100":
		return 100
	case "200":
		return ops.MaxTailLines
	default:
		return ops.DefaultTailLines
	}
}

// findPane は report の panes[] から位置が pos のペインを探す。
func findPane(panes []domain.Pane, pos string) (domain.Pane, bool) {
	for _, p := range panes {
		if p.Pos == pos {
			return p, true
		}
	}
	return domain.Pane{}, false
}

// fetchError は err が取得失敗（*domain.FetchError）ならそれを返す。
func fetchError(err error) (*domain.FetchError, bool) {
	var fe *domain.FetchError
	if errors.As(err, &fe) {
		return fe, true
	}
	return nil, false
}
