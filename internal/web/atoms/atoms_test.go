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
	webtest.MustContain(t, html, "&lt;i&gt;x&lt;/i&gt;", "text-red-300")
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
	webtest.MustContain(t, html, "h-4 w-4")
}

func TestToastAndAlert(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Toast(atoms.ToastProps{Kind: atoms.ToastSuccess, Message: "保存しました"}))
	webtest.MustContain(t, html, `id="toasts"`, `hx-swap-oob="true"`, "保存しました", "border-emerald-500/60")
	html = webtest.Render(t, atoms.Toast(atoms.ToastProps{Kind: "weird", Message: "m"}))
	webtest.MustContain(t, html, "border-sky-500/60")
	html = webtest.Render(t, atoms.Alert("<b>x</b>"))
	webtest.MustContain(t, html, `role="alert"`, "&lt;b&gt;")
}

func TestButton(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "削除", Variant: atoms.ButtonDanger, Attrs: templ.Attributes{"hx-delete": "/connections/x", "hx-confirm": `"確認" <ok>`}}))
	webtest.MustContain(t, html, `type="button"`, `hx-delete="/connections/x"`, "bg-red-700", "&#34;確認&#34;")
	webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "送信", Type: "submit"})), `type="submit"`)
}

func TestInput(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Input(atoms.InputProps{ID: "i", Name: "n", Required: true, Placeholder: "p"}))
	webtest.MustContain(t, html, `type="text"`, `name="n"`, "required", `autocomplete="off"`)
	webtest.MustNotContain(t, webtest.Render(t, atoms.Input(atoms.InputProps{Name: "n"})), "required")
}
