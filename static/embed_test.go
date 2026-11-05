package static_test

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/static"
)

// レイアウトが読み込む 3 ファイルと response-targets 拡張が埋め込まれていること。
func TestFS_EmbedsRequiredAssets(t *testing.T) {
	t.Parallel()

	assets := static.FS()
	for _, name := range []string{
		"css/app.css",
		"js/htmx.min.js",
		"js/response-targets.min.js",
		"js/app.js",
	} {
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Errorf("%s を読めない（先に task generate を実行する）: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s が空", name)
		}
	}
}

// Nerd Font の Symbols Only（woff2）が埋め込まれていること（Issue #9）。
func TestFS_EmbedsSymbolsFont(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), fontPath)
	if err != nil {
		t.Fatalf("%s を読めない（先に task generate を実行する）: %v", fontPath, err)
	}
	// woff2 の先頭 4 バイトは "wOF2"。
	if !strings.HasPrefix(string(data), "wOF2") {
		t.Errorf("%s が woff2 でない（先頭 %q）", fontPath, data[:min(4, len(data))])
	}
}

const fontPath = "fonts/SymbolsNerdFontMono-Regular.woff2"

// @font-face の url() は CSS からの相対パスで、埋め込みのフォントに解決される（CSP default-src 'self'、外部 URL なし）。
// unicode-range は PUA に絞る（日本語・英字は OS のフォント）。
func TestAppCSS_FontFaceIsSelfHosted(t *testing.T) {
	t.Parallel()

	assets := static.FS()
	data, err := fs.ReadFile(assets, "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)

	face := regexp.MustCompile(`@font-face\{[^}]*\}`).FindString(css)
	if face == "" {
		t.Fatal("app.css に @font-face が無い")
	}
	for _, want := range []string{`font-family:Symbols Nerd Font`, `format("woff2")`, `font-display:swap`, `unicode-range:U+E000-F8FF,U+F0000-FFFFD`} {
		if !strings.Contains(face, want) {
			t.Errorf("@font-face に %q が無い: %s", want, face)
		}
	}

	urls := regexp.MustCompile(`url\(["']?([^)"']+)["']?\)`).FindAllStringSubmatch(css, -1)
	if len(urls) == 0 {
		t.Fatal("app.css に url() が無い")
	}
	for _, m := range urls {
		ref := m[1]
		if strings.Contains(ref, "://") || strings.HasPrefix(ref, "//") || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "/") {
			t.Errorf("url(%s) は CSS からの相対パスであるべき（'self' の範囲）", ref)
			continue
		}
		// /static/css/app.css からの相対パスは、埋め込み FS の css/ からの相対パスと同じ。
		if _, err := fs.Stat(assets, path.Join("css", ref)); err != nil {
			t.Errorf("url(%s) が埋め込みのアセットに解決されない: %v", ref, err)
		}
	}
	if !strings.Contains(css, "url(../"+fontPath+")") {
		t.Errorf("@font-face が %s を参照していない", fontPath)
	}
}

// app.js は CSP（unsafe-eval なし）の下で動く必要があるため eval 系を含まない。
func TestAppJS_NoEval(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "js/app.js")
	if err != nil {
		t.Fatalf("js/app.js を読めない（先に task generate を実行する）: %v", err)
	}
	src := string(data)
	for _, banned := range []string{"eval(", "new Function", "setTimeout(\"", "setInterval(\""} {
		if strings.Contains(src, banned) {
			t.Errorf("app.js に %q が含まれている", banned)
		}
	}
}
