package layout_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func render(t *testing.T, token string) string {
	t.Helper()
	child := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p id=\"child\">本文</p>"))
		return err
	})
	var sb strings.Builder
	ctx := templ.WithChildren(context.Background(), child)
	if err := layout.Base(layout.BaseProps{Title: "タイトル", CSRFToken: token}).Render(ctx, &sb); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func TestBase_HeadAndBody(t *testing.T) {
	t.Parallel()
	out := render(t, "csrf-test-token")
	webtest.MustContain(t, out,
		`<!doctype html>`, `lang="ja"`, `<title>タイトル</title>`,
		`<meta name="csrf-token" content="csrf-test-token">`,
		`href="/static/css/app.css"`,
		`src="/static/js/htmx.min.js"`, `src="/static/js/response-targets.min.js"`, `src="/static/js/app.js"`,
		`hx-ext="response-targets"`, `<main id="main"`, `id="child"`, `id="toasts"`,
		"p-4 md:p-6", // ガターはモバイル 16・デスクトップ 24（ガイド S5）
	)
}

func TestBase_HTMXConfigMeta(t *testing.T) {
	t.Parallel()
	metas := webtest.Elements(t, render(t, "x"), "meta")
	var content string
	for _, m := range metas {
		if n, _ := webtest.Attr(m, "name"); n == "htmx-config" {
			content, _ = webtest.Attr(m, "content")
		}
	}
	if content != `{"allowEval":false,"includeIndicatorStyles":false}` {
		t.Errorf("htmx-config = %q", content)
	}
}

func TestBase_HxHeadersCarriesCSRF(t *testing.T) {
	t.Parallel()
	out := render(t, `tok"en<>&`)
	var got string
	for _, b := range webtest.Elements(t, out, "body") {
		got, _ = webtest.Attr(b, "hx-headers")
	}
	// 特殊文字を含んでも壊れた JSON / 属性にならない。
	want := `{"X-CSRF-Token":"tok\"en\u003c\u003e\u0026"}`
	if got != want {
		t.Errorf("hx-headers = %q, want %q", got, want)
	}
	if strings.Contains(out, `tok"en<>&`) {
		t.Error("エスケープされていない CSRF トークンが出力されている")
	}
}

// インラインスクリプト・インラインスタイル・hx-on・フィルタ式を使わない（CSP、NFR-SEC-9）。
func TestBase_NoInlineScriptOrStyle(t *testing.T) {
	t.Parallel()
	out := render(t, "x")
	scripts := webtest.Elements(t, out, "script")
	if len(scripts) != 3 {
		t.Errorf("script は外部 3 本だけ: %d", len(scripts))
	}
	for _, s := range scripts {
		if src, ok := webtest.Attr(s, "src"); !ok || !strings.HasPrefix(src, "/static/") {
			t.Errorf("src のない・外部の script: %v", s.Attr)
		}
		if s.FirstChild != nil {
			t.Error("script に本文がある")
		}
	}
	if len(webtest.Elements(t, out, "style")) != 0 {
		t.Error("<style> がある")
	}
	keys := webtest.AllAttrKeys(t, out)
	for k := range keys {
		if k == "style" || strings.HasPrefix(k, "on") || strings.HasPrefix(k, "hx-on") {
			t.Errorf("禁止の属性 %q", k)
		}
	}
	webtest.MustNotContain(t, out, "http://", "https://", "javascript:")
}
