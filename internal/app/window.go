package app

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/ousiassllc/swarm-viewer/internal/app/navpath"
)

// Window はメイン WebView ウィンドウ。service.Navigator を実装する（Open → SetURL＋前面化）。
//
// 閉じる操作は終了ではなく非表示にする（トレイ常駐。FR-APP-1）。
// アプリの起動が済む前（ready 前）は、前面化の呼び出し（Show）が未生成のウィンドウを作ってしまうため
// 遷移先（SetURL＝初回に読む URL）だけ設定する。コールド起動のディープリンクがこの経路を通る。
type Window struct {
	win *application.WebviewWindow
	log *slog.Logger

	quitting atomic.Bool

	mu    sync.Mutex
	ready bool
}

// newWindow はメインウィンドウを作る（app.Run の前に呼ぶ）。最初は `/` を読み込む。
func newWindow(app *application.App, log *slog.Logger) *Window {
	w := &Window{log: log}
	w.win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "swarm-viewer",
		Width:     1100,
		Height:    760,
		MinWidth:  640,
		MinHeight: 480,
		URL:       navpath.Home,
	})
	// 閉じる操作を横取りして非表示にする。終了中は通す（フックを使うので既定のウィンドウ破棄より先に走る）。
	w.win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if w.quitting.Load() {
			return
		}
		e.Cancel()
		w.win.Hide()
	})
	return w
}

// markQuitting は以後のウィンドウ close を通す（終了処理中）。
func (w *Window) markQuitting() { w.quitting.Store(true) }

// markReady はアプリの起動完了後に呼ぶ。以後 Open・Show が前面化まで行う。
// ウィンドウ自体は Wails が起動時に表示するので、ここでは Show しない（未生成のウィンドウへの Show は二重生成になりうる）。
func (w *Window) markReady() {
	w.mu.Lock()
	w.ready = true
	w.mu.Unlock()
}

// Open は同一プロセスのハンドラのパスへ遷移し、ウィンドウを前面に出す（service.Navigator）。
// パスが検証に通らなければホームへ遷移する。
func (w *Window) Open(path string) {
	clean, ok := navpath.Clean(path)
	if !ok {
		w.log.Warn("navigation path rejected; opening home")
	}
	w.win.SetURL(clean)
	w.Show()
}

// Show は非表示・最小化のウィンドウを前面に出す（トレイ「開く」・二重起動・通知クリック）。
func (w *Window) Show() {
	w.mu.Lock()
	ready := w.ready
	w.mu.Unlock()
	if !ready {
		return
	}
	w.win.Show()
	w.win.UnMinimise()
	w.win.Focus()
}
