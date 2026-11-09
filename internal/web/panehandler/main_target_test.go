package panehandler_test

import (
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms/panedetail"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// ペイン詳細の本文（#pane-live）とフォームは hx-swap="outerHTML" を持つ。その中から #main へ描画するリンクが
// hx-swap を継承すると <main> ごと差し替わり、「メイン画面へ」が効かなくなる（Issue #100）。

func TestPaneDetail_ShellStartLinkDoesNotInheritOuterHTMLSwap(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := live(t, s, "T4")
	mustContain(t, html, "＋ 始める")
	webtest.MustSwapMainInner(t, html)
}

func TestPaneStart_CancelLinkDoesNotInheritOuterHTMLSwap(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	page := body(t, s.Get(opsconsole.PanePath("T4", "/start")), 200)
	mustContain(t, page, "やめる")
	webtest.MustSwapMainInner(t, page)
}

func TestPaneLive_RepairLinkDoesNotInheritOuterHTMLSwap(t *testing.T) {
	html := webtest.Render(t, panedetail.Live(panedetail.LiveProps{
		Ref:    panedetail.Ref{ConnectionID: opsconsole.ConnID, Session: "0", Pos: "T3"},
		Kind:   domain.KindOps,
		Banner: "再ペアリングが必要です",
		Repair: true,
	}))
	mustContain(t, html, "再ペアリング")
	webtest.MustSwapMainInner(t, html)
}

// 操作の接続のペイン詳細には送信欄が出て、見るだけの案内は出ない（Issue #101）。
func TestOpsConnection_PaneDetailShowsPromptFieldWithoutViewOnlyNote(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := live(t, s, "T3")
	mustContain(t, html, "<textarea", `id="pane-send-text"`, "指示を送る")
	mustNotContain(t, html, "data-view-only-prompt-note", "見るだけです")
}
