package panehandler_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

// 状態ごとの「できる操作」（契約 §4.2〜§4.4）。操作の接続のペイン詳細で、各部品が出る・出ないを確かめる。
func TestOpsConnection_PaneDetailShowsControlsMatchingState(t *testing.T) {
	tests := []struct {
		name, pos string
		// want・banned は /live の HTML に含まれる・含まれない文字列。
		want, banned []string
	}{
		{"working: 送信欄は無効で理由を出し、中断だけ（終了は出さない）", "T1",
			[]string{"data-send-blocked", "作業中のため送れません。終わるまで待つか、中断してください", "<fieldset class=\"flex flex-col gap-2\" disabled", "中断", `hx-confirm="作業中のエージェントを止めます"`},
			[]string{"終了", "始める", "data-ask-card"}},
		{"asking: 質問カードのボタンと「その他」、送信欄は無効、中断は出る", "T2",
			[]string{"data-ask-card", "おすすめ", "その他（自分で書く）", "質問に答えてから送れます", `disabled`, "中断", `hx-vals="{&#34;lines&#34;:&#34;30&#34;,&#34;n&#34;:&#34;1&#34;}"`},
			[]string{"終了", "この質問には、操作できる接続から答えられます"}},
		{"idle: 送信欄が使え、中断と終了が出る", "T3",
			[]string{"送る", "中断", "終了", `hx-confirm="このペインのエージェントを終了して、片付けます。よろしいですか？"`},
			[]string{"data-send-blocked", "始める"}},
		{"shell: 送信欄は無効で「始める」から起動、終了は出る（中断は出ない）", "T4",
			[]string{"エージェントが動いていません。「始める」から起動してください", "＋ 始める", "終了"},
			[]string{"中断"}},
		{"stalled: 送信欄が使え、中断が出る（終了は出ない）", "T5",
			[]string{"送る", "中断"},
			[]string{"data-send-blocked", "終了", "始める"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := opsconsole.NewStack(t, pathOps)
			html := live(t, s, tc.pos)
			mustContain(t, html, tc.want...)
			mustNotContain(t, html, tc.banned...)
			// 送信欄が使える状態（idle・stalled）だけ fieldset の disabled が無い。
			enabled := tc.pos == "T3" || tc.pos == "T5"
			if got := !strings.Contains(html, `<fieldset class="flex flex-col gap-2" disabled`); got != enabled {
				t.Errorf("送信欄が使える = %v; want %v", got, enabled)
			}
		})
	}
}

func TestOpsConnection_MainScreenShowsStartLinkOnlyOnShellPane(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := body(t, s.Get("/connections/c1/sessions/0/snapshot"), 200)
	if n := strings.Count(html, "data-pane-start"); n != 1 {
		t.Errorf("「＋ 始める」= %d 個; want 1（shell の T4 だけ）", n)
	}
	mustContain(t, html, `hx-get="`+opsconsole.PanePath("T4", "/start")+`"`, `hx-get="`+opsconsole.PanePath("T2", "")+`"`)
}

func TestOpsConnection_StartScreenIsBlockedUnlessPaneIsShell(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	for _, pos := range []string{"T1", "T3", "T9"} {
		html := body(t, s.Get(opsconsole.PanePath(pos, "/start")), 200)
		mustNotContain(t, html, "<form", "hx-post")
	}
	mustContain(t, body(t, s.Get(opsconsole.PanePath("T1", "/start")), 200), "ペインの状態が変わったため")
	mustContain(t, body(t, s.Get(opsconsole.PanePath("T9", "/start")), 200), "ペインが見つかりません")
}

func TestStartScreen_ShowsRecentFoldersFromAgentPanesWithoutDuplicates(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	body(t, s.Get("/connections/c1/sessions/0/snapshot"), 200) // report を取っておく（最近使ったフォルダの元）
	html := body(t, s.Get(opsconsole.PanePath("T4", "/start/dirs")), 200)

	// agent があるペインの cwd（/work/one は T1・T3 で重複 → 1 件）。agent の無い shell の /home/x は出さない。
	mustContain(t, html, "最近使ったフォルダ", "data-recent-folders", "/work/one", "/work/two", "/work/five")
	mustNotContain(t, html, "/home/x")
	if n := strings.Count(html, "%2Fwork%2Fone"); n != 1 {
		t.Errorf("/work/one の候補 = %d 件; want 重複なしの 1 件", n)
	}
}

func TestPaneDetail_NotInReportShowsPaneNotFoundWithoutControls(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	html := live(t, s, "T9")
	mustContain(t, html, "ペインが見つかりません。画面を更新してください")
	mustNotContain(t, html, "<form", "hx-post", "<textarea")
	if n := len(s.Console.CallsOf(pathOps, "pane_tail")); n != 0 {
		t.Errorf("存在しないペインで pane_tail を呼んだ（%d 回）", n)
	}
}
