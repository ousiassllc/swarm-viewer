// Package notice は OS 通知の中身の組み立てとクリック時の遷移先の復元を行う。
// Wails に依存しない（実際の送出は app/notifier.go）。
package notice

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// 通知 payload のキー。値は接続 ID とセッション名だけ（token・URL は載せない）。
const (
	DataConnectionID = "connection_id"
	DataSession      = "session"
)

// 表示上限（ルーン数）。ホストから来る文字列は信頼せず切り詰める。
const (
	maxTitleRunes  = 80
	maxLineRunes   = 120
	maxDetailRunes = 200
)

// Message は OS 通知 1 件の中身。
type Message struct {
	ID    string
	Title string
	Body  string
	Data  map[string]any
}

// Build は service.Notification から通知の中身を作る（FR-NTF 8.2）。
// タイトルは接続表示名、本文は「セッション・pos・状態ラベル」（asking は detail を添える）。
func Build(id string, n service.Notification) Message {
	title := clean(n.ConnectionName, maxTitleRunes)
	if title == "" {
		title = "swarm-viewer"
	}
	line := strings.Join(nonEmpty(
		clean(n.Session, maxLineRunes),
		clean(n.Pos, maxLineRunes),
		StateLabel(n.State),
	), " · ")
	body := line
	if detail := clean(n.Detail, maxDetailRunes); detail != "" {
		body += "\n" + detail
	}
	return Message{
		ID:    id,
		Title: title,
		Body:  body,
		Data:  map[string]any{DataConnectionID: n.ConnectionID, DataSession: n.Session},
	}
}

// StateLabel は通知対象の状態のラベル（機能要件 §3 の表に一致させる）。
func StateLabel(s domain.PaneState) string {
	switch s {
	case domain.StateAsking:
		return "答え待ち"
	case domain.StateStalled:
		return "止まってるかも"
	default:
		return string(s)
	}
}

// Target は通知のクリックで返ってきた payload から遷移先のパスを復元する。
// 接続 ID・セッション名のどちらかが欠けた・文字列でないときは ok=false。
func Target(data map[string]any) (string, bool) {
	cid, ok1 := data[DataConnectionID].(string)
	sid, ok2 := data[DataSession].(string)
	if !ok1 || !ok2 || cid == "" || sid == "" {
		return "", false
	}
	return paths.Session(cid, sid), true
}

// clean は制御文字（改行を含む）を空白にして連続空白を畳み、max ルーンで切る。
func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

func nonEmpty(in ...string) []string {
	out := in[:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
