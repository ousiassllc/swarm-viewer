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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

// begin は更新中の状態へ移す。適用できる新しい版が無ければ ErrNotAvailable、更新中なら ErrBusy。
func (s *Service) begin() (update.Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.st.Updating:
		return update.Release{}, ErrBusy
	case !s.enabled || !s.st.Available:
		return update.Release{}, ErrNotAvailable
	}
	s.st.Updating, s.st.ApplyFailed, s.st.Done, s.st.Total = true, FailNone, 0, 0
	return s.rel, nil
}

// ApplyAsync は更新を別 goroutine で始める（画面は状態を読み直して進み具合を出す）。始められなければ ErrNotAvailable・ErrBusy。
// 返すチャネルは更新の終わり（失敗・または Quit を呼んだあと）で閉じる。
func (s *Service) ApplyAsync() (<-chan struct{}, error) {
	rel, err := s.begin()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	base := s.runCtx
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(base, applyTimeout)
		defer cancel()
		s.run(ctx, rel)
	}()
	return done, nil
}

// Apply は更新を同期で行う（テスト用。画面は ApplyAsync を使う）。
func (s *Service) Apply(ctx context.Context) error {
	rel, err := s.begin()
	if err != nil {
		return err
	}
	s.run(ctx, rel)
	return nil
}

// run は取得・検証・適用を行い、成功すれば Quit を呼ぶ。失敗は状態（ApplyFailed）にして、取得したファイルを消す。
func (s *Service) run(ctx context.Context, rel update.Release) {
	kind, dir, err := s.fetchAndInstall(ctx, rel)
	if err != nil {
		if dir != "" {
			_ = os.RemoveAll(dir)
		}
		s.opts.Log.Warn("update failed", "tag", rel.Tag, "kind", kind, "err", err)
		s.mu.Lock()
		s.st.Updating, s.st.ApplyFailed = false, kind
		s.mu.Unlock()
		return
	}
	s.opts.Log.Info("update installed; quitting", "tag", rel.Tag)
	if s.opts.Quit != nil {
		s.opts.Quit()
	}
}

// fetchAndInstall は更新の本体。失敗したときは種類・作った一時フォルダ（消すもの）・原因を返す。
// 成功したとき、Windows はインストーラがそのファイルを使うので一時フォルダを残す（OS の一時フォルダの掃除に任せる）。
func (s *Service) fetchAndInstall(ctx context.Context, rel update.Release) (FailKind, string, error) {
	name, _ := update.AssetName(rel.Tag, s.opts.GOOS)
	asset, err := rel.Asset(name)
	if err != nil {
		return FailVerify, "", err
	}
	sumsAsset, err := rel.Asset(update.SumsName)
	if err != nil {
		return FailVerify, "", err
	}
	dir, err := os.MkdirTemp(s.opts.TempRoot, "swarm-viewer-update-")
	if err != nil {
		return FailDownload, "", err
	}

	want, kind, err := s.fetchSum(ctx, sumsAsset, name)
	if err != nil {
		return kind, dir, err
	}
	file := filepath.Join(dir, name) // name は AssetName が作る固定の形（区切り文字を含まない）
	got, err := s.download(ctx, asset, file)
	if err != nil {
		return FailDownload, dir, err
	}
	if got != want {
		return FailVerify, dir, errHashMismatch
	}
	if err := s.opts.Installer.Install(ctx, file); err != nil {
		return FailInstall, dir, err
	}
	if s.opts.GOOS != "windows" {
		_ = os.RemoveAll(dir) // 適用済み。Windows だけは、起動したインストーラがファイルを使う
	}
	return FailNone, "", nil
}

// fetchSum は SHA256SUMS を取得して、name の行のハッシュ（小文字の 16 進）を返す。
func (s *Service) fetchSum(ctx context.Context, sumsAsset update.Asset, name string) (string, FailKind, error) {
	var buf bytes.Buffer
	if err := s.opts.Source.Download(ctx, sumsAsset, &limitedWriter{w: &buf, left: maxSumsBytes}); err != nil {
		if errors.Is(err, errSumsTooLarge) {
			return "", FailVerify, err
		}
		return "", FailDownload, err
	}
	sums, err := update.ParseSums(buf.Bytes())
	if err != nil {
		return "", FailVerify, err
	}
	want, err := sums.Lookup(name)
	if err != nil {
		return "", FailVerify, err
	}
	return want, FailNone, nil
}

// download は asset を file へ取得し、SHA-256（小文字の 16 進）を返す。進み具合は状態へ反映する。
func (s *Service) download(ctx context.Context, asset update.Asset, file string) (string, error) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // file は自分で作った一時フォルダの下の固定名
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.st.Total = asset.Size
	s.mu.Unlock()
	h := sha256.New()
	err = s.opts.Source.Download(ctx, asset, io.MultiWriter(f, h, &progress{s: s}))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// progress は書き込まれたバイト数を状態へ反映する。
type progress struct{ s *Service }

func (p *progress) Write(b []byte) (int, error) {
	p.s.mu.Lock()
	p.s.st.Done += int64(len(b))
	p.s.mu.Unlock()
	return len(b), nil
}

// limitedWriter は上限を超える書き込みを errSumsTooLarge で拒否する。
type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(b []byte) (int, error) {
	if len(b) > l.left {
		return 0, fmt.Errorf("%w (limit %d bytes)", errSumsTooLarge, maxSumsBytes)
	}
	l.left -= len(b)
	return l.w.Write(b)
}
