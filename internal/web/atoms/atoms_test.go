package atoms_test

import (
	"testing"

	"github.com/a-h/templ"

	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func TestBadge(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Badge(atoms.BadgeProps{Text: "<i>x</i>", Tone: format.ToneError}))
	webtest.MustContain(t, html, "&lt;i&gt;x&lt;/i&gt;", "text-danger")
	webtest.MustNotContain(t, html, "<i>")
}

func TestProgressBar_ValueClampedAndNoStyleAttr(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in   float64
		want string
	}{{-3, `value="0"`}, {42.5, `value="42.5"`}, {250, `value="100"`}} {
		html := webtest.Render(t, atoms.ProgressBar(atoms.ProgressBarProps{Percent: tt.in, Tone: format.ToneAccent, Label: "L"}))
		webtest.MustContain(t, html, tt.want, `max="100"`, `aria-label="L"`, "<progress")
		webtest.MustNotContain(t, html, "style=")
	}
}

func TestSpinner(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Spinner(atoms.SpinnerProps{ID: "sp", Size: atoms.SpinnerLarge}))
	webtest.MustContain(t, html, `id="sp"`, "htmx-indicator", "h-6 w-6")
	html = webtest.Render(t, atoms.Spinner(atoms.SpinnerProps{}))
	webtest.MustNotContain(t, html, "id=")
	webtest.MustContain(t, html, "h-5 w-5")
}

func TestToast(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Toast(atoms.ToastProps{Message: "保存しました"}))
	webtest.MustContain(t, html, `id="toasts"`, `hx-swap-oob="true"`, `role="status"`, "保存しました")
}

// Alert は種類ごとにアイコン（装飾の svg）・文言・色を持ち、本文はエスケープされる（ガイド C4）。
func TestAlert_KindsHaveIconAndText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind  atoms.AlertKind
		role  string
		color string
		icon  string
	}{
		{atoms.AlertDanger, `role="alert"`, "text-danger", "M12 7V13"},
		{atoms.AlertWarning, `role="alert"`, "text-warning", "M20.0429 21H3.95705"},
		{atoms.AlertInfo, `role="status"`, "text-info", "M12 11.5V16.5"},
		{atoms.AlertSuccess, `role="status"`, "text-success", "M7 12.5L10 15.5L17 8.5"},
		{atoms.AlertKind("???"), `role="alert"`, "text-danger", "M12 7V13"},
	}
	for _, tt := range tests {
		html := webtest.Render(t, atoms.Alert(atoms.AlertProps{Kind: tt.kind, Message: "<b>x</b>"}))
		webtest.MustContain(t, html, tt.role, tt.color, tt.icon, `<svg`, `aria-hidden="true"`, "&lt;b&gt;")
		webtest.MustNotContain(t, html, "<b>")
	}
}

// Message が空なら <p> を出さない（子要素だけを載せる使い方）。追加の属性は要素に付く。
func TestAlert_AttrsAndEmptyMessage(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Alert(atoms.AlertProps{Kind: atoms.AlertWarning, Attrs: templ.Attributes{"data-warning-code": "w1"}}))
	webtest.MustContain(t, html, `data-warning-code="w1"`)
	webtest.MustNotContain(t, html, "<p>")
}

func TestButton(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "削除", Variant: atoms.ButtonDanger, Attrs: templ.Attributes{"hx-delete": "/connections/x", "hx-confirm": `"確認" <ok>`}}))
	webtest.MustContain(t, html, `type="button"`, `hx-delete="/connections/x"`, "bg-danger", "&#34;確認&#34;")
	webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "送信", Type: "submit"})), `type="submit"`)
}

func TestInput(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Input(atoms.InputProps{ID: "i", Name: "n", Required: true, Placeholder: "p"}))
	webtest.MustContain(t, html, `type="text"`, `name="n"`, "required", `autocomplete="off"`)
	webtest.MustNotContain(t, webtest.Render(t, atoms.Input(atoms.InputProps{Name: "n"})), "required")
}
