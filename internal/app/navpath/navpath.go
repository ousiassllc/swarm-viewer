// Package navpath は WebView へ渡す遷移先パスの検証を行う（Wails 非依存）。
//
// SetURL に渡す値は同一プロセスのアセットハンドラのパスだけに限る。
// 外部 URL・別 scheme・プロトコル相対（//host）は WebView に開かせない（NFR-SEC-9）。
package navpath

import (
	"net/url"
	"strings"
	"unicode"
)

// Home は検証に失敗したときの遷移先。
const Home = "/"

// Clean は path が同一オリジンの絶対パス（`/` で始まり `//` や `\` を含まない）なら
// そのまま返す。違えば Home を返し ok=false にする。クエリは許可する（`/pairing?link=invalid`）。
func Clean(path string) (string, bool) {
	if path == "" || path[0] != '/' || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\\") {
		return Home, false
	}
	if strings.ContainsFunc(path, unicode.IsControl) {
		return Home, false
	}
	u, err := url.Parse(path)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Fragment != "" {
		return Home, false
	}
	return path, true
}
