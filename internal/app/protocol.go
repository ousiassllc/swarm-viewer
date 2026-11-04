package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/ousiassllc/swarm-viewer/internal/app/deeplink"
)

// registerProtocol は URL スキーム経由の起動・受信（Windows のコールド起動引数、macOS の URL イベント）を
// links に流す。起動中の 2 つ目の起動の引数は Shell.onSecondInstance が受ける。
//
// links が処理関数を持つまでのリンクは Dispatcher が溜める。URL の中身（token を含む）はログに出さない。
func registerProtocol(app *application.App, links *deeplink.Dispatcher) {
	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		if raw := e.Context().URL(); raw != "" {
			links.Submit(raw)
		}
	})
}
