// Package ops は swarm-console のペインを見る・操作するツール（pane_tail・ask_card・ask_pick・send_text・send_escape・
// pane_start・pane_close・dir_list）の入力の検査と結果の解析を持つ（API 仕様 §5.1・§5.2）。
//
// domain と同じく純粋で、標準ライブラリと domain だけを使う（linterly の 1 ディレクトリの行数上限のため domain とは別のパッケージ）。
// 結果の解析は、未知のキーを無視し、欠落・null の項目は零値、JSON が壊れている・型が想定外なら domain.ErrInvalidResponse にする（応答本文はエラーに含めない）。
package ops

import (
	"bytes"
	"encoding/json"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// PaneTail は pane_tail の結果。Text は信頼できない入力（HTML として解釈しない）。
type PaneTail struct {
	Text      string
	Truncated bool
}

// AskOption は質問カードの選択肢 1 つ。N は 1 始まりの番号。
type AskOption struct {
	N           int
	Label       string
	Recommended bool
}

// AskCard は ask_card の結果。Asking が false のとき、ほかの項目は零値（Options は空のスライス）。
type AskCard struct {
	Asking       bool
	Question     string
	Options      []AskOption
	Cursor       int
	AllowsCustom bool
}

// PickResult は ask_pick の結果。OK かつ CardClosed なら選択は確定している。
type PickResult struct {
	OK          bool
	State       string
	PickedN     int
	PickedLabel string
	CardClosed  bool
}

// SendResult は send_text の結果。Accepted が false なら受け付けられなかった（エラーではない）。
type SendResult struct {
	OK       bool
	State    string
	Accepted bool
	Chars    int
}

// EscapeResult は send_escape の結果。
type EscapeResult struct {
	OK          bool
	State       string
	Interrupted bool
}

// StartResult は pane_start の結果。
type StartResult struct {
	OK    bool
	State string
	Agent string
	Cwd   string
	Title string
}

// CloseResult は pane_close の結果。
type CloseResult struct {
	OK           bool
	State        string
	NoteArchived bool
}

// DirEntry は dir_list の 1 件。Git は直下に .git があるフォルダ。
type DirEntry struct {
	Name string
	Path string
	Git  bool
}

// DirList は dir_list の結果。Path は path を省略したとき空。Truncated は一部のみ（件数上限）。
type DirList struct {
	Path      string
	Entries   []DirEntry
	Truncated bool
}

// decode はツール結果の JSON（content[0].text）を dst に読み込む。オブジェクトでない・型が想定外なら ErrInvalidResponse。
// 欠落・null の項目は零値になる（ParseReport と同じ）。
func decode(data []byte, dst any) error {
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) || json.Unmarshal(data, dst) != nil {
		return domain.ErrInvalidResponse
	}
	return nil
}

// ParsePaneTail は pane_tail の応答を解析する。
func ParsePaneTail(data []byte) (PaneTail, error) {
	var w struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	}
	if err := decode(data, &w); err != nil {
		return PaneTail{}, err
	}
	return PaneTail{Text: w.Text, Truncated: w.Truncated}, nil
}

// ParseAskCard は ask_card の応答を解析する。asking が false なら残りは捨てて零値にする。
func ParseAskCard(data []byte) (AskCard, error) {
	var w struct {
		Asking   bool   `json:"asking"`
		Question string `json:"question"`
		Options  []struct {
			N           int    `json:"n"`
			Label       string `json:"label"`
			Recommended bool   `json:"recommended"`
		} `json:"options"`
		Cursor       int  `json:"cursor"`
		AllowsCustom bool `json:"allows_custom"`
	}
	if err := decode(data, &w); err != nil {
		return AskCard{}, err
	}
	card := AskCard{Options: []AskOption{}}
	if !w.Asking {
		return card, nil
	}
	card.Asking, card.Question, card.Cursor, card.AllowsCustom = true, w.Question, w.Cursor, w.AllowsCustom
	for _, o := range w.Options {
		card.Options = append(card.Options, AskOption{N: o.N, Label: o.Label, Recommended: o.Recommended})
	}
	return card, nil
}

// ParsePick は ask_pick の応答を解析する。
func ParsePick(data []byte) (PickResult, error) {
	var w struct {
		OK     bool   `json:"ok"`
		State  string `json:"state"`
		Picked struct {
			N     int    `json:"n"`
			Label string `json:"label"`
		} `json:"picked"`
		CardClosed bool `json:"card_closed"`
	}
	if err := decode(data, &w); err != nil {
		return PickResult{}, err
	}
	return PickResult{OK: w.OK, State: w.State, PickedN: w.Picked.N, PickedLabel: w.Picked.Label, CardClosed: w.CardClosed}, nil
}

// ParseSend は send_text の応答を解析する。
func ParseSend(data []byte) (SendResult, error) {
	var w struct {
		OK       bool   `json:"ok"`
		State    string `json:"state"`
		Accepted bool   `json:"accepted"`
		Chars    int    `json:"chars"`
	}
	if err := decode(data, &w); err != nil {
		return SendResult{}, err
	}
	return SendResult{OK: w.OK, State: w.State, Accepted: w.Accepted, Chars: w.Chars}, nil
}

// ParseEscape は send_escape の応答を解析する。
func ParseEscape(data []byte) (EscapeResult, error) {
	var w struct {
		OK          bool   `json:"ok"`
		State       string `json:"state"`
		Interrupted bool   `json:"interrupted"`
	}
	if err := decode(data, &w); err != nil {
		return EscapeResult{}, err
	}
	return EscapeResult{OK: w.OK, State: w.State, Interrupted: w.Interrupted}, nil
}

// ParseStart は pane_start の応答を解析する。
func ParseStart(data []byte) (StartResult, error) {
	var w struct {
		OK    bool   `json:"ok"`
		State string `json:"state"`
		Agent string `json:"agent"`
		Cwd   string `json:"cwd"`
		Title string `json:"title"`
	}
	if err := decode(data, &w); err != nil {
		return StartResult{}, err
	}
	return StartResult{OK: w.OK, State: w.State, Agent: w.Agent, Cwd: w.Cwd, Title: w.Title}, nil
}

// ParseClose は pane_close の応答を解析する。
func ParseClose(data []byte) (CloseResult, error) {
	var w struct {
		OK           bool   `json:"ok"`
		State        string `json:"state"`
		NoteArchived bool   `json:"note_archived"`
	}
	if err := decode(data, &w); err != nil {
		return CloseResult{}, err
	}
	return CloseResult{OK: w.OK, State: w.State, NoteArchived: w.NoteArchived}, nil
}

// ParseDirList は dir_list の応答を解析する。name か path が空の項目は捨てる。
func ParseDirList(data []byte) (DirList, error) {
	var w struct {
		Path    string `json:"path"`
		Entries []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Git  bool   `json:"git"`
		} `json:"entries"`
		Truncated bool `json:"truncated"`
	}
	if err := decode(data, &w); err != nil {
		return DirList{}, err
	}
	out := DirList{Path: w.Path, Entries: make([]DirEntry, 0, len(w.Entries)), Truncated: w.Truncated}
	for _, e := range w.Entries {
		if e.Name == "" || e.Path == "" {
			continue
		}
		out.Entries = append(out.Entries, DirEntry{Name: e.Name, Path: e.Path, Git: e.Git})
	}
	return out, nil
}
