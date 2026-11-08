package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// snapshot は `GET …/snapshot`。取得に失敗しても常に 200 のフラグメント（前回値のグレーアウト＋バナー OOB）。
// タブが古いときは sessions_list も並行して引き直し、タブバーを OOB で差し替える。
// session_forbidden / session_missing / session_required で、タブにそのセッションが無いと分かったときは、
// HX-Redirect で先頭の許可セッション（許可セッションが無ければ接続）へ移す。
func (h *Handler) snapshot(c *gin.Context) {
	ctx := c.Request.Context()
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	sid := c.Param("sid")

	pending := h.startTabsFetch(ctx, conn.ID)
	res := h.snaps.Get(ctx, conn.ID, sid)
	tabs, tabsOK := pending.wait()

	if res.Err != nil && sessionGone(res.Err.Kind) {
		if !tabsOK {
			// 古いキャッシュは信じず、引き直す。
			var err error
			tabs, err = h.loadTabs(ctx, conn.ID)
			tabsOK = err == nil
		}
		if target, move := redirectTarget(conn.ID, sid, tabs, tabsOK); move {
			c.Header(htmxRedirect, target)
			c.Status(http.StatusOK)
			return
		}
	}

	pollSeconds := domain.DefaultScreenPollSeconds
	if st, err := h.settings.Get(ctx); err == nil {
		pollSeconds = st.ScreenPollSeconds
	}
	p := organisms.SnapshotViewProps{
		Snapshot:     res.Snapshot,
		Fresh:        res.Fresh,
		Err:          res.Err,
		PollSeconds:  pollSeconds,
		Repair:       h.conns.NeedsRepair(conn.ID),
		ConnectionID: conn.ID,
		Session:      sid,
		Now:          h.clock.Now(),
		Kind:         conn.Kind(),
	}
	if tabsOK {
		p.Tabs = tabs
	}
	h.render(c, http.StatusOK, organisms.SnapshotView(p))
}

func sessionGone(k domain.ErrKind) bool {
	return k == domain.ErrSessionForbidden || k == domain.ErrSessionMissing || k == domain.ErrSessionRequired
}

// redirectTarget は sid が許可セッションに無いときの移動先。タブを取得できなかった・sid がタブにあるときは移動しない
// （移動しても同じ失敗を繰り返すため。バナーを出して次のポーリングで再試行する）。
func redirectTarget(cid, sid string, tabs []domain.Session, tabsOK bool) (string, bool) {
	switch {
	case !tabsOK:
		return "", false
	case len(tabs) == 0:
		return paths.Connection(cid), true
	case !hasSession(tabs, sid):
		return paths.Session(cid, tabs[0].ID), true
	}
	return "", false
}
