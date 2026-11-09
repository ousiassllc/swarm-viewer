package static_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/static"
)

// readAppCSS は埋め込まれた app.css（生成物）を読む。
//
// モーダル（確認ダイアログ・Esc・外側クリック・data-dismiss・退出フェードなど）の JS の挙動は、static/src/js/modal.test.js・app.test.js
// （`bun test static/src/js`。CI の test ジョブと task test が実行する）で確かめる。このファイルは CSS のトークンだけを見る。
func readAppCSS(t *testing.T) string {
	t.Helper()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	return string(data)
}

// braceBlock は s の open（"{" の位置）から対応する "}" までの中身（波括弧は含まない）と、次の位置を返す。対応が無ければ ok=false。
func braceBlock(s string, open int) (body string, next int, ok bool) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, true
			}
		}
	}
	return "", 0, false
}

var (
	darkMediaRE    = regexp.MustCompile(`@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)\s*\{`)
	backdropRuleRE = regexp.MustCompile(`dialog\.modal::backdrop\s*\{`)
	shadowModalRE  = regexp.MustCompile(`--shadow-modal\s*:\s*([^;}]+)`)
)

// darkBlocksAndRest は CSS を、prefers-color-scheme: dark の @media ブロックの中身（連結）と、それ以外に分ける。
func darkBlocksAndRest(t *testing.T, css string) (dark, rest string) {
	t.Helper()

	var darkParts, restParts []string
	pos := 0
	for _, loc := range darkMediaRE.FindAllStringIndex(css, -1) {
		if loc[0] < pos {
			continue // 別のダークのブロックの中にある
		}
		body, next, ok := braceBlock(css, loc[1]-1)
		if !ok {
			t.Fatalf("prefers-color-scheme: dark のブロックの波括弧が閉じていない: %.80s", css[loc[0]:])
		}
		restParts = append(restParts, css[pos:loc[0]])
		darkParts = append(darkParts, body)
		pos = next
	}
	restParts = append(restParts, css[pos:])
	return strings.Join(darkParts, "\n"), strings.Join(restParts, "\n")
}

// モーダルの影はダークで見えるよう、prefers-color-scheme: dark のブロックでライトとは別の（濃い）値に上書きする（ガイド C5）。
// Tailwind の minify の書き方（0 1px 2px #0006 など）には依存せず、ダークのブロックに --shadow-modal があり、
// ライト側の値と違うことだけを見る。
func TestAppCSS_ShadowModalIsOverriddenInDark(t *testing.T) {
	t.Parallel()

	dark, rest := darkBlocksAndRest(t, readAppCSS(t))
	if dark == "" {
		t.Fatal("app.css に prefers-color-scheme: dark のブロックが無い")
	}
	darkValue := shadowModalRE.FindStringSubmatch(dark)
	if darkValue == nil {
		t.Fatalf("ダークのブロックに --shadow-modal が無い: %s", dark)
	}
	lightValue := shadowModalRE.FindStringSubmatch(rest)
	if lightValue == nil {
		t.Fatal("ダーク以外に --shadow-modal の定義（ライト）が無い")
	}
	if strings.TrimSpace(darkValue[1]) == strings.TrimSpace(lightValue[1]) {
		t.Errorf("--shadow-modal がダークでもライトと同じ値のまま: %s", darkValue[1])
	}
}

// ダークの --shadow-modal は、モーダルの規則が var(--shadow-modal) で読んで初めて効く。Tailwind v4 の @theme に置くと値がクラスに埋め込まれ、
// ダークの上書きは読まれない（#82）。生成された CSS に var(--shadow-modal) を読む規則があり、.shadow-modal の埋め込みが無いことを見る。
func TestAppCSS_ModalShadowReadsToken(t *testing.T) {
	t.Parallel()

	css := readAppCSS(t)
	if !strings.Contains(css, "box-shadow:var(--shadow-modal)") {
		t.Error("var(--shadow-modal) を box-shadow に読む規則が無い（ダークの影が効かない）")
	}
	if strings.Contains(css, ".shadow-modal") {
		t.Error(".shadow-modal がある（@theme の値がクラスに埋め込まれ、ダークの上書きが効かない）")
	}
}

// モーダルのスクラム（::backdrop）はトークン --scrim の色で、出現と退出のフェードのキーフレームを持つ（ガイド O1）。
func TestAppCSS_ModalBackdropUsesScrimToken(t *testing.T) {
	t.Parallel()

	css := readAppCSS(t)
	loc := backdropRuleRE.FindStringIndex(css)
	if loc == nil {
		t.Fatal("app.css に dialog.modal::backdrop の規則が無い")
	}
	rule, _, ok := braceBlock(css, loc[1]-1)
	if !ok || !strings.Contains(rule, "var(--scrim)") {
		t.Errorf("dialog.modal::backdrop が --scrim を使っていない: %s", rule)
	}
	for _, name := range []string{"backdrop-in", "backdrop-out"} {
		if !strings.Contains(css, "@keyframes "+name) {
			t.Errorf("app.css に @keyframes %s が無い", name)
		}
	}
}

var scrimRE = regexp.MustCompile(`--scrim\s*:\s*color-mix\(\s*in\s+srgb\s*,\s*var\(--(\w+)\)\s*(\d+)%`)

// スクラムの濃さはガイドの見本（ink 18〜22%）に合わせる（#92）。ライトは --ink の 20%。
// ダークは --ink が白になるので、暗い --bone の 60%（--bone は背景とほぼ同色で、20% では暗幕にならない）。
func TestAppCSS_ScrimDarkness(t *testing.T) {
	t.Parallel()

	dark, rest := darkBlocksAndRest(t, readAppCSS(t))
	for _, tc := range []struct {
		name, css, token, percent string
	}{
		{"ライト", rest, "ink", "20"},
		{"ダーク", dark, "bone", "60"},
	} {
		m := scrimRE.FindStringSubmatch(tc.css)
		if m == nil {
			t.Errorf("%s: --scrim が color-mix(in srgb, var(--トークン) N%%, …) の形で見つからない", tc.name)
			continue
		}
		if m[1] != tc.token || m[2] != tc.percent {
			t.Errorf("%s: --scrim は %s の %s%% のはずが %s の %s%%", tc.name, tc.token, tc.percent, m[1], m[2])
		}
	}
}
