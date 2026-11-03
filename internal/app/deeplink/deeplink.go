// Package deeplink は swarm-viewer:// のディープリンクを引数・イベントから取り出し、
// 下書き作成（PairingService.Parse）と画面遷移につなぐ（FR-PAIR-1、NFR-SEC-7、NFR-AVL-6）。
// Wails に依存しない。
//
// ディープリンクには token が含まれる。この package は URL を WebView に渡さず（遷移先は下書き ID のみ）、
// ログにも種別（pair か other）以外を出さない。
package deeplink

import (
	"strings"
	"unicode"
)

// Scheme はディープリンクの scheme（build/config.yml の protocols と一致させる）。
const Scheme = "swarm-viewer"

const prefix = Scheme + "://"

// quoteChars は引数の前後に残りうる引用符。
const quoteChars = "\"'`“”‘’"

// Normalize は引数 1 つがディープリンクなら、前後の空白・引用符を除いて返す。
// scheme は大文字小文字を区別しない（Windows のシェルは大文字化することがある）。
// 他の scheme・スキームだけ・制御文字を含むものは ok=false。
func Normalize(arg string) (string, bool) {
	s := strings.TrimSpace(arg)
	s = strings.Trim(s, quoteChars)
	s = strings.TrimSpace(s)
	if len(s) <= len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", false
	}
	return Scheme + "://" + s[len(prefix):], true
}

// Extract は引数列から最初のディープリンクを返す。無ければ ok=false。
// args[0]（実行ファイル）も走査するが、scheme で始まらないので拾わない。
func Extract(args []string) (string, bool) {
	for _, a := range args {
		if link, ok := Normalize(a); ok {
			return link, true
		}
	}
	return "", false
}

// logKind はログ用に、ディープリンクの種別だけを返す。認識できるペアリングリンク（host が pair）なら "pair"、
// それ以外は固定の "other"。入力の断片（host 部分を含む）はログに出さない（区切りが欠けた不正なリンクは全体が host に見えるため）。
func logKind(link string) string {
	rest := link
	if len(rest) >= len(prefix) && strings.EqualFold(rest[:len(prefix)], prefix) {
		rest = rest[len(prefix):]
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if strings.EqualFold(rest, "pair") {
		return "pair"
	}
	return "other"
}
