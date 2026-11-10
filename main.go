// swarm-viewer は swarm-console の状態を確認する read-only のデスクトップビューア（Wails v3 + HALT）。
// このファイルは依存の組み立てだけを行う（docs/components/overview.md §8）。
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/ousiassllc/swarm-viewer/internal/app"
	"github.com/ousiassllc/swarm-viewer/internal/app/applog"
	"github.com/ousiassllc/swarm-viewer/internal/app/compose"
	"github.com/ousiassllc/swarm-viewer/internal/app/deeplink"
	"github.com/ousiassllc/swarm-viewer/internal/app/idgen"
	"github.com/ousiassllc/swarm-viewer/internal/app/swap"
	"github.com/ousiassllc/swarm-viewer/internal/buildinfo"
	"github.com/ousiassllc/swarm-viewer/internal/repository/secret"
	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/static"
)

const (
	appName = "swarm-viewer"
	// appID は単一インスタンスの識別子。build/config.yml の productIdentifier と同じにする。
	appID          = "com.ousiassllc.swarm-viewer"
	appDescription = "swarm-console の read-only ビューア"
)

//go:embed build/appicon.png
var appIcon []byte

func main() {
	// `wails3 dev` は Vite 用の FRONTEND_DEVSERVER_URL を設定する。この app は別プロセスのフロントエンドを持たず
	// （画面は in-process の Gin が返す）、残すと Wails が存在しない dev server を待って起動に失敗する。
	_ = os.Unsetenv("FRONTEND_DEVSERVER_URL")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "swarm-viewer:", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. ログ（slog + lumberjack）。ログファイルを開けなくても起動は止めない。
	logging := openLogging()
	defer func() { _ = logging.Close() }()
	log := logging.Logger

	dbPath, err := store.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolving the config directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		handler  = &swap.Handler{}
		links    = &deeplink.Dispatcher{}
		ids      = idgen.ULID{}
		notifSvc = notifications.New()
		rt       *compose.Runtime
		notifier *app.Notifier
	)

	// 2. Wails アプリ。単一インスタンスのロックはここで取る（2 つ目の起動は転送して終了する）ので、SQLite を開く前に作る。
	shell := app.NewShell(app.Options{
		Name:          appName,
		Description:   appDescription,
		UniqueID:      appID,
		Icon:          appIcon,
		Handler:       handler,
		Logger:        log,
		Notifications: notifSvc,
		Links:         links,
		OnReady: func() {
			// 通知スケジューラを先に起動する。macOS の許可ダイアログは応答まで数分ブロックしうるため、
			// 待ってからだと許可の有無に関わらず通知用ポーリングが止まる（OnReady は別 goroutine）。
			rt.Ready()
			notifier.RequestPermission()
		},
		BeforeQuit: cancel,
		OnShutdown: func() { cancel(); rt.Close() },
	})

	// 3. repository・service・router。SQLite が壊れている・新しすぎるときは recovery モードで続行する。
	notifier = app.NewNotifier(notifSvc, ids, shell.Window, log)
	rt = compose.New(compose.Config{
		DBPath:   dbPath,
		Logging:  logging,
		Notifier: notifier,
		Clock:    service.SystemClock{},
		IDs:      ids,
		Secrets:  secret.NewStore(),
		Static:   static.FS(),
		Handler:  handler,
		Version:  buildinfo.Version,
		Quit:     shell.Quit, // 更新を適用できたらスケジューラを止めて終了する
	})
	if err := rt.Start(ctx); err != nil {
		return fmt.Errorf("starting: %w", err)
	}

	// 4. ディープリンク（URL イベント・2 つ目の起動の引数）は、下書きを作って確認画面へ遷移する。
	links.Bind(deeplink.NewHandler(rt, shell.Window, log).Handle)

	// 5. 実行。終了時に ctx をキャンセルし、通知スケジューラを止めて SQLite を閉じる（OnShutdown）。
	if err := shell.Run(); err != nil {
		return fmt.Errorf("running: %w", err)
	}
	return nil
}

// openLogging はユーザー設定領域のログへ書く。開けなければ stderr へ書く（NFR-LOG-1 の例外。起動は止めない）。
func openLogging() *applog.Logging {
	dir, err := applog.DefaultDir()
	if err == nil {
		var l *applog.Logging
		if l, err = applog.Open(dir); err == nil {
			return l
		}
	}
	fmt.Fprintln(os.Stderr, "swarm-viewer: log file unavailable:", err)
	return applog.New(os.Stderr)
}
