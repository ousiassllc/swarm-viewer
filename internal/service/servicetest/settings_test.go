package servicetest

import (
	"context"
	"errors"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

func validInput() service.SettingsInput {
	return service.SettingsInput{NotificationsEnabled: true, ScreenPollSeconds: 30, NotifyPollMinutes: 5, DebugLog: true}
}

func TestSettingsUpdateSavesAndKeepsLastConnection(t *testing.T) {
	e := NewEnv(t)
	ctx := context.Background()
	e.Store.Cfg.LastConnectionID = "c1"

	got, err := e.Settings.Update(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Settings{NotificationsEnabled: true, ScreenPollSeconds: 30, NotifyPollMinutes: 5, DebugLog: true, LastConnectionID: "c1"}
	if got != want {
		t.Fatalf("Update = %+v, want %+v", got, want)
	}
	if st, _ := e.Settings.Get(ctx); st != want {
		t.Fatalf("Get = %+v, want %+v", st, want)
	}
}

func TestSettingsUpdateValidation(t *testing.T) {
	cases := map[string]struct {
		mut    func(*service.SettingsInput)
		fields []string
	}{
		"screen 4":     {func(i *service.SettingsInput) { i.ScreenPollSeconds = 4 }, []string{domain.FieldScreenPollSeconds}},
		"screen 0":     {func(i *service.SettingsInput) { i.ScreenPollSeconds = 0 }, []string{domain.FieldScreenPollSeconds}},
		"screen 61":    {func(i *service.SettingsInput) { i.ScreenPollSeconds = 61 }, []string{domain.FieldScreenPollSeconds}},
		"screen -10":   {func(i *service.SettingsInput) { i.ScreenPollSeconds = -10 }, []string{domain.FieldScreenPollSeconds}},
		"notify 15":    {func(i *service.SettingsInput) { i.NotifyPollMinutes = 15 }, []string{domain.FieldNotifyPollMinutes}},
		"notify 0":     {func(i *service.SettingsInput) { i.NotifyPollMinutes = 0 }, []string{domain.FieldNotifyPollMinutes}},
		"both invalid": {func(i *service.SettingsInput) { i.ScreenPollSeconds, i.NotifyPollMinutes = 1, 1 }, []string{domain.FieldScreenPollSeconds, domain.FieldNotifyPollMinutes}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEnv(t)
			before := e.Store.Cfg
			sub := e.Settings.Subscribe()
			in := validInput()
			tc.mut(&in)

			_, err := e.Settings.Update(context.Background(), in)

			var ve *service.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *service.ValidationError", err)
			}
			if len(ve.Fields) != len(tc.fields) {
				t.Fatalf("Fields = %v, want %v", ve.Fields, tc.fields)
			}
			for i := range tc.fields {
				if ve.Fields[i] != tc.fields[i] {
					t.Fatalf("Fields = %v, want %v", ve.Fields, tc.fields)
				}
			}
			var se *domain.SettingsError
			if !errors.As(err, &se) {
				t.Fatal("service.ValidationError should wrap *domain.SettingsError")
			}
			if e.Store.Cfg != before {
				t.Fatal("invalid input must not be saved")
			}
			select {
			case <-sub:
				t.Fatal("invalid input must not notify subscribers")
			default:
			}
		})
	}
}

func TestSettingsAllowedBoundaries(t *testing.T) {
	for _, sec := range domain.AllowedScreenPollSeconds() {
		for _, min := range domain.AllowedNotifyPollMinutes() {
			e := NewEnv(t)
			in := validInput()
			in.ScreenPollSeconds, in.NotifyPollMinutes = sec, min
			if _, err := e.Settings.Update(context.Background(), in); err != nil {
				t.Fatalf("(%d,%d): %v", sec, min, err)
			}
		}
	}
}

func TestSettingsStoreFailures(t *testing.T) {
	for _, method := range []string{"Settings", "SaveSettings"} {
		t.Run(method, func(t *testing.T) {
			e := NewEnv(t)
			sub := e.Settings.Subscribe()
			e.Store.SetErr(method, ErrInjected)
			_, err := e.Settings.Update(context.Background(), validInput())
			if !errors.Is(err, ErrInjected) {
				t.Fatalf("err = %v", err)
			}
			var ve *service.ValidationError
			if errors.As(err, &ve) {
				t.Fatal("store failure must not be a service.ValidationError")
			}
			select {
			case <-sub:
				t.Fatal("failed save must not notify")
			default:
			}
		})
	}
}

func TestSettingsSubscribeDeliversToEverySubscriber(t *testing.T) {
	e := NewEnv(t)
	a, b := e.Settings.Subscribe(), e.Settings.Subscribe()

	want, err := e.Settings.Update(context.Background(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	if got := Recv(t, a, "subscriber a"); got != want {
		t.Fatalf("a = %+v", got)
	}
	if got := Recv(t, b, "subscriber b"); got != want {
		t.Fatalf("b = %+v", got)
	}
}

func TestSettingsSubscribeKeepsLatestForSlowSubscriber(t *testing.T) {
	e := NewEnv(t)
	sub := e.Settings.Subscribe()
	ctx := context.Background()

	first := validInput()
	first.ScreenPollSeconds = 5
	second := validInput()
	second.ScreenPollSeconds = 60
	if _, err := e.Settings.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Settings.Update(ctx, second); err != nil { // 購読者が読む前に 2 回目。送信側はブロックしない
		t.Fatal(err)
	}
	if got := Recv(t, sub, "latest settings"); got.ScreenPollSeconds != 60 {
		t.Fatalf("got %d, want latest value 60", got.ScreenPollSeconds)
	}
}
