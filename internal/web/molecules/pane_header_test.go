package molecules_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/molecules"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

// S4 のヘッダはメモの repo と items を全件・全文で出す（NFR-DSG-19・Issue #106）。
// title 属性はタッチ・キーボードでは出ないので、属性ではなく画面に出るテキスト（DOM のテキスト）で見る。
func TestPaneHeader_NoteShowsEveryItemInFull(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("とても長い項目 very long item ", 12) + "END"
	note := &domain.Note{
		Repo:  "ousiassllc/swarm-viewer/very/long/repository/path/that/should/not/be/cut",
		Items: []string{"one", "two", "three", "four", "five", long},
	}
	html := webtest.Render(t, molecules.PaneHeader(molecules.PaneHeaderProps{
		Pane: domain.Pane{Pos: "T1", State: domain.StateIdle, Title: "t", Note: note},
		Kind: domain.KindOps,
	}))

	// repo は 1 つの p、items は 1 件 1 つの li として、全文がそのまま出る。
	visible := map[string]bool{}
	for _, tag := range []string{"p", "li"} {
		for _, el := range webtest.Elements(t, html, tag) {
			visible[strings.TrimSpace(webtest.Text(el))] = true
		}
	}
	for _, want := range append([]string{note.Repo}, note.Items...) {
		if !visible[want] {
			t.Errorf("画面のテキストに %q の全文が出ない:\n%s", want, html)
		}
	}
	// 3 件に要約しない（「+N」なし）・1 行で切らない（truncate なし）。
	webtest.MustNotContain(t, html, "truncate", "+3")
}

// メモがないときは枠ごと出さない。
func TestPaneHeader_NoNoteHidesPanel(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, molecules.PaneHeader(molecules.PaneHeaderProps{
		Pane: domain.Pane{Pos: "T1", State: domain.StateIdle, Title: "t"},
		Kind: domain.KindOps,
	}))
	webtest.MustNotContain(t, html, "bg-bone")
}

// 全文を本文に出すので、重複するツールチップ（title）は付けない。カードは要約のままツールチップに全文を載せる（NFR-DSG-19）。
func TestPaneHeader_NoteHasNoTooltipButCardKeepsIt(t *testing.T) {
	t.Parallel()
	pane := domain.Pane{Pos: "T1", State: domain.StateIdle, Title: "t", Note: &domain.Note{Repo: "r", Items: []string{"a", "b", "c", "d"}}}
	header := webtest.Render(t, molecules.PaneHeader(molecules.PaneHeaderProps{Pane: pane, Kind: domain.KindOps}))
	webtest.MustNotContain(t, header, "title=")
	card := webtest.Render(t, molecules.PaneCard(molecules.PaneCardProps{Pane: pane}))
	webtest.MustContain(t, card, "title=\"r\na\nb\nc\nd\"", "+1")
}
