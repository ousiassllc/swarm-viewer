package handler

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// registerMain は接続の選択・メイン画面・スナップショットのルートを登録する。
func (h *Handler) registerMain(r gin.IRoutes) {
	r.GET("/connections/:cid", h.connection)
	r.GET("/connections/:cid/sessions/:sid", h.session)
	r.GET("/connections/:cid/sessions/:sid/snapshot", h.snapshot)
}

// home は `GET /`。接続が無ければ /pairing、あれば最後に使った接続（無ければ先頭）へ 302。
func (h *Handler) home(c *gin.Context) {
	ctx := c.Request.Context()
	conns, err := h.conns.List(ctx)
	if err != nil {
		h.internal(c, "list connections", err)
		return
	}
	if len(conns) == 0 {
		c.Redirect(http.StatusFound, paths.Pairing)
		return
	}
	target := conns[0].ID
	if st, err := h.settings.Get(ctx); err == nil {
		for _, conn := range conns {
			if conn.ID == st.LastConnectionID {
				target = conn.ID
			}
		}
	}
	c.Redirect(http.StatusFound, paths.Connection(target))
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

// connection は `GET /connections/{cid}`。last_session（許可外・なければ先頭の許可セッション）へ 302。
func (h *Handler) connection(c *gin.Context) {
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	sid, err := h.conns.ResolveSession(c.Request.Context(), conn.ID, conn.LastSession)
	if err == nil {
		c.Redirect(http.StatusFound, paths.Session(conn.ID, sid))
		return
	}
	if errors.Is(err, service.ErrNoSessions) {
		h.noSessions(c, conn)
		return
	}
	fe, isFetch := fetchError(err)
	if !isFetch {
		h.internal(c, "resolve session", err)
		return
	}
	// 一覧を取れなくても、前回のセッションがあればそこへ（メイン画面が前回値と失敗バナーを出す）。
	if conn.LastSession != "" && !format.NeedsRepair(fe) {
		c.Redirect(http.StatusFound, paths.Session(conn.ID, conn.LastSession))
		return
	}
	p := pages.ConnectionErrorProps{ConnectionID: conn.ID, Name: conn.Name, Err: fe}
	h.view(c, "接続できません", func(s layout.BaseProps) templ.Component { return pages.ConnectionErrorPage(s, p) }, pages.ConnectionErrorContent(p))
}

func (h *Handler) noSessions(c *gin.Context, conn domain.Connection) {
	h.view(c, "許可されたセッションがありません",
		func(s layout.BaseProps) templ.Component { return pages.NoSessionsPage(s, conn.Name) },
		pages.NoSessionsContent(conn.Name))
}

// session は `GET /connections/{cid}/sessions/{sid}`（メイン画面）。
// タブを取得し、{sid} が許可外・消滅なら先頭の許可セッションへ 302。last_session と最後に使った接続を記録する。
func (h *Handler) session(c *gin.Context) {
	ctx := c.Request.Context()
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	sid := c.Param("sid")

	tabs, err := h.conns.Tabs(ctx, conn.ID)
	var tabErr *domain.FetchError
	if err != nil {
		var isFetch bool
		if tabErr, isFetch = fetchError(err); !isFetch {
			h.internal(c, "tabs", err)
			return
		}
		tabs = h.fallbackTabs(conn.ID, sid)
	} else {
		if len(tabs) == 0 {
			h.noSessions(c, conn)
			return
		}
		if !hasSession(tabs, sid) {
			c.Redirect(http.StatusFound, paths.Session(conn.ID, tabs[0].ID))
			return
		}
		if conn.LastSession != sid {
			if err := h.conns.RememberSession(ctx, conn.ID, sid); err != nil {
				h.log.Warn("remembering session failed", "connection", conn.Name, "err", err)
			}
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
		Snapshot:    h.initialSnapshot(c, conn.ID, sid, st.ScreenPollSeconds, tabErr),
	}
	h.view(c, conn.Name, func(s layout.BaseProps) templ.Component { return pages.MainPage(s, p) }, pages.MainContent(p))
}

// initialSnapshot はメイン画面の初回描画の入力。取得はせず（取得は領域の load が行う）、
// 保存済みの直前値を Fresh=false で渡してグレーアウト表示する（FR-POLL-4）。
func (h *Handler) initialSnapshot(c *gin.Context, cid, sid string, pollSeconds int, tabErr *domain.FetchError) organisms.SnapshotViewProps {
	ctx := c.Request.Context()
	sp := organisms.SnapshotViewProps{
		PollSeconds:  pollSeconds,
		ConnectionID: cid,
		Session:      sid,
		Now:          h.clock.Now(),
		Err:          tabErr,
	}
	if last, ok := h.snaps.Last(ctx, cid, sid); ok {
		sp.Snapshot = last
	}
	if h.conns.NeedsRepair(cid) {
		sp.Repair = true
		if sp.Err == nil {
			sp.Err = &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "needs_repair"}
		}
	}
	return sp
}

// fallbackTabs はタブ一覧を取得できないときの表示用。直近のタブ、無ければ選択中の 1 件。
func (h *Handler) fallbackTabs(cid, sid string) []domain.Session {
	if cached, _, ok := h.conns.CachedTabs(cid); ok && len(cached) > 0 {
		if !hasSession(cached, sid) {
			cached = append(cached, domain.Session{ID: sid, Allowed: true})
		}
		return cached
	}
	return []domain.Session{{ID: sid, Allowed: true}}
}

func hasSession(tabs []domain.Session, id string) bool {
	for _, t := range tabs {
		if t.ID == id {
			return true
		}
	}
	return false
}

// snapshot は `GET …/snapshot`。取得に失敗しても常に 200 のフラグメント（前回値のグレーアウト＋バナー OOB）。
// session_forbidden / session_missing / session_required はタブを引き直すため HX-Redirect で接続へ戻す。
func (h *Handler) snapshot(c *gin.Context) {
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	sid := c.Param("sid")
	res := h.snaps.Get(c.Request.Context(), conn.ID, sid)

	if res.Err != nil {
		switch res.Err.Kind {
		case domain.ErrSessionForbidden, domain.ErrSessionMissing, domain.ErrSessionRequired:
			c.Header(htmxRedirect, paths.Connection(conn.ID))
			c.Status(http.StatusOK)
			return
		}
	}

	pollSeconds := domain.DefaultScreenPollSeconds
	if st, err := h.settings.Get(c.Request.Context()); err == nil {
		pollSeconds = st.ScreenPollSeconds
	}
	h.render(c, http.StatusOK, organisms.SnapshotView(organisms.SnapshotViewProps{
		Snapshot:     res.Snapshot,
		Fresh:        res.Fresh,
		Err:          res.Err,
		PollSeconds:  pollSeconds,
		Repair:       h.conns.NeedsRepair(conn.ID),
		ConnectionID: conn.ID,
		Session:      sid,
		Now:          h.clock.Now(),
	}))
}
