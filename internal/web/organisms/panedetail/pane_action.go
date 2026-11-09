package panedetail

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/a-h/templ"

	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// LiveID はペイン詳細の本文（部分描画の差し替え先）の id。操作の結果もここへ outerHTML で返す。
const LiveID = "pane-live"

// BannerID は取得失敗バナーの id（ポーリングの失敗は OOB でこれだけを差し替え、直前の表示を残す）。
const BannerID = "pane-banner"

// SendInputID は指示の入力欄の id。ポーリングの再描画でも入力中の文字を残す（hx-preserve）ために必要。
const SendInputID = "pane-send-text"

// CustomLinesID は「その他」の入力が送る表示行数の hidden の id（保持される <details> の外に置く）。
const CustomLinesID = "pane-custom-lines"

// PreemptPoll は押した操作（ボタン・フォーム）の hx-sync。コンテナ #pane-live は自動更新を this:drop で守るが、子の操作まで
// 黙って捨てられないよう、操作の側が飛行中の自動更新（GET）を中断して自分を通す。逆に操作の実行中に来た自動更新は、
// コンテナの drop で捨てられる。
const PreemptPoll = "#" + LiveID + ":replace"

// PollSeconds はペイン詳細を開いている間の自動更新の間隔（秒）。
const PollSeconds = 5

// 操作の form 値のキー。
const (
	FieldN       = "n"
	FieldText    = "text"
	FieldConfirm = "confirm"
	FieldLines   = "lines"
	FieldAgent   = "agent"
	FieldCwd     = "cwd"
	FieldTitle   = "title"
	FieldPath    = "path"
	FieldSkill   = "skill"
	FieldTask    = "task"
	FieldResend  = "resend"
)

// Ref はペイン詳細の対象（接続・セッション・ペイン位置）。URL の組み立てに使う。
type Ref struct {
	ConnectionID string
	Session      string
	Pos          string
}

func (r Ref) live(lines int, first bool) string {
	u := paths.PaneLive(r.ConnectionID, r.Session, r.Pos) + "?" + FieldLines + "=" + strconv.Itoa(lines)
	if first {
		u += "&first=1"
	}
	return u
}

// actionAttrs は操作の hx-* 属性。結果は #pane-live を outerHTML で差し替え、同じペインのボタン・入力を送信中は全部無効にする
// （二重送信の防止。サーバー側にも排他がある）。url が操作先、vals が追加の form 値（lines を含める）。
func actionAttrs(target string, vals map[string]string) templ.Attributes {
	b, err := json.Marshal(vals)
	if err != nil { // map[string]string の Marshal は失敗しない
		b = []byte("{}")
	}
	return templ.Attributes{
		"hx-post":         target,
		"hx-vals":         string(b),
		"hx-target":       "#" + LiveID,
		"hx-swap":         "outerHTML",
		"hx-disabled-elt": "#" + LiveID + " button, #" + LiveID + " textarea",
		"hx-sync":         PreemptPoll,
	}
}

// queryEscape はクエリ値のエスケープ（フォルダのパスは `/` やスペースを含む）。
func queryEscape(s string) string { return url.QueryEscape(s) }
