// Package updater は新しい版の確認と自動アップデートのユースケース（Issue #109）。
//
// 起動時と 6 時間ごとに GitHub の最新リリースを確認し、現在の版より新しければ「新しい版があります」の状態を持つ。
// 利用者が更新を押したら、この OS 用のアセットと SHA256SUMS を取得し、SHA-256 が SHA256SUMS の該当行と一致したときだけ
// インストーラ（repository/installer）へ渡し、成功したらアプリを終了する。一致しない・取得が途中で切れた・SHA256SUMS に行が無いなど
// 検証できないものは、ファイルを消して何も適用しない。確認の失敗・更新の失敗は状態として持つだけで、メイン画面を止めない。
//
// ネットワークとインストーラは interface（Source・Installer）の裏にあり、テストでは偽物に差し替える。
// 設定の「自動で確認する」がオフの間は、起動時・定期の確認でネットワークに出ない。開発ビルド（版が dev）と更新ファイルの無い OS では何もしない。
package updater

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
	"github.com/ousiassllc/swarm-viewer/internal/repository/release"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

const (
	// CheckInterval は新しい版を自動で確認する間隔（起動時にも 1 回確認する）。
	CheckInterval = 6 * time.Hour
	// applyTimeout は更新（取得・検証・適用）全体の上限。
	applyTimeout = 15 * time.Minute
	// maxSumsBytes は SHA256SUMS の取得上限。
	maxSumsBytes = 64 << 10
)

// 更新の開始の失敗。
var (
	// ErrNotAvailable は適用できる新しい版が無い（確認していない・古い・この環境は対象外）。
	ErrNotAvailable = errors.New("updater: no update is available")
	// ErrBusy はすでに更新中。
	ErrBusy = errors.New("updater: an update is already in progress")
)

// 取得したファイルを検証できなかった理由（FailVerify の元）。
var (
	errHashMismatch = errors.New("updater: sha256 does not match SHA256SUMS")
	errSumsTooLarge = errors.New("updater: SHA256SUMS is too large")
)

// Source は GitHub Releases からの取得（repository/release.Client が満たす）。
type Source interface {
	Latest(ctx context.Context) (update.Release, error)
	// Download は asset を dst へ書く。途中で切れた・HTTP エラー・許可外のホストはエラー。
	Download(ctx context.Context, asset update.Asset, dst io.Writer) error
}

// Installer は検証済みのファイルの適用（repository/installer.Installer が満たす）。
type Installer interface {
	Install(ctx context.Context, file string) error
}

// SettingsReader は「自動で確認する」の設定の読み取りと変更通知（service.SettingsService が満たす）。
type SettingsReader interface {
	Get(ctx context.Context) (domain.Settings, error)
	Subscribe() <-chan domain.Settings
}

// FailKind は更新の失敗の種類（画面は種類ごとの固定文言を出す。原因の詳細・URL は出さない）。
type FailKind string

// FailKind の値。
const (
	FailNone     FailKind = ""
	FailDownload FailKind = "download" // 取得できなかった（オフライン・HTTP エラー・途中で切れた）
	FailVerify   FailKind = "verify"   // ファイルを検証できなかった（アセット・SHA256SUMS の欠け・書式・ハッシュの不一致）
	FailInstall  FailKind = "install"  // 検証したファイルの適用に失敗した
)

// Status は画面に出す状態のスナップショット。
type Status struct {
	// Available は現在の版より新しい版がある。Latest はその tag_name。
	Available bool
	Latest    string
	// Updating は更新中。Done・Total は取得したバイト数と全体（Total が 0 のときは不明）。
	Updating    bool
	Done, Total int64
	// ApplyFailed は直前の更新の失敗（更新を押し直すと消える）。
	ApplyFailed FailKind
	// CheckFailed は直前の確認が失敗した（次の確認の成功で消える）。
	CheckFailed bool
}

// Options は Service の組み立て。
type Options struct {
	Source    Source
	Installer Installer
	Settings  SettingsReader
	Clock     service.Clock
	Log       *slog.Logger
	// Version は現在の版（"X.Y.Z"。buildinfo.Version）。"dev" など読めない値なら何もしない。
	Version string
	// GOOS は実行中の OS（runtime.GOOS）。更新ファイルの無い OS では何もしない。
	GOOS string
	// Quit は更新を適用できたあとにアプリを終了する。
	Quit func()
	// TempRoot は取得したファイルを置くフォルダの親。空なら os.TempDir()。
	TempRoot string
}

// Service は新しい版の確認と更新。
type Service struct {
	opts    Options
	current string // "vX.Y.Z"。読めなければ空
	enabled bool

	checkMu sync.Mutex // 確認を直列にする

	mu     sync.Mutex
	st     Status
	rel    update.Release
	runCtx context.Context //nolint:containedctx // 更新は HTTP リクエストより長く動くので、Run の寿命に結ぶ
}

// New は Service を作る。
func New(o Options) *Service {
	s := &Service{opts: o, runCtx: context.Background()}
	if _, ok := update.AssetName("v0.0.0", o.GOOS); ok {
		if v, ok := update.Parse("v" + o.Version); ok {
			s.current, s.enabled = v.String(), true
		}
	}
	return s
}

// Enabled は確認と更新が使える環境か（リリースの版・更新ファイルのある OS）。false なら通知も確認もしない。
func (s *Service) Enabled() bool { return s.enabled }

// Status は現在の状態。
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

// Run は起動時と CheckInterval ごとに確認する。ctx のキャンセルで返る。「自動で確認する」がオフの間は、ネットワークに出ない。
// オフからオンへ変えたときは、次の周期を待たずに確認する。
func (s *Service) Run(ctx context.Context) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	s.runCtx = ctx
	s.mu.Unlock()

	sub := s.opts.Settings.Subscribe() // 先に購読して、初回の Get との間の更新を取りこぼさない
	auto := s.autoCheck(ctx)
	ticker := s.opts.Clock.NewTicker(CheckInterval)
	defer ticker.Stop()

	if auto {
		s.Check(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case st := <-sub:
			turnedOn := st.AutoUpdateCheck && !auto
			auto = st.AutoUpdateCheck
			if turnedOn {
				s.Check(ctx)
			}
		case <-ticker.C():
			if auto {
				s.Check(ctx)
			}
		}
	}
}

// autoCheck は設定を読む。読めなければ既定（オン）。
func (s *Service) autoCheck(ctx context.Context) bool {
	st, err := s.opts.Settings.Get(ctx)
	if err != nil {
		s.opts.Log.Warn("loading settings for the update check failed; using the default", "err", err)
		return domain.DefaultAutoUpdateCheck
	}
	return st.AutoUpdateCheck
}

// Check は最新リリースを 1 回確認して状態を更新する。更新中は何もしない。
// 新しい版が無い・draft/prerelease・タグが読めない・リリースが無いのはエラーにしない（通知しないだけ）。
func (s *Service) Check(ctx context.Context) {
	if !s.enabled {
		return
	}
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	if s.Status().Updating {
		return
	}
	rel, err := s.opts.Source.Latest(ctx)
	switch {
	case err == nil:
		s.setLatest(rel)
	case errors.Is(err, update.ErrIgnoredRelease), errors.Is(err, update.ErrBadRelease), errors.Is(err, release.ErrNoRelease):
		s.opts.Log.Debug("no usable release", "err", err)
		s.clearAvailable()
	case ctx.Err() != nil:
		return // 終了中。失敗として表示しない
	default:
		s.opts.Log.Info("checking for a new version failed", "err", err)
		s.mu.Lock()
		s.st.CheckFailed = true
		s.mu.Unlock()
	}
}

func (s *Service) setLatest(rel update.Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.CheckFailed = false
	if !update.Newer(s.current, rel.Tag) {
		s.st.Available, s.st.Latest, s.rel = false, "", update.Release{}
		return
	}
	s.st.Available, s.st.Latest, s.rel = true, rel.Tag, rel
}

func (s *Service) clearAvailable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.CheckFailed = false
	s.st.Available, s.st.Latest, s.rel = false, "", update.Release{}
}
