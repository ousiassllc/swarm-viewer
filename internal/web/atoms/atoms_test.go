package atoms_test

import (
	"strings"
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

func TestBadge_Marker(t *testing.T) {
	t.Parallel()
	// 状態を示すバッジは、文言の前に文字と同じ色の 6px の点を置く（ガイドのステータスピルの見本）。
	html := webtest.Render(t, atoms.Badge(atoms.BadgeProps{Text: "待機", Tone: format.ToneSuccess, Marker: true}))
	webtest.MustContain(t, html, `<span class="me-1.5 size-1.5 shrink-0 rounded-full bg-current" aria-hidden="true"></span>`, "待機", "text-success")
	if strings.Index(html, "bg-current") > strings.Index(html, "待機") {
		t.Errorf("点は文言の前にある: %s", html)
	}
	// 状態でないバッジ（エージェント名など）には点がない。
	html = webtest.Render(t, atoms.Badge(atoms.BadgeProps{Text: "claude", Tone: format.ToneMuted}))
	webtest.MustNotContain(t, html, "bg-current", "size-1.5")
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
	html := webtest.Render(t, atoms.Spinner(atoms.SpinnerProps{ID: "sp"}))
	// ガイド §4: 20px・線 2px（track --accent-soft・先端 --accent）の 1 種だけ。
	webtest.MustContain(t, html, `id="sp"`, "htmx-indicator", "h-5 w-5", "border-2", "border-accent-soft", "border-t-accent")
	webtest.MustNotContain(t, html, "h-4 w-4", "h-6 w-6")
	html = webtest.Render(t, atoms.Spinner(atoms.SpinnerProps{}))
	webtest.MustNotContain(t, html, "id=")
}

func TestToast(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Toast(atoms.ToastProps{Message: "保存しました"}))
	webtest.MustContain(t, html, `id="toasts"`, `hx-swap-oob="true"`, `role="status"`, "保存しました", "fade-in", "[--fade-rise:8px]")
	// ガイドの見本は高さ 40（--toast-h）・左右 18px（S2）。
	webtest.MustContain(t, html, "h-(--toast-h)", "px-[18px]", "rounded-full")
	webtest.MustNotContain(t, html, "h-10")
	// 幅の上限はガイドの見本の min(92vw, 420px)。接続名など利用者が決める長い文言でも端まで伸びない（O3）。
	webtest.MustContain(t, html, "max-w-[min(92vw,26.25rem)]")
	webtest.MustNotContain(t, html, "calc(100vw-3rem)")
	webtest.MustNotContain(t, html, "px-6")
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
		// 出現はフェードイン（.fade-in）。ポーリングの再描画で繰り返さないために data-alert で app.js が見分ける（M5）。
		webtest.MustContain(t, html, tt.role, tt.color, tt.icon, `<svg`, `aria-hidden="true"`, "&lt;b&gt;", "fade-in", "data-alert")
		// ガイドの見本の形: surface 地・1px の border・左 3px の状態色・--r-md・文字は ink（状態色はアイコンと左の帯だけ）。
		webtest.MustContain(t, html, "bg-surface", "border-border", "border-l-[3px]", "border-l-"+strings.TrimPrefix(tt.color, "text-"), "rounded-md", "text-ink")
		// アイコンと文の間は見本の .al と同じ 12px（S2）。
		webtest.MustContain(t, html, "items-start gap-3")
		webtest.MustNotContain(t, html, "gap-2")
		webtest.MustNotContain(t, html, "/10", "border-danger/60", "rounded-lg")
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

// 見出し（Title）は任意。あれば SemiBold で本文の前に出し、エスケープされる。無ければ <p> を足さない（ガイド §4・O8）。
func TestAlert_Title(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Alert(atoms.AlertProps{Kind: atoms.AlertDanger, Title: "<i>接続できません</i>", Message: "前回取得 11:58"}))
	webtest.MustContain(t, html, `<p class="font-semibold">&lt;i&gt;接続できません&lt;/i&gt;</p>`, "<p>前回取得 11:58</p>")
	webtest.MustNotContain(t, html, "<i>")
	if strings.Index(html, "接続できません") > strings.Index(html, "前回取得") {
		t.Error("見出しは本文の前に出す")
	}
	webtest.MustNotContain(t, webtest.Render(t, atoms.Alert(atoms.AlertProps{Kind: atoms.AlertDanger, Message: "m"})), "font-semibold")
}

// Checkbox はガイドの形（18px・--r-sm・外枠 --mute）で、オンの印は Iconoir の check（インライン SVG）。フォーカスは入力欄と同じ ring（3px）。
func TestCheckbox(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Checkbox(atoms.CheckboxProps{Name: "debug_log", Value: "on", Checked: true}))
	webtest.MustContain(t, html, `type="checkbox"`, `name="debug_log"`, `value="on"`, " checked ", "h-[18px] w-[18px]", "rounded-sm", "border-mute", "appearance-none", "peer-checked:visible", "M5 13.3636L8.03559", "focus-visible:outline-transparent", "focus-visible:ring-[3px]", "focus-visible:ring-(color:--ring)")
	webtest.MustNotContain(t, html, "accent-accent")
	// チェック印は 16px（--icon-sm）で箱の中央に置く。箱いっぱいの 18px にしない（ガイド I2）。
	webtest.MustContain(t, html, "absolute inset-0 m-auto h-4 w-4")
	webtest.MustNotContain(t, html, "inset-0 h-full w-full")
	webtest.MustNotContain(t, webtest.Render(t, atoms.Checkbox(atoms.CheckboxProps{Name: "n", Value: "on"})), " checked ")
}

// EmptyState は破線の枠・--r-lg・padding 縦 48・横 24（画面幅で変えない）・48px の accent-soft の円に載せた 24px のアイコン・文言・子要素（主ボタンなど）を持つ（ガイド S4）。
func TestEmptyState(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.EmptyState(atoms.EmptyStateProps{Message: "<b>なし</b>"}))
	webtest.MustContain(t, html, "border-dashed", "rounded-lg", "px-6", "py-12", "h-12 w-12", "rounded-full", "bg-accent-soft", "h-6 w-6", `aria-hidden="true"`, "&lt;b&gt;")
	webtest.MustNotContain(t, html, "<b>", "md:p-12", "p-6")
}

// ボタンの押下のリップルは円をボタン内に収める（relative overflow-clip。M4。smoke テストが "hidden" を禁じるので overflow-hidden は使わない）。
func TestButton_HostsRipple(t *testing.T) {
	t.Parallel()
	webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "保存"})), "relative", "overflow-clip")
}

// ボタンのホバーは色の遷移（.hover-fade。ガイド M1）を持つ。
func TestButton_HoverFade(t *testing.T) {
	t.Parallel()
	for _, v := range []atoms.ButtonVariant{atoms.ButtonPrimary, atoms.ButtonSecondary, atoms.ButtonDanger} {
		webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "x", Variant: v})), "hover-fade")
	}
}

// 主ボタンのホバーは文字を薄くせず、地を --accent の 85% にする（opacity は使わない。NFR-DSG §11 の承認済みの逸脱・Issue #93）。
func TestButton_PrimaryHover(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "保存", Variant: atoms.ButtonPrimary}))
	webtest.MustContain(t, html, "bg-accent", "text-bone", "hover:bg-accent/85")
	webtest.MustNotContain(t, html, "hover:opacity-")
}

// ConfirmDialog は確認・破壊的操作の中央モーダル（ネイティブ dialog・--r-xl・--shadow-modal）。確定は危険ボタン。
// 文言・見出し・ラベルは app.js が htmx:confirm で textContent に差し込むので、描画時は空・既定値で、インライン script を持たない（O2・NFR-SEC-9）。
func TestConfirmDialog(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.ConfirmDialog())
	webtest.MustContain(t, html, `<dialog id="confirm-dialog"`, "data-modal", "rounded-xl", "border border-border", "[box-shadow:var(--shadow-modal)]", "m-auto",
		"data-confirm-title", "data-confirm-message", "data-confirm-action", "text-danger", "border-danger/45", `data-modal-result="cancel"`, "キャンセル")
	webtest.MustNotContain(t, html, "<script", "onclick", "hx-on", " open")
	// ガイドの見本の構造: ヘッダー（見出し＋閉じる × のアイコンボタン・下に線）・本文・フッター（右寄せ・ボタン間 8px・上に線）。幅は max 420px（26.25rem）。
	webtest.MustContain(t, html, `<header class="flex items-center justify-between gap-2 border-b border-border px-5 py-4">`, `aria-label="閉じる"`, "data-modal-close", "size-8", "M6.75827 17.2426",
		`<footer class="flex items-center justify-end gap-2 border-t border-border px-5 py-3">`, "p-0", "w-[min(26.25rem,")
	webtest.MustNotContain(t, html, "gap-3", "p-6", "28rem")
	// 出現は --motion-slow の .modal-in。アラート・トーストの .fade-in（320ms）は使わない（M5）。
	webtest.MustContain(t, html, "modal-in")
	webtest.MustNotContain(t, html, "fade-in")
}

func TestButton(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "削除", Variant: atoms.ButtonDanger, Attrs: templ.Attributes{"hx-delete": "/connections/x", "hx-confirm": `"確認" <ok>`}}))
	// 危険ボタンは透明地・danger の文字と 45% の枠（ガイドの見本。塗りつぶさない）。
	webtest.MustContain(t, html, `type="button"`, `hx-delete="/connections/x"`, "bg-transparent", "text-danger", "border-danger/45", "hover:bg-danger/10", "&#34;確認&#34;")
	webtest.MustNotContain(t, html, "bg-danger ", "bg-danger/85")
	webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "送信", Type: "submit"})), `type="submit"`)
}

// ボタンは押せることをカーソルで伝え、無効（処理中の hx-disabled-elt を含む）は not-allowed にする（ガイド O7）。
func TestButton_Cursor(t *testing.T) {
	t.Parallel()
	for _, v := range []atoms.ButtonVariant{atoms.ButtonPrimary, atoms.ButtonSecondary, atoms.ButtonDanger} {
		webtest.MustContain(t, webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "x", Variant: v})), "cursor-pointer", "disabled:cursor-not-allowed", "disabled:opacity-40")
	}
}

// 入力欄・select・チェックボックスの無効（disabled）はボタンと同じく not-allowed にし、opacity はガイドの値（欄 .5・チェック .4）にする（ガイド O7・§4 フォーム）。
func TestFormControls_Disabled(t *testing.T) {
	t.Parallel()
	webtest.MustContain(t, atoms.FieldClass, "disabled:cursor-not-allowed", "disabled:opacity-50")
	webtest.MustContain(t, atoms.SelectClass, "disabled:cursor-not-allowed", "disabled:opacity-50")
	webtest.MustContain(t, webtest.Render(t, atoms.Input(atoms.InputProps{Name: "n"})), "disabled:cursor-not-allowed", "disabled:opacity-50")
	webtest.MustContain(t, webtest.Render(t, atoms.Checkbox(atoms.CheckboxProps{Name: "n", Value: "on"})), "disabled:cursor-not-allowed", "disabled:opacity-40")
}

// ボタンの大きさは標準（高さ 40・padding 18・16px）と小（高さ 32・padding 12・14px）（ガイド O7）。ゼロ値は標準。
func TestButton_Sizes(t *testing.T) {
	t.Parallel()
	standard := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "保存"}))
	webtest.MustContain(t, standard, "h-10", "px-[18px]", "text-base")
	webtest.MustNotContain(t, standard, "h-8", "text-sm")
	small := webtest.Render(t, atoms.Button(atoms.ButtonProps{Label: "削除", Size: atoms.ButtonSmall}))
	webtest.MustContain(t, small, "h-8", "px-3", "text-sm")
	webtest.MustNotContain(t, small, "h-10", "px-[18px]")
}

func TestInput(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, atoms.Input(atoms.InputProps{ID: "i", Name: "n", Required: true, Placeholder: "p"}))
	webtest.MustContain(t, html, `type="text"`, `name="n"`, "required", `autocomplete="off"`)
	webtest.MustNotContain(t, webtest.Render(t, atoms.Input(atoms.InputProps{Name: "n"})), "required")
	// 入力欄の文字は本文と同じ 16px（ガイド T3）。
	webtest.MustContain(t, html, "min-h-10", "text-base")
	webtest.MustNotContain(t, html, "text-sm")
}
