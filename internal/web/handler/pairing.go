package handler

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// maxPairingFormBytes は貼り付けフォームの本文上限。接続文字列は 4,096 バイト以内（API 仕様 §2.1）なので十分余裕を見る。
const maxPairingFormBytes = 16 << 10

// invalidLinkMessage は受け取ったディープリンクを解析できなかったときの固定文言（受け取った文字列は出さない）。
const invalidLinkMessage = "リンクを開けませんでした。接続文字列を貼り付けてください"

func (h *Handler) registerPairing(r gin.IRoutes) {
	r.GET("/pairing", h.pairingPage)
	r.POST("/pairing", h.pairingPaste)
	r.GET("/pairing/drafts/:did", h.draftPage)
	r.POST("/pairing/drafts/:did/confirm", h.draftConfirm)
	r.POST("/pairing/drafts/:did/discard", h.draftDiscard)
}

func (h *Handler) pairingProps(c *gin.Context, form organisms.PairingFormProps) pages.PairingProps {
	conns, err := h.conns.List(c.Request.Context())
	if err != nil {
		h.log.Warn("listing connections failed", "err", err)
	}
	return pages.PairingProps{Form: form, HasConnections: len(conns) > 0}
}

// pairingPage は `GET /pairing`。`?link=invalid` はディープリンクの解析失敗を知らせる（クエリの値は出さない）。
func (h *Handler) pairingPage(c *gin.Context) {
	var form organisms.PairingFormProps
	if c.Query(paths.PairingLinkParam) == paths.PairingLinkInvalid {
		form.Error = invalidLinkMessage
	}
	p := h.pairingProps(c, form)
	h.view(c, "接続を追加", func(s layout.BaseProps) templ.Component { return pages.PairingPage(s, p) }, pages.PairingContent(p))
}

// pairingPaste は `POST /pairing`。接続文字列を 1 回だけ読み、解析して下書きを作る。
// 失敗は 422 でフォームだけを返す。貼り付けた文字列・token は応答にもログにも出さない（NFR-SEC-7）。
func (h *Handler) pairingPaste(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPairingFormBytes)
	raw := c.PostForm(organisms.FieldPairing)
	// 解析後に値を持ち回さない。フォーム値のコピーを消す。
	c.Request.PostForm, c.Request.Form, c.Request.MultipartForm = nil, nil, nil

	id, err := h.pairing.Parse(raw)
	if err != nil {
		var ve *service.ValidationError
		if !errors.As(err, &ve) {
			h.internal(c, "parse pairing", errors.New("unexpected parse failure"))
			return
		}
		h.render(c, http.StatusUnprocessableEntity, organisms.PairingForm(organisms.PairingFormProps{Error: format.PairingErrorMessage(err)}))
		return
	}
	c.Redirect(http.StatusSeeOther, paths.Draft(string(id)))
}

// draftPage は `GET /pairing/drafts/{did}`。期限切れ・未知の ID は 404 でなく「もう一度ペアリングしてください」。
func (h *Handler) draftPage(c *gin.Context) {
	p, ok := h.draftProps(c, "")
	if !ok {
		return
	}
	h.view(c, "接続の確認", func(s layout.BaseProps) templ.Component { return pages.PairingDraftPage(s, p) }, pages.PairingDraftContent(p))
}

// draftProps は確認画面の入力。下書きが無ければ「もう一度ペアリングしてください」を返して false。
func (h *Handler) draftProps(c *gin.Context, errMsg string) (pages.DraftProps, bool) {
	did := c.Param("did")
	v, err := h.pairing.Draft(service.DraftID(did))
	if err != nil {
		if errors.Is(err, service.ErrDraftExpired) {
			h.draftExpired(c)
		} else {
			h.internal(c, "draft", err)
		}
		return pages.DraftProps{}, false
	}
	return pages.DraftProps{
		DraftID:      did,
		Name:         v.Name,
		Host:         v.Host,
		SessionsHint: v.SessionsHint,
		ExpiresText:  format.Until(v.ExpiresAt, h.clock.Now()),
		Error:        errMsg,
	}, true
}

func (h *Handler) draftExpired(c *gin.Context) {
	h.view(c, "もう一度ペアリングしてください",
		func(s layout.BaseProps) templ.Component { return pages.DraftExpiredPage(s) },
		pages.DraftExpiredContent())
}

// draftConfirm は `POST …/confirm`。疎通確認に成功したときだけ保存して 303 でメイン画面へ。
// 失敗は保存せず、下書きを残したまま確認画面のフラグメントに固定文言を出す（再試行できる）。
func (h *Handler) draftConfirm(c *gin.Context) {
	did := c.Param("did")
	conn, err := h.pairing.Confirm(c.Request.Context(), service.DraftID(did))
	if err == nil {
		c.Redirect(http.StatusSeeOther, paths.Connection(conn.ID))
		return
	}
	if errors.Is(err, service.ErrDraftExpired) {
		h.render(c, http.StatusOK, pages.DraftExpiredContent())
		return
	}
	msg := "接続を保存できませんでした。もう一度お試しください"
	if fe, ok := fetchError(err); ok {
		msg = format.FetchErrorMessage(fe)
	} else {
		h.log.Error("confirming pairing failed", "err", err)
	}
	p, ok := h.draftProps(c, msg)
	if !ok {
		return
	}
	h.render(c, http.StatusOK, pages.PairingDraftContent(p))
}

// draftDiscard は `POST …/discard`。下書きを即時に破棄し、303 で /（接続があるとき）か /pairing へ。
func (h *Handler) draftDiscard(c *gin.Context) {
	h.pairing.Discard(service.DraftID(c.Param("did")))
	target := paths.Pairing
	if conns, err := h.conns.List(c.Request.Context()); err == nil && len(conns) > 0 {
		target = paths.Home
	}
	c.Redirect(http.StatusSeeOther, target)
}
