// Package compose は SQLite・repository・service・router を組み立て、通常モードと recovery モードを切り替える
// （アーキテクチャ設計 §5.1、コンポーネント設計 §8）。Wails に依存しない。
//
// SQLite が壊れている・新しすぎるときは起動を止めず、初期化導線だけの recovery ハンドラで始める
// （NFR-AVL-5・NFR-CMP-4）。初期化（POST /recovery/reset）に成功したら通常モードへ切り替え、
// 通知スケジューラを起動する。
package compose

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/app/applog"
	"github.com/ousiassllc/swarm-viewer/internal/app/swap"
	"github.com/ousiassllc/swarm-viewer/internal/repository/report"
	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/router"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
)

// ErrUnavailable は通常モードでないため（recovery モード）、機能を使えないときのエラー。
var ErrUnavailable = errors.New("compose: service unavailable in recovery mode")

// closeTimeout はバックグラウンド処理の終了を待つ上限。
const closeTimeout = 5 * time.Second

// Secrets は OS 秘密領域（secret.Store が満たす）。report クライアントの token 取得にも使う。
type Secrets interface {
	service.SecretStore
}

// Config は組み立てに必要な外部要素。
type Config struct {
	// DBPath は SQLite のパス。
	DBPath string
	// Logging はログ設定。デバッグログの切替に Level を使う。
	Logging *applog.Logging
	// Notifier は OS 通知（app が実装）。
	Notifier service.Notifier
	Clock    service.Clock
	IDs      service.IDGenerator
	Secrets  Secrets
	// Report が nil なら repository/report の実クライアントを使う。
	Report service.ReportClient
	// Panes（ペイン詳細・操作）が nil なら repository/report の実クライアントを使う。
	Panes paneservice.Client
	// Static は埋め込みの静的アセット（static.FS()）。
	Static fs.FS
	// Handler は Wails のアセットハンドラ。モードの切り替えでここへ差し替える。
	Handler *swap.Handler
}

// Runtime は起動中のモードとバックグラウンド処理を持つ。
type Runtime struct {
	cfg Config
	log *slog.Logger

	mu      sync.Mutex
	workCtx context.Context //nolint:containedctx // 実行時に再起動するワーカー（reset 後の通知スケジューラ）の寿命を表す
	cancel  context.CancelFunc
	store   *store.Store
	pairing *service.PairingService
	svcs    services
	ready   bool
	started bool
	workers sync.WaitGroup
}

// New は Runtime を作る。Start を呼ぶまで cfg.Handler は 503 を返す。
func New(cfg Config) *Runtime {
	return &Runtime{cfg: cfg, log: cfg.Logging.Logger}
}

// Start は SQLite を開いて通常モードで始める。開けなければ recovery モードで始める（エラーにしない）。
// ctx のキャンセルで通知スケジューラが止まる。ハンドラの構築に失敗したときだけエラーを返す。
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workCtx, r.cancel = context.WithCancel(ctx)

	err := r.enterNormalLocked()
	if err == nil {
		return nil
	}
	reason, ok := Classify(err)
	if !ok {
		return err
	}
	r.log.Warn("starting in recovery mode", "reason", reason, "err", err)
	h, rerr := router.NewRecovery(router.RecoveryDeps{
		Logger: r.log,
		Reason: reason,
		Reset:  r.reset,
		Static: r.cfg.Static,
	})
	if rerr != nil {
		return fmt.Errorf("compose: building the recovery handler: %w", rerr)
	}
	r.cfg.Handler.Set(h)
	return nil
}

// Parse はディープリンクから下書きを作る（deeplink.Parser）。recovery モードでは ErrUnavailable。
func (r *Runtime) Parse(raw string) (service.DraftID, error) {
	r.mu.Lock()
	p := r.pairing
	r.mu.Unlock()
	if p == nil {
		return "", ErrUnavailable
	}
	return p.Parse(raw)
}

// Close はバックグラウンド処理を止めて SQLite を閉じる。ctx のキャンセル後に呼ぶ。何度呼んでもよい。
func (r *Runtime) Close() {
	r.mu.Lock()
	cancel, st := r.cancel, r.store
	r.store = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() { r.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(closeTimeout):
		r.log.Warn("background workers did not stop in time")
	}
	if st != nil {
		if err := st.Close(); err != nil {
			r.log.Warn("closing the database failed", "err", err)
		}
	}
}

// reset は recovery モードの初期化。DB を退避（削除はしない）して開き直し、通常モードへ切り替える。
func (r *Runtime) reset(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workCtx.Err() != nil {
		return r.workCtx.Err() // 終了処理中。DB を触らない
	}
	if r.store != nil {
		// 二重送信などで、すでに通常モードへ切り替わっている。動いている DB は退避しない。
		return nil
	}
	backup, err := store.MoveAside(r.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("compose: moving the database aside: %w", err)
	}
	r.log.Warn("database moved aside", "backed_up", backup != "")
	return r.enterNormalLocked()
}

// enterNormalLocked は SQLite を開き、service・router を組み立てて通常モードへ切り替える（r.mu を保持して呼ぶ）。
func (r *Runtime) enterNormalLocked() error {
	st, err := store.Open(r.cfg.DBPath)
	if err != nil {
		return err
	}
	h, svcs, err := r.build(st)
	if err != nil {
		if cerr := st.Close(); cerr != nil {
			r.log.Warn("closing the database failed", "err", cerr)
		}
		return fmt.Errorf("compose: building the handler: %w", err)
	}
	r.store, r.pairing, r.svcs = st, svcs.pairing, svcs
	r.cfg.Handler.Set(h)
	r.followLogLevelLocked()
	r.startWorkersLocked()
	return nil
}

// Ready は OS 通知サービスなど Wails 側の準備が済んだことを伝える。通常モードなら通知スケジューラを起動する。
// 準備前に走り出すと、起動直後の通知が未初期化の通知サービスに当たるため、Start とは分けている。
func (r *Runtime) Ready() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = true
	r.startWorkersLocked()
}

type services struct {
	settings     *service.SettingsService
	notification *service.NotificationService
	pairing      *service.PairingService
}

// build は documented order（Settings → Connection → Notification → Snapshot → Pairing）で service を作り、
// 通常モードのハンドラを返す。
func (r *Runtime) build(st *store.Store) (http.Handler, services, error) {
	c := r.cfg
	real := report.NewClient(c.Secrets, r.log)
	var rep service.ReportClient = real
	if c.Report != nil {
		rep = c.Report
	}
	var paneClient paneservice.Client = real
	if c.Panes != nil {
		paneClient = c.Panes
	}
	settings := service.NewSettingsService(st)
	conns := service.NewConnectionService(rep, st, c.Secrets, c.Clock, r.log)
	notif := service.NewNotificationService(rep, st, c.Notifier, c.Clock, settings, conns, r.log)
	snaps := service.NewSnapshotService(rep, st, notif, conns, c.Clock, r.log)
	pairing := service.NewPairingService(rep, st, c.Secrets, c.IDs, c.Clock, conns, r.log)
	pairing.SetPollTrigger(notif)
	panes := paneservice.New(paneClient, conns, r.log)

	h, err := router.New(router.Deps{
		Logger:      r.log,
		Settings:    settings,
		Connections: conns,
		Snapshots:   snaps,
		Pairing:     pairing,
		Panes:       panes,
		Notifier:    c.Notifier,
		Clock:       c.Clock,
		Static:      c.Static,
	})
	return h, services{settings: settings, notification: notif, pairing: pairing}, err
}

// followLogLevelLocked は設定のデバッグログ切替にログレベルを追随させる（r.mu を保持して呼ぶ）。
func (r *Runtime) followLogLevelLocked() {
	s, ctx := r.svcs.settings, r.workCtx
	if cur, err := s.Get(ctx); err == nil {
		r.cfg.Logging.Level.Set(applog.LevelFor(cur.DebugLog))
	} else {
		r.log.Warn("loading settings for the log level failed", "err", err)
	}
	changes := s.Subscribe()
	r.workers.Add(1)
	go func() {
		defer r.workers.Done()
		r.cfg.Logging.Follow(ctx, changes)
	}()
}

// startWorkersLocked は通知スケジューラを、通常モードかつ準備済みのときに 1 度だけ起動する（r.mu を保持して呼ぶ）。
func (r *Runtime) startWorkersLocked() {
	if !r.ready || r.store == nil || r.started {
		return
	}
	r.started = true
	n, ctx := r.svcs.notification, r.workCtx
	r.workers.Add(1)
	go func() {
		defer r.workers.Done()
		n.Run(ctx)
	}()
}
