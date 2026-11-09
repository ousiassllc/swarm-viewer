package static_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/static"
)

// ホバーの色の遷移は --motion-base（180ms）・ease-out で、色・背景・枠・下線の色だけを変える。reduced-motion では遷移を止める（M1・M3）。
func TestAppCSS_HoverFade(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)
	for _, want := range []string{
		".hover-fade{transition:color var(--motion-base) var(--ease-out)",
		"background-color var(--motion-base) var(--ease-out)",
		"border-color var(--motion-base) var(--ease-out)",
		"text-decoration-color var(--motion-base) var(--ease-out)",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
	// 動きを伴う transform や opacity は遷移させない（ホバーは色・背景のみ）。
	if i := strings.Index(css, ".hover-fade{"); i >= 0 {
		rule := css[i : i+strings.Index(css[i:], "}")]
		for _, banned := range []string{"transform", "opacity", "all"} {
			if strings.Contains(rule, banned) {
				t.Errorf(".hover-fade が %q を遷移させている: %s", banned, rule)
			}
		}
	}
	if !strings.Contains(css, ".hover-fade{transition:none}") {
		t.Error("reduced-motion のブロックが .hover-fade の遷移を止めていない")
	}
}

// テキストリンクは下線 --mute・位置 3px で、ホバーは下線の色を --ink にするだけ（面は付けない。ガイド O7）。
func TestAppCSS_TextLink(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)
	// Tailwind（Lightning CSS）が -webkit- 付きの宣言を足すので、宣言ごとに確かめる。
	for _, want := range []string{
		".text-link{text-decoration-line:underline;",
		"text-decoration-color:var(--mute)",
		"text-underline-offset:3px}",
		".text-link:hover{",
		"text-decoration-color:var(--ink)}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
}

// 画面のガターを切り替える幅の区分（medium 600px・expanded 840px）は Android の WindowWidthSizeClass と同じ境目で、app.css が持つ（NFR-DSG-10）。
func TestAppCSS_WidthClassGutters(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readAppCSS(t), " ", "")
	for _, want := range []string{
		"@media(min-width:600px){.medium\\:p-6{padding:calc(var(--spacing)*6)}}",
		"@media(min-width:840px){.expanded\\:p-8{padding:calc(var(--spacing)*8)}}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
}
