package domain

import (
	"slices"
	"strings"
)

// 設定の既定値（ER §3.2）。
const (
	DefaultScreenPollSeconds    = 10
	DefaultNotifyPollMinutes    = 10
	DefaultNotificationsEnabled = true
	// DefaultAutoUpdateCheck は新しい版の自動確認の既定（オン。#109）。
	DefaultAutoUpdateCheck = true
)

// 設定項目名（SettingsError.Fields の要素）。
const (
	FieldScreenPollSeconds = "screen_poll_seconds"
	FieldNotifyPollMinutes = "notify_poll_minutes"
)

var (
	screenPollSeconds = []int{5, 10, 30, 60}
	notifyPollMinutes = []int{5, 10, 30, 60}
)

// Settings はアプリ全体の設定（1 行固定）。LastConnectionID は未設定（NULL）のとき空文字。
type Settings struct {
	NotificationsEnabled bool
	ScreenPollSeconds    int
	NotifyPollMinutes    int
	DebugLog             bool
	// AutoUpdateCheck は新しい版を起動時と 6 時間ごとに自動で確認するか（#109）。
	AutoUpdateCheck  bool
	LastConnectionID string
}

// DefaultSettings は初期設定を返す（通知オン・画面更新 10 秒・通知用ポーリング 10 分・デバッグログオフ・更新の自動確認オン）。
func DefaultSettings() Settings {
	return Settings{
		NotificationsEnabled: DefaultNotificationsEnabled,
		ScreenPollSeconds:    DefaultScreenPollSeconds,
		NotifyPollMinutes:    DefaultNotifyPollMinutes,
		AutoUpdateCheck:      DefaultAutoUpdateCheck,
	}
}

// AllowedScreenPollSeconds は画面更新間隔（秒）の許容値（昇順のコピー）。
func AllowedScreenPollSeconds() []int { return slices.Clone(screenPollSeconds) }

// AllowedNotifyPollMinutes は通知用ポーリング間隔（分）の許容値（昇順のコピー）。
func AllowedNotifyPollMinutes() []int { return slices.Clone(notifyPollMinutes) }

// ValidScreenPollSeconds は許容値（5 / 10 / 30 / 60）かを返す。
func ValidScreenPollSeconds(n int) bool { return slices.Contains(screenPollSeconds, n) }

// ValidNotifyPollMinutes は許容値（5 / 10 / 30 / 60）かを返す。
func ValidNotifyPollMinutes(n int) bool { return slices.Contains(notifyPollMinutes, n) }

// SettingsError は許容外の設定値。Fields は不正な項目名（Field* 定数）。
type SettingsError struct {
	Fields []string
}

// Error は固定文言（項目名のみ。値は含めない）。
func (e *SettingsError) Error() string {
	return "domain: invalid settings: " + strings.Join(e.Fields, ", ")
}

// Validate は許容値を検査する。問題が無ければ nil、あれば *SettingsError（errors.As で取り出せる）。
func (s Settings) Validate() error {
	var fields []string
	if !ValidScreenPollSeconds(s.ScreenPollSeconds) {
		fields = append(fields, FieldScreenPollSeconds)
	}
	if !ValidNotifyPollMinutes(s.NotifyPollMinutes) {
		fields = append(fields, FieldNotifyPollMinutes)
	}
	if len(fields) == 0 {
		return nil
	}
	return &SettingsError{Fields: fields}
}
