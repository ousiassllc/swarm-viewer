// Package app は Wails v3 のデスクトップシェル（ウィンドウ・トレイ・通知・URL スキーム・単一インスタンス）。
// service を呼び、service が定義した interface（Navigator・Notifier）を実装する。web には依存しない。
//
// Wails に依存しない部分（ログのマスク・ディープリンクの抽出・ハンドラ差し替え・組み立て）は
// applog・deeplink・swap・compose など兄弟 package に置き、Linux の CI でもテストできるようにしている。
package app

import (
	"log/slog"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/ousiassllc/swarm-viewer/internal/app/deeplink"
)

// Options は Shell の組み立てに必要な値。
type Options struct {
	// Name はアプリ名（Windows の通知の AppID にも使われる）。
	Name        string
	Description string
	// UniqueID は単一インスタンスの識別子（build/config.yml の productIdentifier と同じ）。
	UniqueID string
	Icon     []byte
	// Handler は WebView が話すアセットハンドラ。TCP ポートは開かない（NFR-SEC-3）。
	Handler http.Handler
	Logger  *slog.Logger
	// Notifications は Notifier が使う Wails の通知サービス（Services に登録する）。
	Notifications *notifications.NotificationService
	// Links は swarm-viewer:// の受信口。URL イベントと 2 つ目の起動の引数をここへ流す。
	Links *deeplink.Dispatcher
	// OnReady は Wails の起動完了時に呼ぶ（通知スケジューラの起動など）。別 goroutine で呼ばれる。
	OnReady func()
	// BeforeQuit は終了の直前に呼ぶ（スケジューラを止める ctx のキャンセル）。何度呼ばれてもよいこと。
	BeforeQuit func()
	// OnShutdown は Wails の終了処理で呼ぶ（SQLite を閉じるなど）。
	OnShutdown func()
}

// Shell は Wails アプリ・ウィンドウ・トレイをまとめる。
type Shell struct {
	app    *application.App
	Window *Window
	opts   Options
	// built は NewShell の完了で閉じる。application.New の中で起動する 2 つ目の起動の受信 goroutine が、
	// Window の代入より前に onSecondInstance を呼ばないよう待たせる。
	built chan struct{}
}

// NewShell は Wails アプリを作り、ウィンドウ・トレイ・URL 受信を登録する（Run はまだ呼ばない）。
//
// 単一インスタンスのロックはここ（application.New）で取る。2 つ目の起動はここで既存インスタンスへ
// 引数を転送して終了するので、SQLite を開く前に呼ぶこと（二重起動で DB を同時に開かない。NFR-AVL-6）。
func NewShell(o Options) *Shell {
	s := &Shell{opts: o, built: make(chan struct{})}
	s.app = application.New(application.Options{
		Name:        o.Name,
		Description: o.Description,
		Icon:        o.Icon,
		Logger:      o.Logger,
		LogLevel:    slog.LevelWarn,
		Services:    []application.Service{application.NewService(o.Notifications)},
		Assets: application.AssetOptions{
			Handler:        o.Handler,
			DisableLogging: true, // リクエストログは router の RequestLog が持つ。URL を二重に出さない
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		// 最後のウィンドウを閉じても終了しない（トレイ常駐）。Mac の既定も false だが明示する。
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               o.UniqueID,
			OnSecondInstanceLaunch: s.onSecondInstance,
		},
		ShouldQuit: func() bool { s.beginQuit(); return true },
		OnShutdown: func() {
			s.beginQuit()
			if o.OnShutdown != nil {
				o.OnShutdown()
			}
		},
	})
	s.Window = newWindow(s.app, o.Logger)
	s.app.RegisterService(application.NewService(&backgroundSync{app: s.app, win: s.Window}))
	s.app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(*application.ApplicationEvent) {
		s.Window.applyTheme(s.app.Env.IsDarkMode())
	})
	newTray(s.app, s.Window, o.Icon, s.Quit)
	registerProtocol(s.app, o.Links)
	s.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		s.Window.markReady()
		if o.OnReady != nil {
			o.OnReady()
		}
	})
	close(s.built)
	return s
}

// Run はアプリを実行する。ウィンドウとトレイが閉じて Quit されるまで返らない。
func (s *Shell) Run() error { return s.app.Run() }

// Quit はスケジューラを止めてからアプリを終了する（トレイ「終了」）。
func (s *Shell) Quit() {
	s.beginQuit()
	s.app.Quit()
}

// beginQuit は終了の準備（ウィンドウ close を通す・スケジューラの停止）。何度呼んでもよい。
func (s *Shell) beginQuit() {
	s.Window.markQuitting()
	if s.opts.BeforeQuit != nil {
		s.opts.BeforeQuit()
	}
}

// onSecondInstance は 2 つ目の起動の引数を受ける。既存ウィンドウを前面に出し、ディープリンクがあれば処理する。
// 引数は信頼できない入力として扱い、ログには出さない（token を含みうる。NFR-SEC-7・NFR-AVL-6）。
func (s *Shell) onSecondInstance(data application.SecondInstanceData) {
	<-s.built
	s.opts.Logger.Info("second instance launch forwarded", "args", len(data.Args))
	s.Window.Show()
	if link, ok := deeplink.Extract(data.Args); ok {
		s.opts.Links.Submit(link)
	}
}
