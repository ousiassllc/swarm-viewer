package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	t.Parallel()
	got := DefaultSettings()
	want := Settings{NotificationsEnabled: true, ScreenPollSeconds: 10, NotifyPollMinutes: 10}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsValidValues(t *testing.T) {
	t.Parallel()
	for _, n := range []int{-5, 0, 1, 4, 6, 9, 11, 29, 31, 59, 61, 120} {
		if ValidScreenPollSeconds(n) || ValidNotifyPollMinutes(n) {
			t.Errorf("%d は許容外", n)
		}
	}
	for _, n := range []int{5, 10, 30, 60} {
		if !ValidScreenPollSeconds(n) || !ValidNotifyPollMinutes(n) {
			t.Errorf("%d は許容値", n)
		}
	}
	want := []int{5, 10, 30, 60}
	if !reflect.DeepEqual(AllowedScreenPollSeconds(), want) || !reflect.DeepEqual(AllowedNotifyPollMinutes(), want) {
		t.Error("許容値の一覧")
	}
	// 返した一覧を書き換えても内部の許容値は変わらない
	AllowedScreenPollSeconds()[0] = 1
	if ValidScreenPollSeconds(1) {
		t.Error("許容値が外から書き換えられた")
	}
}

func TestSettingsValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mod    func(*Settings)
		fields []string
	}{
		{"既定", func(*Settings) {}, nil},
		{"通知オフ・デバッグオン・最後の接続", func(s *Settings) {
			s.NotificationsEnabled, s.DebugLog, s.LastConnectionID = false, true, "01ABC"
		}, nil},
		{"画面更新が不正", func(s *Settings) { s.ScreenPollSeconds = 15 }, []string{FieldScreenPollSeconds}},
		{"通知用が不正", func(s *Settings) { s.NotifyPollMinutes = 0 }, []string{FieldNotifyPollMinutes}},
		{
			"両方不正", func(s *Settings) { s.ScreenPollSeconds, s.NotifyPollMinutes = -1, 7 },
			[]string{FieldScreenPollSeconds, FieldNotifyPollMinutes},
		},
		{"零値は不正", func(s *Settings) { *s = Settings{} }, []string{FieldScreenPollSeconds, FieldNotifyPollMinutes}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := DefaultSettings()
			tt.mod(&s)
			err := s.Validate()
			if tt.fields == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var se *SettingsError
			if !errors.As(err, &se) || !reflect.DeepEqual(se.Fields, tt.fields) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
