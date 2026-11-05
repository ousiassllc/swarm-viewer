package format_test

import (
	"math"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
)

func TestStateLabelAndTone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state domain.PaneState
		label string
		tone  format.Tone
	}{
		{domain.StateWorking, "作業中", format.ToneAccent},
		{domain.StateAsking, "答え待ち", format.ToneWarning},
		{domain.StateIdle, "指示待ち", format.ToneSuccess},
		{domain.StateStalled, "止まってるかも", format.ToneError},
		{domain.StateShell, "空き", format.ToneMuted},
		{domain.StateUnknown, "判定できない", format.ToneMuted},
		{domain.PaneState("future-state"), "判定できない", format.ToneMuted},
		{domain.PaneState(""), "判定できない", format.ToneMuted},
	}
	for _, tt := range tests {
		if got := format.StateLabel(tt.state); got != tt.label {
			t.Errorf("StateLabel(%q) = %q, want %q", tt.state, got, tt.label)
		}
		if got := format.StateTone(tt.state); got != tt.tone {
			t.Errorf("StateTone(%q) = %q, want %q", tt.state, got, tt.tone)
		}
	}
}

func TestToneClasses_AllTonesAndUnknown(t *testing.T) {
	t.Parallel()
	tones := []format.Tone{format.ToneAccent, format.ToneWarning, format.ToneSuccess, format.ToneError, format.ToneMuted, format.Tone("???")}
	seen := map[string]format.Tone{}
	for _, tone := range tones[:5] {
		for name, fn := range map[string]func(format.Tone) string{
			"badge": format.BadgeClass, "bar": format.BarClass, "text": format.TextClass, "border": format.BorderClass,
		} {
			cls := fn(tone)
			if cls == "" {
				t.Errorf("%s(%q) が空", name, tone)
			}
			key := name + "|" + cls
			if prev, dup := seen[key]; dup {
				t.Errorf("%s: %q と %q が同じクラス %q", name, prev, tone, cls)
			}
			seen[key] = tone
		}
	}
	for name, fn := range map[string]func(format.Tone) string{
		"badge": format.BadgeClass, "bar": format.BarClass, "text": format.TextClass, "border": format.BorderClass,
	} {
		if fn(format.Tone("???")) != fn(format.ToneMuted) {
			t.Errorf("%s: 未知の Tone は muted と同じになるべき", name)
		}
	}
}

func TestPercentTone_Boundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    float64
		want format.Tone
	}{
		{0, format.ToneAccent},
		{79.9, format.ToneAccent},
		{80, format.ToneWarning},
		{94.9, format.ToneWarning},
		{95, format.ToneError},
		{100, format.ToneError},
		{math.NaN(), format.ToneAccent},
	}
	for _, tt := range tests {
		if got := format.PercentTone(tt.v); got != tt.want {
			t.Errorf("PercentTone(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestSlotTone(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	tests := []struct {
		name string
		slot domain.Slot
		want format.Tone
	}{
		{"percent 79.9", domain.Slot{Available: true, Used: f(79.9), Unit: domain.UnitPercent}, format.ToneAccent},
		{"percent 80", domain.Slot{Available: true, Used: f(80), Unit: domain.UnitPercent}, format.ToneWarning},
		{"percent 94.9", domain.Slot{Available: true, Used: f(94.9), Unit: domain.UnitPercent}, format.ToneWarning},
		{"percent 95", domain.Slot{Available: true, Used: f(95), Unit: domain.UnitPercent}, format.ToneError},
		{"exhausted は used に関わらず error", domain.Slot{Available: true, Used: f(10), Unit: domain.UnitPercent, Status: domain.SlotStatusExhausted}, format.ToneError},
		{"usd はしきい値なし", domain.Slot{Available: true, Used: f(500), Unit: domain.UnitUSD}, format.ToneAccent},
		{"unavailable", domain.Slot{}, format.ToneAccent},
	}
	for _, tt := range tests {
		if got := format.SlotTone(tt.slot); got != tt.want {
			t.Errorf("%s: SlotTone = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFromDomainTone_Unknown(t *testing.T) {
	t.Parallel()
	if got := format.FromDomainTone(domain.Tone("x")); got != format.ToneMuted {
		t.Errorf("未知の domain.Tone = %q, want muted", got)
	}
}

func TestPercentAndUSD(t *testing.T) {
	t.Parallel()
	for in, want := range map[float64]string{0: "0%", 79.9: "79.9%", 80: "80%", 94.96: "95%", 12.34: "12.3%", 100: "100%"} {
		if got := format.Percent(in); got != want {
			t.Errorf("Percent(%v) = %q, want %q", in, got, want)
		}
	}
	if got := format.Percent(math.NaN()); got != format.Dash {
		t.Errorf("Percent(NaN) = %q", got)
	}
	for in, want := range map[float64]string{0: "$0.00", 12.5: "$12.50", 1234.567: "$1234.57"} {
		if got := format.USD(in); got != want {
			t.Errorf("USD(%v) = %q, want %q", in, got, want)
		}
	}
	if got := format.USD(math.Inf(1)); got != format.Dash {
		t.Errorf("USD(Inf) = %q", got)
	}
	if got := format.GB(12.34); got != "12.3 GB" {
		t.Errorf("GB = %q", got)
	}
}

func TestBarValue(t *testing.T) {
	t.Parallel()
	for in, want := range map[float64]float64{-5: 0, 0: 0, 42.5: 42.5, 100: 100, 250: 100, math.NaN(): 0} {
		if got := format.BarValue(in); got != want {
			t.Errorf("BarValue(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestUntil(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-time.Hour, "まもなく"},
		{0, "まもなく"},
		{59 * time.Second, "まもなく"},
		{time.Minute, "あと 1 分"},
		{59*time.Minute + 59*time.Second, "あと 59 分"},
		{time.Hour, "あと 1 時間"},
		{3*time.Hour + 30*time.Minute, "あと 3 時間"},
		{23*time.Hour + 59*time.Minute, "あと 23 時間"},
		{24 * time.Hour, "あと 1 日"},
		{49 * time.Hour, "あと 2 日"},
	}
	for _, tt := range tests {
		if got := format.Until(now.Add(tt.d), now); got != tt.want {
			t.Errorf("Until(+%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestClockAndDateTime_ZeroIsDash(t *testing.T) {
	t.Parallel()
	if format.ClockTime(time.Time{}) != format.Dash || format.DateTime(time.Time{}) != format.Dash {
		t.Error("零値は Dash になるべき")
	}
	ts := time.Date(2026, 11, 3, 9, 5, 0, 0, time.Local)
	if got := format.ClockTime(ts); got != "09:05" {
		t.Errorf("ClockTime = %q", got)
	}
	if got := format.DateTime(ts); got != "11/03 09:05" {
		t.Errorf("DateTime = %q", got)
	}
}

func TestAttentionLabelAndTone(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]string{
		"asking": "答え待ち", "stalled": "止まってるかも", "report_waiting": "報告待ち",
		"auth_expired": "ログイン期限切れ", "auth_expiring": "ログイン期限間近", "future_kind": "future_kind",
	} {
		if got := format.AttentionLabel(kind); got != want {
			t.Errorf("AttentionLabel(%q) = %q, want %q", kind, got, want)
		}
	}
	if format.AttentionTone("future_kind") != format.ToneMuted {
		t.Error("未知の kind は muted")
	}
}

func TestUsageStatusLabel(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]string{
		"ok": "", "stale": "古いデータ", "unavailable": "取得できません", "pending": "取得中",
		"disabled": "利用上限の取得は無効です", "weird": "取得できません", "": "取得できません",
	} {
		if got := format.UsageStatusLabel(status); got != want {
			t.Errorf("UsageStatusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestUsageReasonLabel(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	for name, tt := range map[string]struct {
		in   *string
		want string
	}{
		"without_usage": {str("without_usage"), "ログイン切れの可能性"},
		"no_email":      {str("no_email"), "アカウントを特定できません（ログイン切れの可能性）"},
		"知らない値":         {str("brand-new"), ""},
		"空文字":           {str(""), ""},
		"nil":           {nil, ""},
	} {
		if got := format.UsageReasonLabel(tt.in); got != tt.want {
			t.Errorf("%s: UsageReasonLabel = %q, want %q", name, got, tt.want)
		}
	}
}

func TestNoteSummary(t *testing.T) {
	t.Parallel()
	if r, i, m := format.NoteSummary(nil); r != "" || i != nil || m != 0 {
		t.Error("nil は空")
	}
	n := &domain.Note{Repo: "r", Items: []string{"a", "b", "c", "d", "e"}}
	r, items, more := format.NoteSummary(n)
	if r != "r" || len(items) != 3 || more != 2 {
		t.Errorf("got %q %v %d", r, items, more)
	}
	_, items, more = format.NoteSummary(&domain.Note{Items: []string{"a", "b", "c"}})
	if len(items) != 3 || more != 0 {
		t.Errorf("ちょうど 3 件: %v %d", items, more)
	}
}

func TestCursorWindowLabel(t *testing.T) {
	t.Parallel()
	m, w, x := "monthly", "weekly", "daily"
	if format.CursorWindowLabel(nil) != "" || format.CursorWindowLabel(&m) != "月間" ||
		format.CursorWindowLabel(&w) != "週間" || format.CursorWindowLabel(&x) != "daily" {
		t.Error("CursorWindowLabel")
	}
}
