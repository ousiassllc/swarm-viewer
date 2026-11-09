// Package webtest はコンポーネントテストの共通ヘルパー。テストコードからだけ使う。
package webtest

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

// Render はコンポーネントを HTML 文字列に描画する。失敗はテスト失敗。
func Render(t testing.TB, c templ.Component) string {
	t.Helper()
	var sb strings.Builder
	if err := c.Render(context.Background(), &sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// MustContain は html に want がすべて含まれることを確認する。
func MustContain(t testing.TB, html string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(html, w) {
			t.Errorf("HTML に %q が含まれない:\n%s", w, html)
		}
	}
}

// MustNotContain は html に banned がどれも含まれないことを確認する。
func MustNotContain(t testing.TB, html string, banned ...string) {
	t.Helper()
	for _, b := range banned {
		if strings.Contains(html, b) {
			t.Errorf("HTML に %q が含まれている:\n%s", b, html)
		}
	}
}

// Elements は html を解析し、タグ名 tag の要素をすべて返す（文書順）。
func Elements(t testing.TB, htmlText, tag string) []*html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// Attr は要素の属性値を返す。無ければ空文字と false。
func Attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// AllAttrKeys は html 内のすべての要素の属性名を集める。
func AllAttrKeys(t testing.TB, htmlText string) map[string]bool {
	t.Helper()
	root, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	keys := map[string]bool{}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		for _, a := range n.Attr {
			keys[a.Key] = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return keys
}

// MustSwapMainInner は html の hx-target="#main" を持つ要素がすべて hx-swap="innerHTML" を自分で持つことを確認する。
// htmx は hx-swap を祖先から継承する。ポーリング領域（outerHTML）の中のリンクが継承すると、<main id="main"> ごと差し替わって
// 以降の #main への遷移（戻るリンクなど）が targetError になる。
func MustSwapMainInner(t testing.TB, htmlText string) {
	t.Helper()
	root, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found := 0
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if target, _ := Attr(n, "hx-target"); target == "#main" {
				found++
				if swap, _ := Attr(n, "hx-swap"); swap != "innerHTML" {
					t.Errorf("<%s hx-target=\"#main\"> の hx-swap = %q; want %q（祖先の hx-swap を継承させない）", n.Data, swap, "innerHTML")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if found == 0 {
		t.Errorf("hx-target=\"#main\" を持つ要素が無い:\n%s", htmlText)
	}
}

// Text は要素以下のテキスト（属性は含まない。画面に出る文字）を連結して返す。
func Text(n *html.Node) string {
	var sb strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
