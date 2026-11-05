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

// 同梱フォント（Nerd Font の Symbols Only・Ousiass デザインガイドの Sora / Noto Sans JP / JetBrains Mono）が
// woff2 として埋め込まれていること（Issue #9・#30）。
func TestFS_EmbedsFonts(t *testing.T) {
	t.Parallel()

	for _, name := range fontFiles {
		data, err := fs.ReadFile(static.FS(), name)
		if err != nil {
			t.Fatalf("%s を読めない（先に task generate を実行する）: %v", name, err)
		}
		// woff2 の先頭 4 バイトは "wOF2"。
		if !strings.HasPrefix(string(data), "wOF2") {
			t.Errorf("%s が woff2 でない（先頭 %q）", name, data[:min(4, len(data))])
		}
	}
}

const symbolsFontPath = "fonts/SymbolsNerdFontMono-Regular.woff2"

var fontFiles = []string{
	symbolsFontPath,
	"fonts/Sora-Regular.woff2",
	"fonts/Sora-SemiBold.woff2",
	"fonts/NotoSansJP-Regular.woff2",
	"fonts/NotoSansJP-SemiBold.woff2",
	"fonts/JetBrainsMono-Regular.woff2",
}

// @font-face の url() は CSS からの相対パスで、埋め込みのフォントに解決される（CSP default-src 'self'、外部 URL なし）。
// Symbols Nerd Font の unicode-range は PUA に絞る（PUA の文字が現れるまで読み込まない）。
func TestAppCSS_FontFaceIsSelfHosted(t *testing.T) {
	t.Parallel()

	assets := static.FS()
	data, err := fs.ReadFile(assets, "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)

	var symbols string
	for _, face := range regexp.MustCompile(`@font-face\{[^}]*\}`).FindAllString(css, -1) {
		if strings.Contains(face, `font-family:Symbols Nerd Font`) {
			symbols = face
		}
	}
	if symbols == "" {
		t.Fatal("app.css に Symbols Nerd Font の @font-face が無い")
	}
	for _, want := range []string{`format("woff2")`, `font-display:swap`, `unicode-range:U+E000-F8FF,U+F0000-FFFFD`} {
		if !strings.Contains(symbols, want) {
			t.Errorf("@font-face に %q が無い: %s", want, symbols)
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
	for _, name := range fontFiles {
		if !strings.Contains(css, "url(../"+name+")") {
			t.Errorf("@font-face が %s を参照していない", name)
		}
	}
}

// アコーディオン（<details>）は summary を SemiBold にし、ブラウザ既定の三角を消して CSS のシェブロンを付ける（ガイド §7）。
// シェブロンは見本（tokens-preview の .acc）どおり右端（::after）・閉じて下向き（45deg）・開いて上向き（-135deg）。
// CSP（default-src 'self'）で data: URI の画像を使えないので、枠線の回転で描く。
func TestAppCSS_Accordion(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)
	for _, want := range []string{"details>summary{", "justify-content:space-between", "font-weight:600", "list-style:none", "details>summary::-webkit-details-marker{display:none}", "details>summary:after{",
		"transition:transform var(--motion-base) var(--ease-out)", "transform:rotate(45deg)", "details[open]>summary:after{margin-top:.25rem;transform:rotate(-135deg)}"} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
	if strings.Contains(css, "summary:before") {
		t.Error("シェブロンは右端の ::after で、::before は使わない")
	}
	// 開閉マークの線はアイコンのストローク（≈1.5）で、色は --mute（ガイド I2）。
	for _, want := range []string{"border-right:1.5px solid var(--mute)", "border-bottom:1.5px solid var(--mute)"} {
		if !strings.Contains(css, want) {
			t.Errorf("開閉マークの線に %q が無い", want)
		}
	}
}

// 角丸の --r-full（9999px）は他の --r-* と同じく :root のトークンで、@theme（--radius-full）経由で Tailwind の rounded-full になる。
// リップルは数値を直書きせず --r-full を使う（ガイド S1）。
func TestAppCSS_RadiusFullIsToken(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)
	for _, want := range []string{"--r-full:9999px", "--radius-full:var(--r-full)", ".ripple{border-radius:var(--r-full)"} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
	if strings.Contains(css, "border-radius:9999px") {
		t.Error("app.css に角丸 9999px の直書きがある（--r-full を使う）")
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

// 出現・退出・リップルのモーションはガイドのトークン（320ms ease-out / 240ms ease-in / 280ms）で、
// reduced-motion では付かない（M3・M4・M5、NFR-DSG-6）。
func TestAppCSS_MotionUsesTokensAndRespectsReducedMotion(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	css := string(data)
	for _, want := range []string{
		"@keyframes fade-in", "@keyframes fade-out", "@keyframes ripple",
		"animation:fade-in var(--motion-fade-in) var(--ease-out)",
		"animation:fade-out var(--motion-fade-out) var(--ease-in)",
		"animation:ripple var(--motion-slow) var(--ease-out)",
		"--motion-slow:",
		// モーダルの出現は --motion-slow（280ms）。アラート・トーストの .fade-in（320ms）とは別（M5）。
		".modal-in{animation:fade-in var(--motion-slow) var(--ease-out)",
		"dialog.modal-in::backdrop{animation:backdrop-in var(--motion-slow) var(--ease-out)",
		".fade-in.is-leaving,.modal-in.is-leaving{animation:fade-out var(--motion-fade-out) var(--ease-in)",
		// スピナーの回転はガイドの 0.9s（M1）。
		"--animate-spin:spin .9s linear infinite",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
	// reduced-motion のブロックに、フェードとリップルを止める指定がある。
	idx := strings.LastIndex(css, "@media (prefers-reduced-motion:reduce){.fade-in")
	if idx < 0 {
		t.Fatal("app.css に .fade-in を止める prefers-reduced-motion のブロックが無い")
	}
	if block := css[idx:]; !strings.Contains(block, ".ripple{display:none}") {
		t.Errorf("reduced-motion のブロックが .ripple を止めていない: %s", block)
	}
}

// 退出フェードとトーストの時間は CSS のトークン（--motion-fade-out・--toast-duration）が唯一の出どころで、
// app.js は getComputedStyle で読む。数値の直書き（TOAST_MS・LEAVE_MS）を持たない（M1）。
func TestAppJS_ReadsDurationsFromCSSTokens(t *testing.T) {
	t.Parallel()

	css, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	// Tailwind の minify は 40px・3s をそのまま、240ms を .24s に直すので、どちらの書き方でも通るのは --motion-fade-out のほうだけ。
	for _, want := range []string{"--toast-h:40px", "--toast-duration:3s", "--motion-fade-out:.24s"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}

	js, err := fs.ReadFile(static.FS(), "js/app.js")
	if err != nil {
		t.Fatalf("js/app.js を読めない（先に task generate を実行する）: %v", err)
	}
	for _, want := range []string{"getComputedStyle", `cssDurationMs("--toast-duration")`, `cssDurationMs("--motion-fade-out")`} {
		if !strings.Contains(string(js), want) {
			t.Errorf("app.js に %q が無い", want)
		}
	}
	for _, banned := range []string{"TOAST_MS", "LEAVE_MS", "3000", "240"} {
		if strings.Contains(string(js), banned) {
			t.Errorf("app.js に時間の直書き %q がある（CSS のトークンを読む）", banned)
		}
	}
}

// app.js はトーストの退出を animationend で消し（reduced-motion は即時）、ボタン押下でリップルを足す。
// リップルの位置は CSSOM（element.style）で決め、マークアップの style 属性は使わない（CSP style-src 'self'）。
func TestAppJS_ToastLeavingAndRipple(t *testing.T) {
	t.Parallel()

	data, err := fs.ReadFile(static.FS(), "js/app.js")
	if err != nil {
		t.Fatalf("js/app.js を読めない（先に task generate を実行する）: %v", err)
	}
	src := string(data)
	for _, want := range []string{"is-leaving", "animationend", "prefers-reduced-motion: reduce", `"pointerdown"`, `"ripple"`, "is-settled", "[data-alert]", `"leave-fade"`, `"htmx:beforeSwap"`, "swapOverride", "dialog[data-modal][open]"} {
		if !strings.Contains(src, want) {
			t.Errorf("app.js に %q が無い", want)
		}
	}
	for _, banned := range []string{"setAttribute(\"style\"", "innerHTML", "cssText"} {
		if strings.Contains(src, banned) {
			t.Errorf("app.js に %q が含まれている（CSP・エスケープの観点で使わない）", banned)
		}
	}
}

// 確認はネイティブの dialog で取る（hx-confirm の window.confirm を使わない）。スクラムはトークン（--scrim）で、ダークでは暗い色に切り替わる。
// 外側クリック・Esc・フェードアウト・showModal への引き上げ・htmx:confirm を app.js が受ける（ガイド O1・O2）。
func TestModal_AppJSAndCSS(t *testing.T) {
	t.Parallel()

	js, err := fs.ReadFile(static.FS(), "js/app.js")
	if err != nil {
		t.Fatalf("js/app.js を読めない（先に task generate を実行する）: %v", err)
	}
	for _, want := range []string{`"htmx:confirm"`, "showModal()", "issueRequest", "data-dismiss", "dataset.dismiss", `"cancel"`, "getBoundingClientRect", "data-modal-result", "data-modal-close", "textContent"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("app.js に %q が無い", want)
		}
	}
	if strings.Contains(string(js), "window.confirm") {
		t.Error("app.js が window.confirm を呼んでいる")
	}

	css, err := fs.ReadFile(static.FS(), "css/app.css")
	if err != nil {
		t.Fatalf("css/app.css を読めない（先に task generate を実行する）: %v", err)
	}
	for _, want := range []string{"dialog.modal::backdrop", "background-color:var(--scrim)", "@keyframes backdrop-in", "@keyframes backdrop-out", "--scrim:"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("app.css に %q が無い", want)
		}
	}
	// モーダルの影はダークで見えるよう、prefers-color-scheme: dark のブロックでライトより濃い値（.4）に切り替える（ガイド C5）。
	dark := string(css)
	start := strings.Index(dark, "prefers-color-scheme:dark")
	if start < 0 {
		t.Fatal("app.css に prefers-color-scheme: dark のブロックが無い")
	}
	dark = dark[start:]
	if end := strings.Index(dark, "@property"); end >= 0 {
		dark = dark[:end]
	}
	if !strings.Contains(dark, "--shadow-modal:0 1px 2px #0006") {
		t.Errorf("ダークのブロックに --shadow-modal（0 1px 2px rgb(0 0 0 / .4)）が無い: %s", dark)
	}
}
