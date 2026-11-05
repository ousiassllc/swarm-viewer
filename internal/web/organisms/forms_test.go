package organisms_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func TestConnectionList(t *testing.T) {
	t.Parallel()
	conns := []domain.Connection{
		{ID: "01A", Name: "devbox", URL: "http://100.64.0.5:48120/report"},
		{ID: "01B", Name: `<b>evil</b>`, URL: "http://100.64.0.6:48120/report"},
	}
	html := webtest.Render(t, organisms.ConnectionList(organisms.ConnectionListProps{Connections: conns, CurrentID: "01A"}))
	webtest.MustContain(t, html,
		`id="connection-list"`,
		`hx-delete="/connections/01A"`, `hx-delete="/connections/01B"`,
		`hx-confirm=`, `hx-get="/connections/01B"`,
		"devbox", "100.64.0.5", "100.64.0.6", "表示中",
		`hx-target="#connection-list"`,
	)
	webtest.MustNotContain(t, html, "48120", "/report", "<b>", "http://", `hx-get="/connections/01A"`)
	if strings.Count(html, `hx-get=`) != 1 {
		t.Error("切替は表示中以外の行だけにある")
	}
	if strings.Count(html, "hx-confirm") != 2 {
		t.Error("削除ごとに確認がある")
	}
	// 確認は中央モーダル（#confirm-dialog）で取る。見出しとボタンのラベルは要素の data-confirm-* から差し込まれる（ガイド O2）。
	if strings.Count(html, `data-confirm-title="接続を削除"`) != 2 || strings.Count(html, `data-confirm-label="削除"`) != 2 {
		t.Error("削除ごとに確認ダイアログの見出しとラベルがある")
	}
	// 行のボタン（とバッジ）は 8px 間隔のまとまりに入れる。名前ブロックとの間は 12px のまま（ガイド S2）。
	if strings.Count(html, `<li class="flex items-center gap-3 px-4 py-3">`) != 2 || strings.Count(html, `<div class="flex items-center gap-2">`) != 2 {
		t.Errorf("行は gap-3、ボタンの列は gap-2: %s", html)
	}
	// 接続名と host の 2 行は、タイトル→キャプションの 2px（--list-title-caption-gap）を空ける（ガイド S2）。
	if strings.Count(html, "flex-col gap-(--list-title-caption-gap)") != 2 {
		t.Errorf("接続名と host の間に --list-title-caption-gap がある: %s", html)
	}
}

func TestConnectionList_Empty(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.ConnectionList(organisms.ConnectionListProps{}))
	webtest.MustContain(t, html, `id="connection-list"`, "保存済みの接続はありません")
	webtest.MustNotContain(t, html, "hx-delete")
	// 空状態は破線の枠＋アイコン＋文言＋主ボタン「接続を追加」（ガイド S4）。
	webtest.MustContain(t, html, "border-dashed", "<svg", `hx-get="/pairing"`, "接続を追加", "bg-accent")
}

// checkedCount はチェック済みの input の数（class の `checked:` 修飾子は数えない）。
func checkedCount(html string) int {
	return len(regexp.MustCompile(`\schecked[\s>/]`).FindAllString(html, -1))
}

func TestPairingForm(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.PairingForm(organisms.PairingFormProps{}))
	webtest.MustContain(t, html, `hx-post="/pairing"`, `hx-target-422="#pairing-form"`, `name="pairing"`, "<textarea", `type="submit"`)
	webtest.MustNotContain(t, html, `role="alert"`, "border-danger!", "--ring-danger")

	// 検証失敗時も、貼り付けられた値は戻さない（textarea は空のまま）。
	html = webtest.Render(t, organisms.PairingForm(organisms.PairingFormProps{Error: "接続文字列の形式が正しくありません"}))
	webtest.MustContain(t, html, `role="alert"`, "接続文字列の形式が正しくありません", "></textarea>")
	// 検証失敗のとき入力欄の枠は --danger、フォーカスのリングは --ring-danger（ガイド O7 の .fld.bad）。
	webtest.MustContain(t, html, "border-danger!", "focus-visible:border-danger!", "[--ring:var(--ring-danger)]")
	// ラベル→入力 8px・入力→エラー 4px・項目間 16px（ガイド S2）。
	webtest.MustContain(t, html, `class="flex flex-col gap-4"`, `<div class="flex flex-col gap-2"><label for="pairing-input"`, `<div class="flex flex-col gap-1"><textarea`)
	// 接続文字列の欄は本文と同じ 16px の等幅（ガイド T3・T5）。
	webtest.MustContain(t, html, "font-mono", "text-base")
	webtest.MustNotContain(t, html, "font-mono text-sm")
}

func TestSettingsForm(t *testing.T) {
	t.Parallel()
	s := domain.Settings{NotificationsEnabled: true, ScreenPollSeconds: 30, NotifyPollMinutes: 5, DebugLog: false}
	html := webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: s, Permission: organisms.NotifyPermissionDenied}))
	webtest.MustContain(t, html,
		`hx-put="/settings"`, `hx-target="#settings-form"`, `hx-target-422="#settings-form"`,
		`name="screen_poll_seconds"`, `name="notify_poll_minutes"`, `name="notifications_enabled"`, `name="debug_log"`,
		`<option value="5">5 秒</option>`, `<option value="10">10 秒</option>`, `<option value="30" selected>30 秒</option>`, `<option value="60">60 秒</option>`,
		`<option value="5" selected>5 分</option>`, `<option value="10">10 分</option>`, `<option value="30">30 分</option>`, `<option value="60">60 分</option>`,
		"OS が通知を許可していません",
	)
	// select の文字は本文と同じ 16px（ガイド T3）。
	if strings.Count(html, "w-40 pl-3") != 2 || strings.Contains(html, "w-40 pl-3 text-sm") || !strings.Contains(html, "text-base") {
		t.Errorf("select は text-base（text-sm を足さない）: %s", html)
	}
	// select は OS 既定の矢印を消し、mute のシェブロン（linear-gradient 2 本）を右端に出す（ガイド §4 フォーム・O8）。
	if n := strings.Count(html, "appearance-none pr-9"); n != 2 {
		t.Errorf("select 2 つに appearance-none と pr-9 が要る: %d", n)
	}
	webtest.MustContain(t, html, "linear-gradient(45deg,transparent_50%,var(--mute)_50%)", "linear-gradient(135deg,var(--mute)_50%,transparent_50%)")
	if n := checkedCount(html); n != 1 {
		t.Errorf("通知だけがオン（デバッグログはオフ）: %d", n)
	}
	// ラベル→入力は 8px（ガイド S2）。チェックボックスは 18px・--r-sm・外枠 --mute（ガイド S1・§4）。
	// チェックボックス→文言は 10px（--list-marker-gap。ガイド S2 の .chk）。
	webtest.MustContain(t, html, `<div class="flex flex-col gap-2"><label for="screen-poll"`, "rounded-sm border border-mute", "appearance-none")
	if n := strings.Count(html, `<label class="`+atoms.CheckboxRowClass+`">`); n != 2 {
		t.Errorf("チェックボックスの行 2 つは CheckboxRowClass（gap-(--list-marker-gap)）: %d", n)
	}
	webtest.MustNotContain(t, html, "accent-accent")
	// 「保存」とスピナーのボタン列は 8px（ガイド S2）。
	// （エラーのアラートは アイコンと文の間が 12px の gap-3 なので、ボタン列の gap だけを見る。）
	webtest.MustContain(t, html, `<div class="flex items-center gap-2"><button`)
	webtest.MustNotContain(t, html, `<div class="flex items-center gap-3"`)
	webtest.MustNotContain(t, html, `<option value="15"`, "7 秒")
}

func TestSettingsForm_DebugOnNotifyOffAndPermissions(t *testing.T) {
	t.Parallel()
	s := domain.Settings{NotificationsEnabled: false, ScreenPollSeconds: 10, NotifyPollMinutes: 10, DebugLog: true}
	html := webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: s, Permission: organisms.NotifyPermissionGranted, Error: "設定を保存できません"}))
	if n := checkedCount(html); n != 1 {
		t.Errorf("デバッグログだけがオン: %d", n)
	}
	webtest.MustContain(t, html, "設定を保存できません", `role="alert"`)
	// Granted・Unknown は許可状態の <p> を出さない（Denied のときだけ案内を出す）。
	const deniedText = "OS が通知を許可していません。OS の設定で許可してください"
	webtest.MustNotContain(t, html, deniedText, "OS の通知")
	unknown := webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: domain.DefaultSettings()}))
	webtest.MustNotContain(t, unknown, deniedText, "OS の通知")
	denied := webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: domain.DefaultSettings(), Permission: organisms.NotifyPermissionDenied}))
	webtest.MustContain(t, denied, deniedText)
	// 通知の拒否は注意のアラート（アイコン＋文言＋状態色。ガイド C4）で、プレーンな <p> ではない。
	webtest.MustContain(t, denied, `role="alert"`, "data-alert", warningIcon)
	webtest.MustNotContain(t, denied, `<p class="text-sm text-ink">OS が通知を許可していません`)
}
