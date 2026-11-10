package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

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
