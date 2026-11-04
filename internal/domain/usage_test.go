package domain

import "testing"

func TestUsageTone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		slot Slot
		want Tone
	}{
		{"0", Slot{Available: true, Used: new(float64(0)), Unit: "percent", Status: "ok"}, ToneNormal},
		{"79.9", Slot{Available: true, Used: new(float64(79.9)), Unit: "percent", Status: "ok"}, ToneNormal},
		{"80", Slot{Available: true, Used: new(float64(80)), Unit: "percent", Status: "ok"}, ToneWarning},
		{"94.9", Slot{Available: true, Used: new(float64(94.9)), Unit: "percent", Status: "ok"}, ToneWarning},
		{"95", Slot{Available: true, Used: new(float64(95)), Unit: "percent", Status: "ok"}, ToneError},
		{"100", Slot{Available: true, Used: new(float64(100)), Unit: "percent", Status: "ok"}, ToneError},
		{"超過", Slot{Available: true, Used: new(float64(130)), Unit: "percent", Status: "ok"}, ToneError},
		{"exhausted は used に関わらず赤", Slot{Available: true, Used: new(float64(10)), Unit: "percent", Status: "exhausted"}, ToneError},
		{"exhausted で used なし", Slot{Available: true, Unit: "percent", Status: "exhausted"}, ToneError},
		{"usd は高額でもしきい値なし", Slot{Available: true, Used: new(float64(500)), Unit: "usd", Status: "ok"}, ToneNormal},
		{"usd でも exhausted は赤", Slot{Available: true, Used: new(float64(5)), Unit: "usd", Status: "exhausted"}, ToneError},
		{"unit なし", Slot{Available: true, Used: new(float64(99)), Status: "ok"}, ToneNormal},
		{"unit 未知", Slot{Available: true, Used: new(float64(99)), Unit: "tokens", Status: "ok"}, ToneNormal},
		{"used なし", Slot{Available: true, Unit: "percent", Status: "ok"}, ToneNormal},
		{"unavailable な枠", Slot{}, ToneNormal},
		{"unavailable で他が入っていても通常", Slot{Available: false, Used: new(float64(99)), Unit: "percent", Status: "exhausted"}, ToneNormal},
		{"status unknown でもしきい値は効く", Slot{Available: true, Used: new(float64(96)), Unit: "percent", Status: "unknown"}, ToneError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := UsageTone(tt.slot); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestPercentTone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   float64
		want Tone
	}{
		{-1, ToneNormal},
		{0, ToneNormal},
		{79.9, ToneNormal},
		{80, ToneWarning},
		{94.9, ToneWarning},
		{95, ToneError},
		{100, ToneError},
	}
	for _, tt := range tests {
		if got := PercentTone(tt.in); got != tt.want {
			t.Errorf("PercentTone(%v) = %s; want %s", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeUsageStatus(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"ok": "ok", "stale": "stale", "unavailable": "unavailable", "pending": "pending", "disabled": "disabled",
		"": "unavailable", "OK": "unavailable", "weird": "unavailable",
	} {
		if got := NormalizeUsageStatus(in); got != want {
			t.Errorf("NormalizeUsageStatus(%q) = %q; want %q", in, got, want)
		}
	}
}
