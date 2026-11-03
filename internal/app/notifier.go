package app

import (
	"context"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/ousiassllc/swarm-viewer/internal/app/notice"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

// Notifier は Wails の通知サービスで OS 通知を出す（service.Notifier）。
// クリックされたらウィンドウを前面に出し、該当接続・セッションのタブへ遷移する（FR-NTF）。
type Notifier struct {
	svc *notifications.NotificationService
	ids service.IDGenerator
	nav service.Navigator
	log *slog.Logger
}

// NewNotifier は通知サービス svc を使う Notifier を返し、クリック時の遷移を登録する。
// svc は application.Options.Services に登録されたものと同じインスタンスにする。
func NewNotifier(svc *notifications.NotificationService, ids service.IDGenerator, nav service.Navigator, log *slog.Logger) *Notifier {
	n := &Notifier{svc: svc, ids: ids, nav: nav, log: log}
	svc.OnNotificationResponse(n.onResponse)
	return n
}

// Notify は通知を 1 件出す。
func (n *Notifier) Notify(ctx context.Context, msg service.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m := notice.Build(n.ids.NewID(), msg)
	return n.svc.SendNotification(notifications.NotificationOptions{
		ID:    m.ID,
		Title: m.Title,
		Body:  m.Body,
		Data:  m.Data,
	})
}

// Permitted は OS の通知許可状態を返す。取得できなければ unknown。
func (n *Notifier) Permitted() service.PermissionState {
	ok, err := n.svc.CheckNotificationAuthorization()
	switch {
	case err != nil:
		n.log.Warn("checking the notification permission failed", "err", err)
		return service.PermissionUnknown
	case ok:
		return service.PermissionGranted
	default:
		return service.PermissionDenied
	}
}

// RequestPermission は初回起動時に OS へ通知の許可を求める（macOS はダイアログが出る。Windows は常に許可）。
func (n *Notifier) RequestPermission() {
	ok, err := n.svc.RequestNotificationAuthorization()
	if err != nil {
		n.log.Warn("requesting the notification permission failed", "err", err)
		return
	}
	n.log.Info("notification permission requested", "granted", ok)
}

// onResponse は通知のクリックを受けて遷移する。payload が不正なら何もしない。
func (n *Notifier) onResponse(res notifications.NotificationResult) {
	if res.Error != nil {
		n.log.Warn("notification response failed", "err", res.Error)
		return
	}
	path, ok := notice.Target(res.Response.UserInfo)
	if !ok {
		n.log.Warn("notification response ignored: no target")
		return
	}
	n.nav.Open(path)
}
