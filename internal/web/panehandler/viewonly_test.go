package panehandler_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 操作の部品（フォーム・ボタン・確認ダイアログ・操作ルート）を表す文字列。見るだけの接続の HTML に 1 つも出てはいけない。
var operationMarkers = []string{
	"<form", "<textarea", "hx-post", "hx-confirm", "data-confirm", "data-pane-danger",
	"/pick", "/send", "/escape", "/close", "/start",
	"中断", "終了", "送る", "始める", "その他（自分で書く）",
}

func TestViewOnlyConnection_PaneDetailHasNoOperationControls_AndNeverCallsOps(t *testing.T) {
	s := opsconsole.NewStack(t, pathReport)

	// 状態の違うペインすべて（working・asking・idle・shell・stalled）の詳細を開く。
	for _, pos := range []string{"T1", "T2", "T3", "T4", "T5"} {
		// 詳細（フルページには、どの画面にも常にある空の確認ダイアログの枠があるので、本文のフラグメントで見る）。
		detail := body(t, s.Get(opsconsole.PanePath(pos, "")), 200)
		mustContain(t, detail, `hx-get="`+opsconsole.PanePath(pos, "/live")+`?lines=30&amp;first=1"`)
		mustNotContain(t, detail, operationMarkers...)

		html := live(t, s, pos)
		mustContain(t, html, "見るだけ", `data-pane-header`, `data-pane-tail`, "TAIL-BODY-MARKER")
		// 入力欄が無い理由と、出すための方法を 1 行で示す（Issue #101。操作の部品ではない）。
		mustContain(t, html, `data-view-only-prompt-note`, "操作の接続（/ops）で登録すると入力できます")
		mustNotContain(t, html, operationMarkers...)
		// 読み取りだけの hx-get（更新・もっと見る）のほかに、hx-* の操作を持たない。
		mustNotContain(t, html, "hx-delete", "hx-put", "hx-patch", "hx-vals", "hx-disabled-elt")
	}

	// asking のペイン: 質問文と選択肢は一覧（ボタンにしない）で、案内の 1 行だけが付く。
	html := live(t, s, "T2")
	card := html[strings.Index(html, "data-ask-card"):]
	card = card[:strings.Index(card, "</section>")]
	mustContain(t, card, "ASK-QUESTION-MARKER", "TS 仕様どおり", "&lt;b&gt;する&lt;/b&gt;", "この質問には、操作できる接続から答えられます", "data-ask-options")
	mustNotContain(t, card, "<button", "<input", "<form", "hx-")

	// pane_tail と ask_card は /report に届く（ask_card は asking のペイン T2 だけ）。/ops へのリクエストは 0 件。
	if len(s.Console.CallsOf(pathReport, "pane_tail")) < 5 {
		t.Error("pane_tail が各ペインで呼ばれていない")
	}
	cards := s.Console.CallsOf(pathReport, "ask_card")
	if len(cards) == 0 {
		t.Error("ask_card が呼ばれていない")
	}
	for _, c := range cards {
		if c.Args["pane"] != "T2" || c.Args["session"] != "0" {
			t.Errorf("ask_card の引数 = %v; want asking の T2・session 0", c.Args)
		}
	}
	if n := len(s.Console.Requests(pathOps)); n != 0 {
		t.Errorf("/ops へのリクエスト = %d 件; want 0", n)
	}
	for _, tool := range opsOnlyTools {
		if n := len(s.Console.CallsOf(pathReport, tool)); n != 0 {
			t.Errorf("見るだけの接続が %s を呼んだ（%d 回）", tool, n)
		}
	}
}

func TestViewOnlyConnection_MainScreenHasNoStartLink_ButCardsLinkToDetail(t *testing.T) {
	s := opsconsole.NewStack(t, pathReport)
	html := body(t, s.Get("/connections/c1/sessions/0/snapshot"), 200)
	mustContain(t, html, `data-pane-link`, `hx-get="`+opsconsole.PanePath("T4", "")+`"`, `data-attention-link`)
	mustNotContain(t, html, "data-pane-start", "始める")
	if n := len(s.Console.Requests(pathOps)); n != 0 {
		t.Errorf("/ops へのリクエスト = %d 件", n)
	}
}

func TestViewOnlyConnection_OperationRoutesAreForbiddenWithoutContactingConsole(t *testing.T) {
	s := opsconsole.NewStack(t, pathReport)
	const msg = "この接続では操作できません（見るだけの接続です）"

	posts := []struct {
		path string
		form map[string]string
	}{
		{"/pick", map[string]string{"n": "1"}},
		{"/pick", map[string]string{"text": "hello"}},
		{"/send", map[string]string{"text": "hello"}},
		{"/escape", map[string]string{"confirm": "true"}},
		{"/close", map[string]string{"confirm": "true"}},
		{"/start", map[string]string{"agent": "omp", "cwd": "/work", "title": "t", "confirm": "1"}},
	}
	for _, p := range posts {
		var kv []string
		for k, v := range p.form {
			kv = append(kv, k, v)
		}
		html := body(t, s.Post(opsconsole.PanePath("T3", p.path), form(kv...)), http.StatusForbidden)
		mustContain(t, html, msg)
		mustNotContain(t, html, "<form", "hx-post")
	}
	for _, path := range []string{"/start", "/start/dirs", "/start/dirs?path=%2Fwork"} {
		html := body(t, s.Get(opsconsole.PanePath("T4", path)), http.StatusForbidden)
		mustContain(t, html, msg)
	}

	if all := s.Console.AllRequests(); len(all) != 0 {
		t.Fatalf("偽 console への通信 = %d 件; want 0（%v）", len(all), all)
	}
	if !strings.Contains(s.Logs.String(), "operation route refused") {
		t.Error("拒否のログが無い")
	}
}
