package service

import (
	"context"
	"errors"
	"sync"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// SettingsInput は設定フォームの入力。LastConnectionID は含まない（SetLastConnection 経由）。
type SettingsInput struct {
	NotificationsEnabled bool
	ScreenPollSeconds    int
	NotifyPollMinutes    int
	DebugLog             bool
	AutoUpdateCheck      bool
}

// SettingsService はアプリ全体の設定（FR-SET-1）。
type SettingsService struct {
	store Store

	mu   sync.Mutex // Update の読み取り→保存と、購読者リストを守る
	subs []chan domain.Settings
}

// NewSettingsService は SettingsService を作る。
func NewSettingsService(store Store) *SettingsService {
	return &SettingsService{store: store}
}

// Get は現在の設定を返す。
func (s *SettingsService) Get(ctx context.Context) (domain.Settings, error) {
	return s.store.Settings(ctx)
}

// Update は設定を検証して保存し、購読者へ新しい設定を送る。許容外は *ValidationError。
func (s *SettingsService) Update(ctx context.Context, in SettingsInput) (domain.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, err := s.store.Settings(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	next := cur
	next.NotificationsEnabled = in.NotificationsEnabled
	next.ScreenPollSeconds = in.ScreenPollSeconds
	next.NotifyPollMinutes = in.NotifyPollMinutes
	next.DebugLog = in.DebugLog
	next.AutoUpdateCheck = in.AutoUpdateCheck
	if err := next.Validate(); err != nil {
		ve := &ValidationError{Err: err}
		var se *domain.SettingsError
		if errors.As(err, &se) {
			ve.Fields = se.Fields
		}
		return domain.Settings{}, ve
	}
	if err := s.store.SaveSettings(ctx, next); err != nil {
		return domain.Settings{}, err
	}
	for _, ch := range s.subs {
		publish(ch, next)
	}
	return next, nil
}

// Subscribe は変更通知のチャネルを返す（呼び出しごとに独立）。スケジューラの ticker Reset・ログレベル更新が使う。
// 遅い購読者のために 1 件だけ保持し、溜まったら古い値を捨てて最新を残す。購読は解除できない（アプリの寿命と同じ）。
func (s *SettingsService) Subscribe() <-chan domain.Settings {
	ch := make(chan domain.Settings, 1)
	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	return ch
}

// publish は最新値を優先して非ブロッキングで送る。送信側は Update の mutex で直列化されている。
func publish(ch chan domain.Settings, st domain.Settings) {
	select {
	case ch <- st:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- st:
	default:
	}
}
