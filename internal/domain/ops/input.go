package ops

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// 操作・入力の上限（API 仕様 §5.2）。
const (
	// MaxSendChars は send_text・ask_pick の text の最大文字数。
	MaxSendChars = 8000
	// MaxTitleChars は pane_start の title の最大文字数。
	MaxTitleChars = 80
	// DefaultTailLines・MaxTailLines は pane_tail の lines の既定と上限。
	DefaultTailLines = 30
	MaxTailLines     = 200
)

// StartAgents は pane_start で起動できるエージェント（表示順）。
var StartAgents = []string{"omp", "claude", "codex"}

// Error は送る前の検査で拒否したときの失敗（通信しない）。code は swarm-console と同じ語彙。
func Error(code string) *domain.FetchError {
	return &domain.FetchError{Kind: domain.ErrOperation, Code: code}
}

// Pick は ask_pick で送る選択。番号（N ≥ 1）か「その他」の文字列（Text）のどちらか一方だけ。
type Pick struct {
	N    int
	Text string
}

// Check は送る前の検査。問題がなければ空文字、あれば swarm-console と同じエラーコードを返す。
func (p Pick) Check() string {
	switch {
	case p.N != 0 && p.Text != "":
		return "invalid_args" // 同時指定は不可
	case p.N != 0:
		if p.N < 1 {
			return "invalid_args"
		}
		return ""
	default:
		return CheckText(p.Text)
	}
}

// CheckText は send_text・ask_pick の本文の検査。問題がなければ空文字、あれば swarm-console と同じエラーコードを返す。
// 空は invalid_args、8000 字（rune）超は text_too_long、改行（\n）とタブ以外の制御文字（\r・C0・DEL・C1）は forbidden_control_char。
// 呼び出し側は改行を \n に揃えてから渡す（ブラウザのフォームは CRLF で送る）。
func CheckText(text string) string {
	if text == "" {
		return "invalid_args"
	}
	if utf8.RuneCountInString(text) > MaxSendChars {
		return "text_too_long"
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "forbidden_control_char"
		}
	}
	return ""
}

// StartRequest は pane_start の入力。
type StartRequest struct {
	Agent string
	Cwd   string
	Title string
}

// Normalize は Title の前後の空白を除いた入力を返し、検査する。問題がなければ code は空。
// agent は StartAgents のいずれか、cwd は空でなく制御文字を含まない、title は 1〜80 字で制御文字を含まない。
// 許可フォルダかどうかは swarm-console が判定する（forbidden_path）。
func (r StartRequest) Normalize() (out StartRequest, code string) {
	out = StartRequest{Agent: r.Agent, Cwd: r.Cwd, Title: strings.TrimSpace(r.Title)}
	n := utf8.RuneCountInString(out.Title)
	valid := isAgent(out.Agent) && out.Cwd != "" && n >= 1 && n <= MaxTitleChars &&
		!strings.ContainsFunc(out.Cwd, unicode.IsControl) && !strings.ContainsFunc(out.Title, unicode.IsControl)
	if !valid {
		return out, "invalid_args"
	}
	return out, ""
}

func isAgent(s string) bool {
	for _, a := range StartAgents {
		if a == s {
			return true
		}
	}
	return false
}

// NormalizeTailLines は pane_tail の lines を 1〜200 に収める。0 以下は既定の 30。
func NormalizeTailLines(n int) int {
	switch {
	case n <= 0:
		return DefaultTailLines
	case n > MaxTailLines:
		return MaxTailLines
	default:
		return n
	}
}

// Failure は操作系のツール（セッション・ペインを引数に取るもの）が isError で返した失敗を分類する。
// 通常は domain.Classify と同じだが、invalid_args は details.field が "session"（sessions にない名前）のときだけ
// ErrSessionMissing、それ以外（ペイン・入力の不正）は ErrOperation にする。field は応答由来の信頼できない値で、判定にだけ使う。
func Failure(toolCode, detailsField string) *domain.FetchError {
	if toolCode == "invalid_args" && detailsField != "session" {
		return Error("invalid_args")
	}
	return domain.Classify(nil, http.StatusOK, toolCode)
}
