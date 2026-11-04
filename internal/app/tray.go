package app

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// トレイメニューのラベル（FR-APP-1: 「開く」「終了」）。
const (
	trayLabelOpen = "開く"
	trayLabelQuit = "終了"
	trayTooltip   = "swarm-viewer"
)

// newTray はシステムトレイ（macOS はメニューバー）を作る。メニューは「開く」「終了」のみ。
// 通知オフ・取得失敗の状態はアイコンに出さない（最小構成）。
func newTray(app *application.App, win *Window, icon []byte, quit func()) *application.SystemTray {
	menu := app.NewMenu()
	menu.Add(trayLabelOpen).OnClick(func(*application.Context) { win.Show() })
	menu.AddSeparator()
	menu.Add(trayLabelQuit).OnClick(func(*application.Context) { quit() })

	tray := app.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetTooltip(trayTooltip)
	tray.SetMenu(menu)
	if runtime.GOOS == "windows" {
		// Windows ではトレイアイコンの左クリックでウィンドウを開く（右クリックはメニュー）。
		// macOS はメニューバーのアイコンをクリックするとメニューが開く標準の挙動に任せる。
		tray.OnClick(win.Show)
	}
	return tray
}
