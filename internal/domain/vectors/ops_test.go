package vectors_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

// 操作モードのテストベクタ（toolresult.json・opsinput.json）。Android 版（Kotlin）と共有する。形式は testdata/vectors/README.md。

const (
	minToolResultCases = 55
	minOpsInputCases   = 28
)

// 解析後の値（ベクタの want）。ask_pick の picked は平らにする。
type (
	wantTail struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	}
	wantOption struct {
		N           int    `json:"n"`
		Label       string `json:"label"`
		Recommended bool   `json:"recommended"`
	}
	wantCard struct {
		Asking       bool         `json:"asking"`
		Question     string       `json:"question"`
		Options      []wantOption `json:"options"`
		Cursor       int          `json:"cursor"`
		AllowsCustom bool         `json:"allows_custom"`
	}
	wantPick struct {
		OK          bool   `json:"ok"`
		State       string `json:"state"`
		PickedN     int    `json:"picked_n"`
		PickedLabel string `json:"picked_label"`
		CardClosed  bool   `json:"card_closed"`
	}
	wantSend struct {
		OK       bool   `json:"ok"`
		State    string `json:"state"`
		Accepted bool   `json:"accepted"`
		Chars    int    `json:"chars"`
	}
	wantEscape struct {
		OK          bool   `json:"ok"`
		State       string `json:"state"`
		Interrupted bool   `json:"interrupted"`
	}
	wantStart struct {
		OK    bool   `json:"ok"`
		State string `json:"state"`
		Agent string `json:"agent"`
		Cwd   string `json:"cwd"`
		Title string `json:"title"`
	}
	wantClose struct {
		OK           bool   `json:"ok"`
		State        string `json:"state"`
		NoteArchived bool   `json:"note_archived"`
	}
	wantEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Git  bool   `json:"git"`
	}
	wantDirs struct {
		Path      string      `json:"path"`
		Entries   []wantEntry `json:"entries"`
		Truncated bool        `json:"truncated"`
	}
)

// resultCases は 1 ツール分の {name, input, want}。
type resultCases[T any] []struct {
	Name  string `json:"name"`
	Input string `json:"input"`
	Want  T      `json:"want"`
}

// checkResults は各ケースの input を parse で解析し、convert で want と同じ形にして比べる。
func checkResults[T, R any](t *testing.T, cases resultCases[T], parse func([]byte) (R, error), convert func(R) T) int {
	t.Helper()
	for _, c := range cases {
		got, err := parse([]byte(c.Input))
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if g := convert(got); !reflect.DeepEqual(g, c.Want) {
			t.Errorf("%s:\n got  %+v\n want %+v", c.Name, g, c.Want)
		}
	}
	return len(cases)
}

func TestVectorsToolResults(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		PaneTail    resultCases[wantTail]   `json:"pane_tail"`
		AskCard     resultCases[wantCard]   `json:"ask_card"`
		AskPick     resultCases[wantPick]   `json:"ask_pick"`
		SendText    resultCases[wantSend]   `json:"send_text"`
		SendEscape  resultCases[wantEscape] `json:"send_escape"`
		PaneStart   resultCases[wantStart]  `json:"pane_start"`
		PaneClose   resultCases[wantClose]  `json:"pane_close"`
		DirList     resultCases[wantDirs]   `json:"dir_list"`
		Invalid     []struct {
			Name     string `json:"name"`
			Tool     string `json:"tool"`
			InputRaw string `json:"input_raw"`
		} `json:"invalid"`
	}
	loadVectors(t, "toolresult.json", &f)

	n := checkResults(t, f.PaneTail, ops.ParsePaneTail, func(r ops.PaneTail) wantTail { return wantTail(r) })
	n += checkResults(t, f.AskCard, ops.ParseAskCard, func(r ops.AskCard) wantCard {
		w := wantCard{Asking: r.Asking, Question: r.Question, Options: []wantOption{}, Cursor: r.Cursor, AllowsCustom: r.AllowsCustom}
		for _, o := range r.Options {
			w.Options = append(w.Options, wantOption(o))
		}
		return w
	})
	n += checkResults(t, f.AskPick, ops.ParsePick, func(r ops.PickResult) wantPick { return wantPick(r) })
	n += checkResults(t, f.SendText, ops.ParseSend, func(r ops.SendResult) wantSend { return wantSend(r) })
	n += checkResults(t, f.SendEscape, ops.ParseEscape, func(r ops.EscapeResult) wantEscape { return wantEscape(r) })
	n += checkResults(t, f.PaneStart, ops.ParseStart, func(r ops.StartResult) wantStart { return wantStart(r) })
	n += checkResults(t, f.PaneClose, ops.ParseClose, func(r ops.CloseResult) wantClose {
		return wantClose{OK: r.OK, State: r.State, NoteArchived: r.NoteArchived}
	})
	n += checkResults(t, f.DirList, ops.ParseDirList, func(r ops.DirList) wantDirs {
		w := wantDirs{Path: r.Path, Entries: []wantEntry{}, Truncated: r.Truncated}
		for _, e := range r.Entries {
			w.Entries = append(w.Entries, wantEntry(e))
		}
		return w
	})
	requireMinCases(t, "toolresult.json", n+len(f.Invalid), minToolResultCases)
	checkInvalidResults(t, f.Invalid)
}

func checkInvalidResults(t *testing.T, cases []struct {
	Name     string `json:"name"`
	Tool     string `json:"tool"`
	InputRaw string `json:"input_raw"`
},
) {
	t.Helper()
	parsers := map[string]func([]byte) error{
		"pane_tail":   func(b []byte) error { _, err := ops.ParsePaneTail(b); return err },
		"ask_card":    func(b []byte) error { _, err := ops.ParseAskCard(b); return err },
		"ask_pick":    func(b []byte) error { _, err := ops.ParsePick(b); return err },
		"send_text":   func(b []byte) error { _, err := ops.ParseSend(b); return err },
		"send_escape": func(b []byte) error { _, err := ops.ParseEscape(b); return err },
		"pane_start":  func(b []byte) error { _, err := ops.ParseStart(b); return err },
		"pane_close":  func(b []byte) error { _, err := ops.ParseClose(b); return err },
		"dir_list":    func(b []byte) error { _, err := ops.ParseDirList(b); return err },
	}
	for _, c := range cases {
		parse, ok := parsers[c.Tool]
		if !ok {
			t.Fatalf("%s: 未知の tool %q", c.Name, c.Tool)
		}
		err := parse([]byte(c.InputRaw))
		if !errors.Is(err, domain.ErrInvalidResponse) {
			t.Errorf("%s: err = %v; want ErrInvalidResponse", c.Name, err)
			continue
		}
		if c.InputRaw != "" && strings.Contains(err.Error(), c.InputRaw) {
			t.Errorf("%s: エラーに本文が含まれる: %v", c.Name, err)
		}
	}
}

// repeated は text・title の入力。repeat があれば char を count 回繰り返す。
type repeated struct {
	Text   string `json:"text"`
	Repeat *struct {
		Char  string `json:"char"`
		Count int    `json:"count"`
	} `json:"repeat"`
}

func (r repeated) value() string {
	if r.Repeat != nil {
		return strings.Repeat(r.Repeat.Char, r.Repeat.Count)
	}
	return r.Text
}

func TestVectorsOpsInput(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		TextCases   []struct {
			Name string `json:"name"`
			repeated
			Want string `json:"want"`
		} `json:"text_cases"`
		TitleCases []struct {
			Name  string  `json:"name"`
			Input string  `json:"input"`
			Want  *string `json:"want"`
		} `json:"title_cases"`
		TailLinesCases []struct {
			Name  string `json:"name"`
			Lines int    `json:"lines"`
			Want  int    `json:"want"`
		} `json:"tail_lines_cases"`
	}
	loadVectors(t, "opsinput.json", &f)
	requireMinCases(t, "opsinput.json", len(f.TextCases)+len(f.TitleCases)+len(f.TailLinesCases), minOpsInputCases)

	for _, c := range f.TextCases {
		if got := ops.CheckText(c.value()); got != c.Want {
			t.Errorf("text: %s: got %q; want %q", c.Name, got, c.Want)
		}
		// ask_pick の「その他」は同じ規則。番号との同時指定は不可。
		if got := (ops.Pick{Text: c.value()}).Check(); got != c.Want {
			t.Errorf("pick text: %s: got %q; want %q", c.Name, got, c.Want)
		}
	}
	for _, c := range f.TitleCases {
		out, code := ops.StartRequest{Agent: "omp", Cwd: "/work", Title: c.Input}.Normalize()
		switch {
		case c.Want == nil && code == "":
			t.Errorf("title: %s: 受け付けた（%q）", c.Name, out.Title)
		case c.Want != nil && (code != "" || out.Title != *c.Want):
			t.Errorf("title: %s: got %q, %q; want %q", c.Name, out.Title, code, *c.Want)
		}
	}
	for _, c := range f.TailLinesCases {
		if got := ops.NormalizeTailLines(c.Lines); got != c.Want {
			t.Errorf("tail lines: %s: got %d; want %d", c.Name, got, c.Want)
		}
	}
}
