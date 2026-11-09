package panehandler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms/panedetail"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// maxFormBytes は操作のフォーム本文の上限。本文は 8000 字まで（UTF-8 で最大 32,000 バイト。URL エンコードで 3 倍弱）なので、
// 余裕を見て 256 KiB（swarm-console の要求本文の上限と同じ）。超えたら本文を読まない。
const maxFormBytes = 256 << 10

// outcome は 1 回の操作の結果。toast は ok / accepted などの判定の通知（空なら出さない）、err は操作の失敗。
type outcome struct {
	toast string
	err   error
	// clearInput は送信が受理されたとき true（入力欄を空にする）。
	clearInput bool
}

// opsForm は操作ルートの共通の前処理。操作の接続でなければ 403、本文が大きすぎれば 413。lines も読む。
func (h *PaneHandler) opsForm(c *gin.Context) (target, int, bool) {
	t, ok := h.loadOpsTarget(c)
	if !ok {
		return target{}, 0, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFormBytes)
	return t, linesParam(c.PostForm(panedetail.FieldLines)), true
}

// respond は操作のあとの応答。状態を取り直した本文に、失敗のアラート（Notice）と結果のトースト OOB を付けて返す。
// 401・403（再ペアリングが必要）は自動更新を止めた本文にし、ペインの状態が変わった失敗では最新のカードを出す（毎回取り直す）。
func (h *PaneHandler) respond(c *gin.Context, t target, lines int, o outcome) {
	p, _ := h.load(c, t, lines)
	p.ClearInput = o.clearInput
	if o.err != nil && !p.Repair {
		p.Notice, p.RetryClose = noticeOf(o.err), isExitTimeout(o.err)
	}
	comp := templ.Component(panedetail.Live(p))
	if o.toast != "" {
		comp = templ.Join(comp, atoms.Toast(atoms.ToastProps{Message: o.toast}))
	}
	h.resp.Render(c, http.StatusOK, comp)
}

// noticeOf は操作の失敗の固定文言。
func noticeOf(err error) string {
	switch {
	case errors.Is(err, paneservice.ErrBusy):
		return format.BusyMessage
	case errors.Is(err, paneservice.ErrOpsRequired):
		return format.ViewOnlyMessage
	}
	return format.OperationFailureMessage(asFetchError(err))
}

func isExitTimeout(err error) bool {
	fe, ok := fetchError(err)
	return ok && fe.Kind == domain.ErrOperation && fe.Code == "exit_timeout"
}

// connGone は操作の途中で接続が消えたとき（404）。それ以外の失敗は呼び出し側が outcome に載せる。
func (h *PaneHandler) connGone(c *gin.Context, err error) bool {
	if errors.Is(err, service.ErrConnectionNotFound) {
		h.resp.NotFound(c)
		return true
	}
	return false
}

// pick は `POST …/pick`。n（番号）か text（その他）を ask_pick で送る（確認なし。押したらすぐ）。操作の接続のみ。
func (h *PaneHandler) pick(c *gin.Context) {
	t, lines, ok := h.opsForm(c)
	if !ok {
		return
	}
	pick := ops.Pick{Text: strings.ReplaceAll(c.PostForm(panedetail.FieldText), "\r\n", "\n")}
	if raw := c.PostForm(panedetail.FieldN); raw != "" {
		pick.N, _ = strconv.Atoi(raw) // 数でなければ 0 になり、text も無ければ invalid_args で通信前に拒否される
		if pick.N == 0 {
			pick.N = -1
		}
	}
	res, err := h.panes.Pick(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, pick)
	if h.connGone(c, err) {
		return
	}
	o := outcome{err: err}
	if err == nil {
		o.toast = format.PickToast(res)
	}
	h.respond(c, t, lines, o)
}

// send は `POST …/send`。text を send_text で送る。受理されたら入力欄を空にする。操作の接続のみ。
func (h *PaneHandler) send(c *gin.Context) {
	t, lines, ok := h.opsForm(c)
	if !ok {
		return
	}
	text := strings.ReplaceAll(c.PostForm(panedetail.FieldText), "\r\n", "\n")
	res, err := h.panes.Send(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, text)
	if h.connGone(c, err) {
		return
	}
	o := outcome{err: err}
	if err == nil {
		o.toast, o.clearInput = format.SendToast(res), res.Accepted
	}
	h.respond(c, t, lines, o)
}

// escape は `POST …/escape`。作業中のエージェントを中断する（send_escape）。確認ダイアログを経たフォームだけが
// confirm=true を持つ。無ければ send_escape を呼ばない（confirm_required）。操作の接続のみ。
func (h *PaneHandler) escape(c *gin.Context) {
	t, lines, ok := h.opsForm(c)
	if !ok {
		return
	}
	confirmed := c.PostForm(panedetail.FieldConfirm) == "true"
	res, err := h.panes.Escape(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, confirmed)
	if h.connGone(c, err) {
		return
	}
	o := outcome{err: err}
	if err == nil {
		o.toast = format.EscapeToast(res)
	}
	h.respond(c, t, lines, o)
}

// closePane は `POST …/close`。エージェントを終了して片付ける（pane_close）。成功したらメイン画面へ戻る（HX-Redirect）。
// 確認ダイアログを経たフォームだけが confirm=true を持つ。無ければ pane_close を呼ばない（confirm_required）。
// exit_timeout は「もう一度試す」付きの表示。操作の接続のみ。
func (h *PaneHandler) closePane(c *gin.Context) {
	t, lines, ok := h.opsForm(c)
	if !ok {
		return
	}
	// ペインの状態は送る前にサーバー側でも確かめる（画面の古い状態からの終了を通さない）。終了できるのは idle・shell だけ。
	if err := h.requireState(c, t, format.CanClose); err != nil {
		h.respond(c, t, lines, outcome{err: err})
		return
	}
	confirmed := c.PostForm(panedetail.FieldConfirm) == "true"
	res, err := h.panes.Close(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, confirmed)
	if h.connGone(c, err) {
		return
	}
	if err == nil && res.OK {
		c.Header("HX-Redirect", paths.Session(t.conn.ID, t.ref.Session))
		c.Status(http.StatusOK)
		return
	}
	o := outcome{err: err}
	if err == nil {
		o.toast = format.CloseToast(res)
	}
	h.respond(c, t, lines, o)
}

// requireState は最新の report（single-flight）でペインの今の状態を見て、allowed でなければ通信せずに失敗を返す。
// report を取れないときはその失敗（固定文言）、ペインが無ければ pane_not_found、状態が合わなければ invalid_state（どちらも operation）。
func (h *PaneHandler) requireState(c *gin.Context, t target, allowed func(domain.PaneState) bool) error {
	res := h.snaps.Get(c.Request.Context(), t.conn.ID, t.ref.Session)
	if res.Err != nil {
		return res.Err
	}
	pane, ok := findPane(res.Snapshot.Report.Panes, t.ref.Pos)
	switch {
	case !ok:
		return ops.Error("pane_not_found")
	case !allowed(pane.State):
		return ops.Error("invalid_state")
	}
	return nil
}
