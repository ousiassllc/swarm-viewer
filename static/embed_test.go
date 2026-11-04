package static_test

import (
	"io/fs"
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
