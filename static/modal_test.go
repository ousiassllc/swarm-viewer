package static_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/static"
)

// 確認はネイティブの dialog で取る（hx-confirm の window.confirm を使わない）。スクラムはトークン（--scrim）で、ダークでは暗い色に切り替わる。
// 外側クリック・Esc・フェードアウト・showModal への引き上げ・htmx:confirm を modal.js が受ける（ガイド O1・O2）。
func TestModal_ModalJSAndCSS(t *testing.T) {
	t.Parallel()

	js, err := fs.ReadFile(static.FS(), "js/modal.js")
	if err != nil {
		t.Fatalf("js/modal.js を読めない（先に task generate を実行する）: %v", err)
	}
	for _, want := range []string{`"htmx:confirm"`, "showModal()", "issueRequest", "data-dismiss", "dataset.dismiss", `"cancel"`, "getBoundingClientRect", "data-modal-result", "data-modal-close", "textContent"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("modal.js に %q が無い", want)
		}
	}
	if strings.Contains(string(js), "window.confirm") {
		t.Error("modal.js が window.confirm を呼んでいる")
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
