package handler_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func settingsForm(notify, screen, notifyMin, debug string) reqOpt {
	v := url.Values{"screen_poll_seconds": {screen}, "notify_poll_minutes": {notifyMin}}
	if notify != "" {
		v.Set("notifications_enabled", notify)
	}
	if debug != "" {
		v.Set("debug_log", debug)
	}
	return form(v)
}

func TestSettingsPage(t *testing.T) {
	a := newApp(t)
	full := wantStatus(t, a.get("/settings"), http.StatusOK)
	webtest.MustContain(t, full, "<html", `id="settings-form"`, `hx-put="/settings"`, `hx-target-422="#settings-form"`,
		"OS の通知は許可されています")
	frag := wantStatus(t, a.get("/settings", hx()), http.StatusOK)
	webtest.MustNotContain(t, frag, "<html")
}

func TestSettingsUpdate(t *testing.T) {
	a := newApp(t)
	body := wantStatus(t, a.do(http.MethodPut, "/settings", hx(), settingsForm("", "30", "60", "on")), http.StatusOK)
	webtest.MustContain(t, body, `hx-put="/settings"`, "設定を保存しました", `id="toasts"`, `hx-swap-oob="true"`)
	webtest.MustNotContain(t, body, "<html")

	st, err := a.env.Settings.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if st.NotificationsEnabled || st.ScreenPollSeconds != 30 || st.NotifyPollMinutes != 60 || !st.DebugLog {
		t.Fatalf("settings = %+v", st)
	}
	// 保存した値が次の描画に反映される。
	webtest.MustContain(t, wantStatus(t, a.get("/settings", hx()), http.StatusOK), `<option value="30" selected`)
}

func TestSettingsUpdateNotifiesSubscribers(t *testing.T) {
	a := newApp(t)
	ch := a.env.Settings.Subscribe()
	wantStatus(t, a.do(http.MethodPut, "/settings", hx(), settingsForm("on", "5", "5", "")), http.StatusOK)
	if got := servicetest.Recv(t, ch, "settings update"); got.ScreenPollSeconds != 5 {
		t.Fatalf("settings = %+v", got)
	}
}

func TestSettingsUpdateInvalidIs422(t *testing.T) {
	cases := map[string]reqOpt{
		"許容外の画面更新間隔": settingsForm("on", "7", "10", ""),
		"許容外の通知間隔":   settingsForm("on", "10", "11", ""),
		"数値でない":      settingsForm("on", "abc", "10", ""),
		"欠落":         form(url.Values{}),
	}
	for name, opt := range cases {
		t.Run(name, func(t *testing.T) {
			a := newApp(t)
			body := wantStatus(t, a.do(http.MethodPut, "/settings", hx(), opt), http.StatusUnprocessableEntity)
			webtest.MustContain(t, body, `role="alert"`, "設定の値が正しくありません", `hx-put="/settings"`)
			webtest.MustNotContain(t, body, "<html", "設定を保存しました")
			st, err := a.env.Settings.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if st.ScreenPollSeconds != 10 || st.NotifyPollMinutes != 10 {
				t.Fatalf("不正な入力で保存された: %+v", st)
			}
		})
	}
}
