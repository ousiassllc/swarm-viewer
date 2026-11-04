package service

import (
	"context"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Clock は時間の取得と周期タイマー。テストでは手動で進めるフェイクに差し替える（NFR-QLT-3）。
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}

// Ticker は周期タイマー。time.Ticker と同じく Stop 後は C に送られない。
type Ticker interface {
	C() <-chan time.Time
	Reset(d time.Duration)
	Stop()
}

// IDGenerator は接続 ID（ULID）を発行する。
type IDGenerator interface{ NewID() string }

// PermissionState は OS 通知の許可状態（設定画面の表示用）。
type PermissionState string

// OS 通知の許可状態。
const (
	PermissionUnknown PermissionState = "unknown"
	PermissionGranted PermissionState = "granted"
	PermissionDenied  PermissionState = "denied"
)

// Notification は OS 通知 1 件。token・URL は持たない。Detail は asking のときだけ入る。
type Notification struct {
	ConnectionID   string
	ConnectionName string
	Session        string
	Pos            string
	State          domain.PaneState
	Detail         string
}

// Notifier は OS 通知の送出。app が実装する。
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
	Permitted() PermissionState
}

// Navigator は WebView のルート遷移。app が実装する（Wails の SetURL＋前面化）。
type Navigator interface {
	Open(path string)
}

// ReportClient は swarm-console の `/report` クライアント（repository/report.Client が満たす）。
type ReportClient interface {
	Sessions(ctx context.Context, conn domain.Connection) ([]domain.Session, error)
	Report(ctx context.Context, conn domain.Connection, session string) (domain.Report, error)
	// Probe はペアリングの疎通確認。保存前なので token を直接受け取る。
	Probe(ctx context.Context, url, token string) ([]domain.Session, error)
}

// SecretStore は OS 秘密領域（repository/secret.Store が満たす）。
type SecretStore interface {
	Set(connID, token string) error
	Get(connID string) (string, error)
	Delete(connID string) error
}

// Store は SQLite の永続化（repository/store.Store が満たす）。
type Store interface {
	// connections
	Connections(ctx context.Context) ([]domain.Connection, error)
	// ConnectionByID は無いとき errors.Is(err, sql.ErrNoRows) を満たすエラーを返す。
	ConnectionByID(ctx context.Context, id string) (domain.Connection, error)
	ConnectionByURL(ctx context.Context, url string) (domain.Connection, bool, error)
	SaveConnection(ctx context.Context, c domain.Connection) error
	DeleteConnection(ctx context.Context, id string) error
	SetLastSession(ctx context.Context, id, session string) error
	MarkBaselined(ctx context.Context, id string, at time.Time) error

	// settings（id=1 の 1 行）
	Settings(ctx context.Context) (domain.Settings, error)
	SaveSettings(ctx context.Context, st domain.Settings) error
	// SetLastConnection は settings.last_connection_id だけを更新する。空文字は NULL。
	SetLastConnection(ctx context.Context, id string) error

	// snapshots
	Snapshot(ctx context.Context, connID, session string) (domain.Snapshot, bool, error)
	SaveSnapshot(ctx context.Context, connID, session string, sn domain.Snapshot) error
	PruneSnapshots(ctx context.Context, connID string, keep []string) error

	// notified_states
	NotifiedKeys(ctx context.Context, connID, session string) ([]domain.NotifiedKey, error)
	// ApplyNotified は Diff を 1 トランザクションで適用し、実際に挿入できたキーだけを返す。
	ApplyNotified(ctx context.Context, connID, session string, d domain.Diff) (inserted []domain.NotifiedKey, err error)
}
