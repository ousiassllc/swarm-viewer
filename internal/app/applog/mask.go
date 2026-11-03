// Package applog は slog + lumberjack のログ設定とマスクを持つ（NFR-LOG-1〜4）。
// Wails に依存しないので、Linux の CI でもテストできる。
package applog

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
)

// Redacted は伏せた値の置き換え文字列。
const Redacted = "[REDACTED]"

// fullyMaskedKeys は値を丸ごと伏せるキー（小文字で比較する）。
// token・認証情報・ペインの本文（activity・note）・リクエスト/レスポンスの本文を含む。
var fullyMaskedKeys = map[string]struct{}{
	"token":         {},
	"access_token":  {},
	"authorization": {},
	"bearer":        {},
	"password":      {},
	"secret":        {},
	"credential":    {},
	"cookie":        {},
	"body":          {},
	"payload":       {},
	"activity":      {},
	"note":          {},
	"content":       {},
	"pane":          {},
}

// hostOnlyKeys は URL を host のみに丸めるキー。
var hostOnlyKeys = map[string]struct{}{
	"url":      {},
	"uri":      {},
	"endpoint": {},
	"link":     {},
	"href":     {},
	"referer":  {},
}

var (
	// urlPattern は文字列中の URL（scheme://...）を拾う。userinfo・path・クエリ・フラグメントは捨てる対象。
	urlPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://[^\s"'<>]+`)
	// bearerPattern は Authorization ヘッダ値の取りこぼしを防ぐ。
	bearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[^\s"']+`)
)

// ReplaceAttr は slog.HandlerOptions.ReplaceAttr に渡すマスク関数。
//
//   - token・authorization・body・activity・note など: 値を伏せる
//   - url など: host のみに丸める（解釈できなければ伏せる）
//   - それ以外の文字列・error・任意値: 中の URL を scheme://host に、Bearer 値を伏せる
func ReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if _, ok := fullyMaskedKeys[key]; ok {
		return slog.String(a.Key, Redacted)
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindGroup:
		return slog.Attr{Key: a.Key, Value: v}
	case slog.KindString, slog.KindAny:
		if _, ok := hostOnlyKeys[key]; ok {
			return slog.String(a.Key, HostOnly(stringOf(v)))
		}
		return slog.String(a.Key, Scrub(stringOf(v)))
	default:
		if _, ok := hostOnlyKeys[key]; ok {
			return slog.String(a.Key, Redacted)
		}
		return slog.Attr{Key: a.Key, Value: v}
	}
}

func stringOf(v slog.Value) string {
	if v.Kind() == slog.KindString {
		return v.String()
	}
	if v.Any() == nil {
		return "<nil>"
	}
	return fmt.Sprint(v.Any())
}

// HostOnly は URL から host（port を含む）だけを返す。解釈できない・host が空なら Redacted。
func HostOnly(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return Redacted
	}
	return u.Host
}

// Scrub は文字列中の URL を scheme://host に縮め、Bearer の値を伏せる。
func Scrub(s string) string {
	if s == "" {
		return s
	}
	s = urlPattern.ReplaceAllStringFunc(s, shrinkURL)
	return bearerPattern.ReplaceAllString(s, "Bearer "+Redacted)
}

func shrinkURL(raw string) string {
	// 末尾の句読点（"... see http://x/y." など）は URL に含めない。
	trimmed := strings.TrimRight(raw, ".,;:)]}")
	suffix := raw[len(trimmed):]
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return Redacted + suffix
	}
	return u.Scheme + "://" + u.Host + suffix
}
