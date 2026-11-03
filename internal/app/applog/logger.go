package applog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// ローテーションの設定（NFR-LOG-1: 上限 5 MB × 3 世代）。
// 「3 世代」は現行ファイルを含めて 3 ファイルまでと解釈する（現行 1 + 退避 2）。
const (
	MaxSizeMB  = 5
	MaxBackups = 2
	// MaxAgeDays は 0（期間では削除しない。世代数だけで上限を決める）。
	MaxAgeDays = 0
)

const (
	appDirName = "swarm-viewer"
	logDirName = "logs"
	logFile    = "swarm-viewer.log"
	dirMode    = 0o700
)

// Logging はアプリのロガーとログレベル・ファイルの寿命をまとめる。
type Logging struct {
	// Logger は全層へ渡す *slog.Logger（マスク済み）。
	Logger *slog.Logger
	// Level は実行時に切り替えるログレベル（既定は警告以上。NFR-LOG-2）。
	Level *slog.LevelVar

	closer io.Closer
}

// DefaultDir は OS のユーザー設定領域の下のログディレクトリ（UserConfigDir()/swarm-viewer/logs）。
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, logDirName), nil
}

// NewRotator は dir 直下のログファイルへ書く lumberjack を返す（ファイルは初回の書き込みで 0600 で作られる）。
func NewRotator(dir string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   filepath.Join(dir, logFile),
		MaxSize:    MaxSizeMB,
		MaxBackups: MaxBackups,
		MaxAge:     MaxAgeDays,
		LocalTime:  false,
		Compress:   false,
	}
}

// Open は dir（無ければ 0700 で作る）にローテーション付きのログを開く。
func Open(dir string) (*Logging, error) {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, err
	}
	rot := NewRotator(dir)
	l := New(rot)
	l.closer = rot
	return l, nil
}

// New は w へ JSON で書く Logging を返す。メッセージ・属性ともマスクする（NFR-LOG-3）。
func New(w io.Writer) *Logging {
	lv := new(slog.LevelVar)
	lv.Set(LevelFor(false))
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv, ReplaceAttr: ReplaceAttr})
	return &Logging{Logger: slog.New(maskHandler{inner: h}), Level: lv}
}

// LevelFor はデバッグログ設定に対応するレベル。オフは警告以上のみ（NFR-LOG-2）。
func LevelFor(debug bool) slog.Level {
	if debug {
		return slog.LevelDebug
	}
	return slog.LevelWarn
}

// Follow は設定の変更通知を受けてレベルを切り替える。ctx のキャンセルかチャネルの close で返る。
func (l *Logging) Follow(ctx context.Context, changes <-chan domain.Settings) {
	for {
		select {
		case <-ctx.Done():
			return
		case st, ok := <-changes:
			if !ok {
				return
			}
			l.Level.Set(LevelFor(st.DebugLog))
		}
	}
}

// Close はログファイルを閉じる。
func (l *Logging) Close() error {
	if l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

// maskHandler はメッセージ本文にも Scrub をかける（属性は ReplaceAttr が処理する）。
type maskHandler struct{ inner slog.Handler }

func (m maskHandler) Enabled(ctx context.Context, lv slog.Level) bool {
	return m.inner.Enabled(ctx, lv)
}

func (m maskHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = Scrub(r.Message)
	return m.inner.Handle(ctx, r)
}

func (m maskHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return maskHandler{inner: m.inner.WithAttrs(attrs)}
}

func (m maskHandler) WithGroup(name string) slog.Handler {
	return maskHandler{inner: m.inner.WithGroup(name)}
}
