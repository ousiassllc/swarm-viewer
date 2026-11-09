package opsconsole

import "encoding/json"

// 秘密に見える固定値。ログ・画面（固定文言以外）に出てはいけない値の検査に使う。
const (
	// TailSecret は pane_tail の本文に入れる目印（ログに出てはいけない。画面には出る）。
	TailSecret = "TAIL-BODY-MARKER"
	// QuestionSecret は ask_card の質問文に入れる目印。
	QuestionSecret = "ASK-QUESTION-MARKER"
	// ConsoleMessage はツールのエラー本文の message に入れる目印（画面にもログにも出てはいけない）。
	ConsoleMessage = "console-message-do-not-show"
)

// ReportText は既定の report。T1 working・T2 asking（omp）・T3 idle（claude）・T4 shell・T5 stalled。
const ReportText = `{"at":"2026-11-08T10:00:00Z","layout":{"rows":1,"cols":[5]},` +
	`"panes":[` +
	`{"pos":"T1","id":"%1","agent":"omp","state":"working","title":"work one","cwd":"/work/one","note":{"repo":"o/one","items":["#1"]}},` +
	`{"pos":"T2","id":"%2","agent":"omp","state":"asking","title":"ask pane","cwd":"/work/two"},` +
	`{"pos":"T3","id":"%3","agent":"claude","state":"idle","title":"idle pane","cwd":"/work/one"},` +
	`{"pos":"T4","id":"%4","agent":null,"state":"shell","title":"","cwd":"/home/x"},` +
	`{"pos":"T5","id":"%5","agent":"codex","state":"stalled","title":"stalled pane","cwd":"/work/five"}],` +
	`"attention":[{"kind":"asking","pane":"T2","detail":"どうしますか"},{"kind":"stalled","pane":"T5","detail":""}]}`

// CardText は既定の ask_card（選択肢 2 つ・その他あり）。
const CardText = `{"asking":true,"question":"` + QuestionSecret + ` どうしますか？","options":[` +
	`{"n":1,"label":"TS 仕様どおり","recommended":true},{"n":2,"label":"拡張 <b>する</b>"}],"cursor":1,"allows_custom":true}`

// SkillName・SkillDescription・SkillInvoke は skill_list の既定の応答に入れる目印（ログに出てはいけない。画面には出る）。
const (
	SkillName        = "SKILL-NAME-MARKER"
	SkillDescription = "SKILL-DESC-MARKER"
	SkillInvoke      = "/skill-invoke-marker"
)

// ToolError はツール失敗（HTTP 200 の isError）の応答。message は画面・ログに出てはいけない。
func ToolError(code string) Reply {
	b, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": ConsoleMessage, "details": map[string]any{"field": "pane"}}})
	return Reply{Text: string(b), IsError: true}
}

func defaultReplies() map[string]Reply {
	return map[string]Reply{
		"sessions_list": {Text: `{"sessions":[{"id":"0","allowed":true}]}`},
		"report":        {Text: ReportText},
		"pane_tail":     {Text: `{"pane":"T2","lines":30,"text":"line1\n<script>alert(1)</script> ` + TailSecret + `","truncated":false}`},
		"ask_card":      {Text: CardText},
		"ask_pick":      {Text: `{"ok":true,"state":"working","picked":{"n":1,"label":"TS 仕様どおり"},"card_closed":true}`},
		"send_text":     {Text: `{"ok":true,"state":"working","accepted":true,"chars":5}`},
		"send_escape":   {Text: `{"ok":true,"state":"idle","interrupted":true}`},
		"pane_start":    {Text: `{"ok":true,"state":"idle","agent":"omp","cwd":"/work/new","title":"agent: t"}`},
		"pane_close":    {Text: `{"ok":true,"state":"shell","steps":["/exit"],"note_archived":true}`},
		"skill_list": {Text: `{"agent":"omp","skills":[` +
			`{"name":"` + SkillName + `","description":"` + SkillDescription + ` <b>bold</b>","invoke":"` + SkillInvoke + `"},` +
			`{"name":"plain","description":"","invoke":"/plain"}],"truncated":false}`},
		"dir_list": {Text: `{"entries":[{"name":"work","path":"/work","git":false},{"name":"docs","path":"/docs","git":true}],"truncated":false}`},
	}
}
