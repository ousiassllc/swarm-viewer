package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// DraftTTL はペアリング下書きの有効期間（作成から。confirm 失敗でも延長しない）。
const DraftTTL = 5 * time.Minute

// DraftID はペアリング下書きの不透明な ID（ランダム 128 bit、base64url）。
type DraftID string

// DraftView は確認画面の表示用。token を含まない。
type DraftView struct {
	Name         string
	Host         string   // URL の host のみ
	SessionsHint []string // ホストが申告した許可セッション（表示のヒントのみ）
	ExpiresAt    time.Time
}

type draft struct {
	id      DraftID
	payload domain.PairingPayload
	expires time.Time
}

// PairingService は接続文字列の解析・下書きの管理・疎通確認・接続の保存（FR-PAIR-1〜4）。
// 下書き（token を含む）はメモリだけに置き、同時に 1 件。
type PairingService struct {
	report  ReportClient
	store   Store
	secrets SecretStore
	ids     IDGenerator
	clock   Clock
	conns   *ConnectionService
	log     *slog.Logger

	confirmMu sync.Mutex // Confirm を直列化する（同じ下書きの二重確定・接続の二重作成を防ぐ）

	mu    sync.Mutex // draft を守る
	draft *draft
}

// NewPairingService は PairingService を作る。conns は再ペアリング後の MarkRepaired に使う。
func NewPairingService(report ReportClient, store Store, secrets SecretStore, ids IDGenerator, clock Clock, conns *ConnectionService, log *slog.Logger) *PairingService {
	return &PairingService{report: report, store: store, secrets: secrets, ids: ids, clock: clock, conns: conns, log: log}
}

// Parse は接続文字列を検証して下書きを作る。古い下書きは破棄する。
// 形式・版・URL・token の不正は *ValidationError（原因は errors.Is で domain.ErrPairing* / ErrInvalidURL に写像できる）。
func (s *PairingService) Parse(raw string) (DraftID, error) {
	payload, err := domain.ParsePairing(raw)
	if err != nil {
		return "", &ValidationError{Fields: []string{"pairing"}, Err: err}
	}
	id, err := newDraftID()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.draft = &draft{id: id, payload: payload, expires: s.clock.Now().Add(DraftTTL)}
	s.mu.Unlock()
	return id, nil
}

// Draft は確認画面用の表示情報を返す。期限切れ・破棄済み・未知の ID は ErrDraftExpired。
func (s *PairingService) Draft(id DraftID) (DraftView, error) {
	d, err := s.lookup(id)
	if err != nil {
		return DraftView{}, err
	}
	return DraftView{
		Name:         d.payload.Name,
		Host:         domain.HostOf(d.payload.URL),
		SessionsHint: append([]string(nil), d.payload.SessionsHint...),
		ExpiresAt:    d.expires,
	}, nil
}

// Discard は下書きを即時に破棄する。
func (s *PairingService) Discard(id DraftID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draft != nil && s.draft.id == id {
		s.draft = nil
	}
}

// Confirm は疎通確認（Probe）に成功したときだけ接続を保存する。
//
// 失敗しても下書きは残す（再試行できる。期限は延長しない）。Probe の失敗は *domain.FetchError。
// 同じ URL の接続があれば id・baselined_at を維持して token を差し替え、無ければ新規 ID で baselined_at なしで作る。
// 秘密領域への保存に失敗したら、新規に作った行は削除する。
func (s *PairingService) Confirm(ctx context.Context, id DraftID) (domain.Connection, error) {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	d, err := s.lookup(id)
	if err != nil {
		return domain.Connection{}, err
	}
	p := d.payload

	if _, err := s.report.Probe(ctx, p.URL, p.Token); err != nil {
		fe := classify(err)
		s.log.Warn("pairing probe failed", "host", domain.HostOf(p.URL), "kind", fe.Kind.String(), "code", fe.Code)
		return domain.Connection{}, fe
	}
	// 疎通確認の間に破棄・置換されていたら保存しない。
	if _, err := s.lookup(id); err != nil {
		return domain.Connection{}, err
	}

	conn, created, err := s.save(ctx, p)
	if err != nil {
		return domain.Connection{}, err
	}
	if err := s.secrets.Set(conn.ID, p.Token); err != nil {
		if created {
			// 行だけ残る不整合を防ぐ。呼び出し元が中断していても戻す。
			if delErr := s.store.DeleteConnection(context.WithoutCancel(ctx), conn.ID); delErr != nil {
				s.log.Warn("rollback of new connection failed", "connection_id", conn.ID, "err", delErr)
			}
		}
		s.log.Warn("saving token failed", "connection_id", conn.ID, "host", conn.Host())
		return domain.Connection{}, errors.New("service: saving token to the secret store failed")
	}
	s.conns.MarkRepaired(conn.ID)
	s.Discard(id)
	return conn, nil
}

// save は URL で接続を引き、あれば更新（id・baselined_at 維持）、無ければ新規に保存する。
func (s *PairingService) save(ctx context.Context, p domain.PairingPayload) (conn domain.Connection, created bool, err error) {
	now := s.clock.Now()
	conn, found, err := s.store.ConnectionByURL(ctx, p.URL)
	if err != nil {
		return domain.Connection{}, false, err
	}
	if found {
		conn.Name = p.Name
		conn.UpdatedAt = now
	} else {
		conn = domain.Connection{ID: s.ids.NewID(), Name: p.Name, URL: p.URL, CreatedAt: now, UpdatedAt: now}
	}
	if err := s.store.SaveConnection(ctx, conn); err != nil {
		return domain.Connection{}, false, err
	}
	return conn, !found, nil
}

// lookup は id の下書きを返す。期限切れは消して ErrDraftExpired。
func (s *PairingService) lookup(id DraftID) (draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draft == nil || s.draft.id != id {
		return draft{}, ErrDraftExpired
	}
	if !s.clock.Now().Before(s.draft.expires) {
		s.draft = nil
		return draft{}, ErrDraftExpired
	}
	return *s.draft, nil
}

func newDraftID() (DraftID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return DraftID(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
