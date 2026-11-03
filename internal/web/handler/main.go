package handler

import (
	"errors"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
)

// registerMain は接続の選択・メイン画面・スナップショットのルートを登録する。
func (h *Handler) registerMain(r gin.IRoutes) {
	r.GET("/connections/:cid", h.connection)
	r.GET("/connections/:cid/sessions/:sid", h.session)
	r.GET("/connections/:cid/sessions/:sid/snapshot", h.snapshot)
}

// home は `GET /`。接続が無ければペアリング画面、あれば最後に使った接続（無ければ先頭）のメイン画面をその場で描画する。
// WebKit の WebView は http.Redirect を扱えないため、リダイレクトしない（docs/architecture/overview.md §11）。
func (h *Handler) home(c *gin.Context) {
	ctx := c.Request.Context()
	conns, err := h.conns.List(ctx)
	if err != nil {
		h.internal(c, "list connections", err)
		return
	}
	if len(conns) == 0 {
		h.pairingView(c, organisms.PairingFormProps{})
		return
	}
	target := conns[0]
	if st, err := h.settings.Get(ctx); err == nil {
		for _, conn := range conns {
			if conn.ID == st.LastConnectionID {
				target = conn
			}
		}
	}
	h.showConnection(c, target)
}

// loadConn は {cid} の接続を読む。無ければ 404、他の失敗は 500 を返して false。
func (h *Handler) loadConn(c *gin.Context) (domain.Connection, bool) {
	conn, err := h.conns.Get(c.Request.Context(), c.Param("cid"))
	switch {
	case err == nil:
		return conn, true
	case errors.Is(err, service.ErrConnectionNotFound):
		h.notFound(c)
	default:
		h.internal(c, "load connection", err)
	}
	return domain.Connection{}, false
}

// connection は `GET /connections/{cid}`。last_session（許可外・なければ先頭の許可セッション）のメイン画面をその場で描画する。
func (h *Handler) connection(c *gin.Context) {
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	h.showConnection(c, conn)
}

// showConnection は接続の last_session でメイン画面を描画する。
func (h *Handler) showConnection(c *gin.Context, conn domain.Connection) {
	tabs, sid, ok := h.firstPaintTabs(c, conn, conn.LastSession)
	if !ok {
		return
	}
	h.renderMain(c, conn, sid, tabs)
}

func (h *Handler) noSessions(c *gin.Context, conn domain.Connection) {
	h.view(c, "許可されたセッションがありません",
		func(s layout.BaseProps) templ.Component { return pages.NoSessionsPage(s, conn.Name) },
		pages.NoSessionsContent(conn.Name))
}

// session は `GET /connections/{cid}/sessions/{sid}`（メイン画面）。
// 通信せずに描画する。タブは直近の取得結果（無ければ選択中の 1 件）で、{sid} が許可外と分かっているときは
// 先頭の許可セッションを表示する。最新のタブは snapshot 応答の OOB が反映する（API 仕様 §4.4）。
func (h *Handler) session(c *gin.Context) {
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	tabs, sid, ok := h.firstPaintTabs(c, conn, c.Param("sid"))
	if !ok {
		return
	}
	h.renderMain(c, conn, sid, tabs)
}

// renderMain はメイン画面（フルページ / フラグメント）を描画し、last_session と最後に使った接続を記録する。
func (h *Handler) renderMain(c *gin.Context, conn domain.Connection, sid string, tabs []domain.Session) {
	ctx := c.Request.Context()
	if conn.LastSession != sid {
		if err := h.conns.RememberSession(ctx, conn.ID, sid); err != nil {
			h.log.Warn("remembering session failed", "connection", conn.Name, "err", err)
		}
	}

	st, err := h.settings.Get(ctx)
	if err != nil {
		h.log.Warn("loading settings failed", "err", err)
		st = domain.DefaultSettings()
	} else if st.LastConnectionID != conn.ID {
		if err := h.conns.SetLastConnection(ctx, conn.ID); err != nil {
			h.log.Warn("remembering connection failed", "connection", conn.Name, "err", err)
		}
	}

	all, err := h.conns.List(ctx)
	if err != nil {
		h.log.Warn("listing connections failed", "err", err)
		all = []domain.Connection{conn}
	}

	p := pages.MainProps{
		Connection:  conn,
		Connections: all,
		Sessions:    tabs,
		Selected:    sid,
		Snapshot:    h.initialSnapshot(c, conn.ID, sid, st.ScreenPollSeconds),
	}
	h.view(c, conn.Name, func(s layout.BaseProps) templ.Component { return pages.MainPage(s, p) }, pages.MainContent(p))
}

// initialSnapshot はメイン画面の初回描画の入力。取得はせず（取得は領域の load が行う）、
// 保存済みの直前値を Fresh=false で渡してグレーアウト表示する（FR-POLL-4）。
func (h *Handler) initialSnapshot(c *gin.Context, cid, sid string, pollSeconds int) organisms.SnapshotViewProps {
	ctx := c.Request.Context()
	sp := organisms.SnapshotViewProps{
		PollSeconds:  pollSeconds,
		ConnectionID: cid,
		Session:      sid,
		Now:          h.clock.Now(),
	}
	if last, ok := h.snaps.Last(ctx, cid, sid); ok {
		sp.Snapshot = last
	}
	if h.conns.NeedsRepair(cid) {
		sp.Repair = true
		sp.Err = &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "needs_repair"}
	}
	return sp
}
