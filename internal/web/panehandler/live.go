package panehandler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms/panedetail"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
)

// detail は `GET …/panes/{pos}`（ペイン詳細）。ホストとは通信せず、本文は `…/live` が読み込む。見るだけの接続でも開ける。
func (h *PaneHandler) detail(c *gin.Context) {
	t, ok := h.loadTarget(c)
	if !ok {
		return
	}
	p := pages.PaneDetailProps{Ref: t.ref, Kind: t.conn.Kind()}
	h.resp.View(c, "ペイン "+t.ref.Pos,
		func(s layout.BaseProps) templ.Component { return pages.PaneDetailPage(s, p) },
		pages.PaneDetailContent(p))
}

// live は `GET …/panes/{pos}/live?lines=`（本文の部分描画。5 秒ごとの自動更新と手動更新が叩く）。見るだけの接続でも使える。
//
// report（single-flight）・pane_tail、state が asking のときだけ ask_card を呼ぶ。
// 取得に失敗したら、200 のまま、直前の表示を残して固定文言のバナーだけを差し替える（HX-Reswap: none と OOB）。
// 401・403 は自動更新の属性を外した本文を返す。最初の読み込みの失敗は、バナー付きの本文を返す。
func (h *PaneHandler) live(c *gin.Context) {
	t, ok := h.loadTarget(c)
	if !ok {
		return
	}
	first := c.Query("first") == "1"
	p, fe := h.load(c, t, linesParam(c.Query(panedetail.FieldLines)))
	if fe != nil && !first && !p.Repair {
		c.Header("HX-Reswap", "none")
		h.resp.Render(c, http.StatusOK, panedetail.BannerOOB(p.Banner, false))
		return
	}
	h.resp.Render(c, http.StatusOK, panedetail.Live(p))
}

// load は本文の入力を取得して組み立てる。失敗は Banner（固定文言）と、返り値の *domain.FetchError。
// report が取れないときは直前のスナップショットからヘッダだけを出し、操作の部品は出さない（状態が古いため）。
func (h *PaneHandler) load(c *gin.Context, t target, lines int) (panedetail.LiveProps, *domain.FetchError) {
	ctx := c.Request.Context()
	p := panedetail.LiveProps{Ref: t.ref, Kind: t.conn.Kind(), Lines: lines}
	res := h.snaps.Get(ctx, t.conn.ID, t.ref.Session)
	p.Pane, p.HasPane = findPane(res.Snapshot.Report.Panes, t.ref.Pos)
	if res.Err != nil {
		return failed(p, res.Err), res.Err
	}
	if !p.HasPane {
		return p, nil
	}
	tail, err := h.panes.Tail(ctx, t.conn.ID, t.ref.Session, t.ref.Pos, lines)
	if err != nil {
		return failed(p, asFetchError(err)), asFetchError(err)
	}
	p.Tail, p.HasTail = tail, true
	if p.Pane.State == domain.StateAsking {
		card, err := h.panes.Card(ctx, t.conn.ID, t.ref.Session, t.ref.Pos)
		if err != nil {
			return failed(p, asFetchError(err)), asFetchError(err)
		}
		p.Card, p.HasCard = card, true
	}
	return p, nil
}

// failed は取得失敗の固定文言を p に入れる。直前の本文（tail・card）は持たない。
func failed(p panedetail.LiveProps, fe *domain.FetchError) panedetail.LiveProps {
	p.Banner = format.FetchErrorMessage(fe)
	p.Repair = format.NeedsRepair(fe)
	p.Tail, p.HasTail, p.Card, p.HasCard = ops.PaneTail{}, false, ops.AskCard{}, false
	return p
}

// asFetchError は service が返した取得失敗を *domain.FetchError にする。想定外のエラーは protocol（固定文言）。
func asFetchError(err error) *domain.FetchError {
	if fe, ok := fetchError(err); ok {
		return fe
	}
	return &domain.FetchError{Kind: domain.ErrProtocol}
}
