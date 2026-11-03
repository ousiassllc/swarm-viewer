package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
)

// registerConnections は接続管理のルートを登録する。
func (h *Handler) registerConnections(r gin.IRoutes) {
	r.GET("/connections", h.connections)
	r.DELETE("/connections/:cid", h.deleteConnection)
}

// listProps は接続一覧の入力。CurrentID は最後に使った接続。
func (h *Handler) listProps(c *gin.Context) (organisms.ConnectionListProps, error) {
	ctx := c.Request.Context()
	conns, err := h.conns.List(ctx)
	if err != nil {
		return organisms.ConnectionListProps{}, err
	}
	p := organisms.ConnectionListProps{Connections: conns}
	if st, err := h.settings.Get(ctx); err == nil {
		p.CurrentID = st.LastConnectionID
	}
	return p, nil
}

// connections は `GET /connections`。保存済み接続の一覧（表示名・host のみ。URL 全体と token は出さない）。
func (h *Handler) connections(c *gin.Context) {
	list, err := h.listProps(c)
	if err != nil {
		h.internal(c, "list connections", err)
		return
	}
	p := pages.ConnectionsProps{List: list}
	h.view(c, "接続管理", func(s layout.BaseProps) templ.Component { return pages.ConnectionsPage(s, p) }, pages.ConnectionsContent(p))
}

// deleteConnection は `DELETE /connections/{cid}`。接続を消し（SQLite → 秘密領域）、一覧フラグメントとトースト OOB を返す。
// 最後に使った接続だった場合は settings.last_connection_id を空に戻す（次回は先頭の接続を開く）。
func (h *Handler) deleteConnection(c *gin.Context) {
	ctx := c.Request.Context()
	conn, ok := h.loadConn(c)
	if !ok {
		return
	}
	toast := atoms.ToastProps{Kind: atoms.ToastSuccess, Message: "接続「" + conn.Name + "」を削除しました"}
	if err := h.conns.Delete(ctx, conn.ID); err != nil {
		h.log.Error("deleting connection failed", "connection", conn.Name, "err", err)
		toast = atoms.ToastProps{Kind: atoms.ToastError, Message: "接続を削除できませんでした"}
	} else {
		h.clearLastConnection(c, conn)
	}
	list, err := h.listProps(c)
	if err != nil {
		h.internal(c, "list connections", err)
		return
	}
	h.render(c, http.StatusOK, templ.Join(organisms.ConnectionList(list), atoms.Toast(toast)))
}

func (h *Handler) clearLastConnection(c *gin.Context, deleted domain.Connection) {
	ctx := c.Request.Context()
	st, err := h.settings.Get(ctx)
	if err != nil || st.LastConnectionID != deleted.ID {
		return
	}
	if err := h.conns.SetLastConnection(ctx, ""); err != nil {
		h.log.Warn("clearing last connection failed", "err", err)
	}
}
