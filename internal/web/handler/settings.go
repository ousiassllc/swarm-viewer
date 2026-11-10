package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
)

const (
	settingsInvalidMessage = "設定の値が正しくありません。選択肢の中から選んでください"
	checkboxOn             = "on"
)

func (h *Handler) registerSettings(r gin.IRoutes) {
	r.GET("/settings", h.settingsPage)
	r.PUT("/settings", h.settingsUpdate)
}

func (h *Handler) permission() organisms.NotifyPermission {
	if h.notifier == nil {
		return organisms.NotifyPermissionUnknown
	}
	switch h.notifier.Permitted() {
	case service.PermissionGranted:
		return organisms.NotifyPermissionGranted
	case service.PermissionDenied:
		return organisms.NotifyPermissionDenied
	default:
		return organisms.NotifyPermissionUnknown
	}
}

func (h *Handler) settingsForm(st domain.Settings, errMsg string) organisms.SettingsFormProps {
	return organisms.SettingsFormProps{Settings: st, Permission: h.permission(), Error: errMsg}
}

// settingsPage は `GET /settings`。通知の OS 許可状態も表示する。
func (h *Handler) settingsPage(c *gin.Context) {
	st, err := h.settings.Get(c.Request.Context())
	if err != nil {
		h.internal(c, "load settings", err)
		return
	}
	p := pages.SettingsProps{Form: h.settingsForm(st, "")}
	h.view(c, "設定", func(s layout.BaseProps) templ.Component { return pages.SettingsPage(s, p) }, pages.SettingsContent(p))
}

// settingsUpdate は `PUT /settings`。保存に成功したらフォーム＋トースト OOB、許容外は 422 でフォームを再描画する。
func (h *Handler) settingsUpdate(c *gin.Context) {
	ctx := c.Request.Context()
	cur, err := h.settings.Get(ctx)
	if err != nil {
		h.internal(c, "load settings", err)
		return
	}
	in, ok := parseSettingsInput(c)
	if !ok {
		h.render(c, http.StatusUnprocessableEntity, organisms.SettingsForm(h.settingsForm(echoSettings(cur, c), settingsInvalidMessage)))
		return
	}
	st, err := h.settings.Update(ctx, in)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			h.render(c, http.StatusUnprocessableEntity, organisms.SettingsForm(h.settingsForm(cur, settingsInvalidMessage)))
			return
		}
		h.internal(c, "update settings", err)
		return
	}
	h.render(c, http.StatusOK, templ.Join(
		organisms.SettingsForm(h.settingsForm(st, "")),
		atoms.Toast(atoms.ToastProps{Message: "設定を保存しました"}),
	))
}

// parseSettingsInput はフォーム値を読む。数値でない値は false（許容値の検証は service が行う）。
func parseSettingsInput(c *gin.Context) (service.SettingsInput, bool) {
	screen, err1 := strconv.Atoi(c.PostForm(organisms.FieldScreenPollSeconds))
	notify, err2 := strconv.Atoi(c.PostForm(organisms.FieldNotifyPollMinutes))
	if err1 != nil || err2 != nil {
		return service.SettingsInput{}, false
	}
	return service.SettingsInput{
		NotificationsEnabled: c.PostForm(organisms.FieldNotificationsEnabled) == checkboxOn,
		ScreenPollSeconds:    screen,
		NotifyPollMinutes:    notify,
		DebugLog:             c.PostForm(organisms.FieldDebugLog) == checkboxOn,
		AutoUpdateCheck:      c.PostForm(organisms.FieldAutoUpdateCheck) == checkboxOn,
	}, true
}

// echoSettings は 422 で再描画するフォームの値。保存済みの値を基に、チェックボックスだけ入力を反映する。
func echoSettings(cur domain.Settings, c *gin.Context) domain.Settings {
	cur.NotificationsEnabled = c.PostForm(organisms.FieldNotificationsEnabled) == checkboxOn
	cur.DebugLog = c.PostForm(organisms.FieldDebugLog) == checkboxOn
	cur.AutoUpdateCheck = c.PostForm(organisms.FieldAutoUpdateCheck) == checkboxOn
	return cur
}
