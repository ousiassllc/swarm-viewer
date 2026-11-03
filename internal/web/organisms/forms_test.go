package organisms_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
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
		`hx-confirm=`, `hx-get="/connections/01A"`,
		"devbox", "100.64.0.5", "100.64.0.6", "表示中",
		`hx-target="#connection-list"`,
	)
	webtest.MustNotContain(t, html, "48120", "/report", "<b>", "http://")
	if strings.Count(html, "hx-confirm") != 2 {
		t.Error("削除ごとに確認がある")
	}
}

func TestConnectionList_Empty(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.ConnectionList(organisms.ConnectionListProps{}))
	webtest.MustContain(t, html, `id="connection-list"`, "保存済みの接続はありません")
	webtest.MustNotContain(t, html, "hx-delete")
}

func TestPairingForm(t *testing.T) {
	t.Parallel()
	html := webtest.Render(t, organisms.PairingForm(organisms.PairingFormProps{}))
	webtest.MustContain(t, html, `hx-post="/pairing"`, `hx-target-422="#pairing-form"`, `name="pairing"`, "<textarea", `type="submit"`)
	webtest.MustNotContain(t, html, `role="alert"`)

	// 検証失敗時も、貼り付けられた値は戻さない（textarea は空のまま）。
	html = webtest.Render(t, organisms.PairingForm(organisms.PairingFormProps{Error: "接続文字列の形式が正しくありません"}))
	webtest.MustContain(t, html, `role="alert"`, "接続文字列の形式が正しくありません", "></textarea>")
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
	if strings.Count(html, "checked") != 1 {
		t.Errorf("通知だけがオン（デバッグログはオフ）: %d", strings.Count(html, "checked"))
	}
	webtest.MustNotContain(t, html, `<option value="15"`, "7 秒")
}

func TestSettingsForm_DebugOnNotifyOffAndPermissions(t *testing.T) {
	t.Parallel()
	s := domain.Settings{NotificationsEnabled: false, ScreenPollSeconds: 10, NotifyPollMinutes: 10, DebugLog: true}
	html := webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: s, Permission: organisms.NotifyPermissionGranted, Error: "設定を保存できません"}))
	if strings.Count(html, "checked") != 1 {
		t.Errorf("デバッグログだけがオン: %d", strings.Count(html, "checked"))
	}
	webtest.MustContain(t, html, "OS の通知は許可されています", "設定を保存できません", `role="alert"`)
	webtest.MustContain(t, webtest.Render(t, organisms.SettingsForm(organisms.SettingsFormProps{Settings: domain.DefaultSettings()})), "確認できません")
}
