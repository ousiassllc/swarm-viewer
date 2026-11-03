package web_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// templFiles は internal/web 配下の .templ ファイルを (パス, 内容) で返す。
// Go 側のコメント行（`//` で始まる行）は説明文として除く。
func templFiles(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return nil
		}
		b, err := os.ReadFile(path) //nolint:gosec // WalkDir が列挙した、このパッケージ配下のファイル
		if err != nil {
			return err
		}
		var kept []string
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			kept = append(kept, line)
		}
		out[path] = strings.Join(kept, "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 10 {
		t.Fatalf(".templ が %d 件しか見つからない（作業ディレクトリが internal/web でない？）", len(out))
	}
	return out
}

var (
	reInlineScript = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	reScriptSrc    = regexp.MustCompile(`(?i)\bsrc\s*=`)
	reStyleAttr    = regexp.MustCompile(`(?i)(^|[\s"'])style\s*=`)
	reStyleTag     = regexp.MustCompile(`(?i)<style\b`)
	reHxOn         = regexp.MustCompile(`(?i)\bhx-on[:\-\w]*\s*=`)
	reHandlerAttr  = regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)
	reJSURL        = regexp.MustCompile(`(?i)javascript:`)
	reTriggerExpr  = regexp.MustCompile(`(?i)hx-trigger\s*=\s*"[^"]*\[`)
	reExternalURL  = regexp.MustCompile(`(?i)(src|href)\s*=\s*"https?://`)
)

// NFR-SEC-10: templ.Raw を使わない。HTML として解釈しない。
func TestTemplFiles_NoTemplRaw(t *testing.T) {
	for path, src := range templFiles(t) {
		if strings.Contains(src, "templ.Raw") || strings.Contains(src, "templ.UnsafeRaw") {
			t.Errorf("%s: templ.Raw を使っている（NFR-SEC-10）", path)
		}
	}
}

// NFR-SEC-9: インラインスクリプト・インラインスタイル・hx-on・フィルタ式・外部 URL を使わない。
func TestTemplFiles_NoInlineScriptStyleOrEval(t *testing.T) {
	for path, src := range templFiles(t) {
		for _, tag := range reInlineScript.FindAllString(src, -1) {
			if !reScriptSrc.MatchString(tag) {
				t.Errorf("%s: src のない <script>（インラインスクリプト）: %s", path, tag)
			}
		}
		for name, re := range map[string]*regexp.Regexp{
			"style 属性":          reStyleAttr,
			"<style>":           reStyleTag,
			"hx-on":             reHxOn,
			"イベントハンドラ属性":        reHandlerAttr,
			"javascript: URL":   reJSURL,
			"hx-trigger のフィルタ式": reTriggerExpr,
			"外部 URL":            reExternalURL,
		} {
			if loc := re.FindStringIndex(src); loc != nil {
				t.Errorf("%s: %s を使っている: %q", path, name, strings.TrimSpace(src[max(0, loc[0]-20):min(len(src), loc[1]+30)]))
			}
		}
	}
}

// 読み取り専用のペインカードは、操作系の要素を持たない（FR-PANE）。ソースレベルでも確認する。
func TestPaneCardSource_HasNoActionElements(t *testing.T) {
	src, ok := templFiles(t)[filepath.Join("molecules", "pane_card.templ")]
	if !ok {
		t.Fatal("molecules/pane_card.templ が見つからない")
	}
	src = strings.ToLower(src)
	for _, banned := range []string{"<button", "<form", "<input", "<a ", "<select", "hx-post", "hx-put", "hx-delete", "hx-patch", "hx-get", "atoms.button", "atoms.input"} {
		if strings.Contains(src, banned) {
			t.Errorf("pane_card.templ に %q がある", banned)
		}
	}
}
