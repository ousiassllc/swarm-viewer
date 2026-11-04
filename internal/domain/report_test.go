package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// swarm-console endpoints-report.md の JSON 例に未知キーを足した固定サンプル（NFR-QLT-1）。
const sampleReport = `{
  "at": "2026-10-31T22:10:05+09:00",
  "future_key": {"x": 1},
  "host": {
    "load1": 98.2, "mem_free_gb": 41.3,
    "cpu": {"available": true, "used_percent": 42},
    "swap": {"available": true, "total_gb": 8.0, "used_gb": 0.4, "used_percent": 5},
    "disk": [
      {"mount": "/", "available": true, "free_gb": 120, "total_gb": 450, "used_percent": 73},
      {"mount": "/mnt/nvme", "available": false, "free_gb": null, "total_gb": null, "used_percent": null}
    ]
  },
  "layout": {"rows": 2, "cols": [4, 4]},
  "panes": [
    {"pos": "T1", "id": "%1", "agent": "omp", "state": "working", "title": "t", "cwd": "/w",
     "elapsed": "42m", "activity": "Running go test", "truncated": true, "changed_ago": "20s",
     "note": {"from": "agent-b", "origin": "agent", "repo": "o/r", "items": ["#1", "#2"], "next": "n"}},
    {"pos": "T2", "id": "%2", "agent": null, "state": "asking", "title": "", "cwd": "", "elapsed": null,
     "activity": null, "changed_ago": null, "note": null},
    {"pos": "T3", "id": "%3", "agent": "newagent", "state": "brand-new-state"},
    {"pos": "T4", "id": "%4", "agent": "claude", "state": null},
    {"pos": "T5", "id": "%5", "agent": "codex", "state": 7},
    {"pos": "T6", "id": "%6", "agent": "codex"}
  ],
  "attention": [
    {"kind": "asking", "pane": "T2", "detail": "?"},
    {"kind": "auth_expired", "pane": "", "detail": "claude"},
    {"kind": "future_kind", "pane": "T1", "detail": "x"}
  ],
  "usage": {
    "status": "ok", "reason": null, "fetched_at": "2026-10-31T22:09:30+09:00",
    "claude": [
      {"account": "acct-a",
       "five_hour": {"available": false, "used": null, "unit": null, "status": null, "resets_at": null},
       "seven_day": {"available": true, "used": 48, "unit": "percent", "status": "ok", "resets_at": "2026-11-01T07:00:00+09:00"}}
    ],
    "cursor": [
      {"account": "acct-a", "window": "monthly",
       "models": {"available": true, "used": 0.0, "unit": "percent", "status": "ok", "resets_at": null}},
      {"account": "acct-b", "window": null,
       "models": {"available": true, "used": 12.5, "unit": "usd", "status": "exhausted", "resets_at": null}}
    ],
    "codex": [
      {"account": "acct-a",
       "five_hour": {"available": true, "used": 96.0, "unit": "percent", "status": "ok", "resets_at": null},
       "weekly": {"available": true, "used": 1.0, "unit": "percent", "status": "ok", "resets_at": null}}
    ]
  },
  "omp": {"status": "up_to_date"},
  "update": {"status": "up_to_date"},
  "auth": {"agents": [
    {"agent": "claude", "status": "expired", "expires_at": null},
    {"agent": "codex", "status": "expiring_soon", "expires_at": "2026-11-01T08:10:00+09:00"}
  ]},
  "warnings": [{"code": "layout_changed", "message": "m"}]
}`

func TestParseReportSample(t *testing.T) {
	t.Parallel()
	r, err := ParseReport([]byte(sampleReport))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 31, 13, 10, 5, 0, time.UTC); !r.At.Equal(want) {
		t.Errorf("At = %v", r.At)
	}
	if r.Layout.Rows != 2 || len(r.Layout.Cols) != 2 || !r.Layout.Valid(8) {
		t.Errorf("Layout = %+v", r.Layout)
	}
	if len(r.Panes) != 6 {
		t.Fatalf("panes = %d", len(r.Panes))
	}

	p := r.Panes[0]
	if p.Pos != "T1" || p.ID != "%1" || p.Agent == nil || *p.Agent != "omp" || p.State != StateWorking ||
		p.Title != "t" || p.CWD != "/w" || p.Elapsed != "42m" || p.Activity != "Running go test" ||
		!p.ActivityTruncated || p.ChangedAgo != "20s" {
		t.Errorf("pane0 = %+v", p)
	}
	if p.Note == nil || p.Note.Repo != "o/r" || len(p.Note.Items) != 2 || p.Note.From != "agent-b" || p.Note.Next != "n" {
		t.Errorf("note = %+v", p.Note)
	}

	// null・欠落は零値
	p = r.Panes[1]
	if p.State != StateAsking || p.Agent != nil || p.Note != nil || p.Elapsed != "" || p.Activity != "" || p.ChangedAgo != "" {
		t.Errorf("pane1 = %+v", p)
	}
	// 未知の agent はそのまま、未知・null・非文字列・欠落の state は unknown
	if r.Panes[2].Agent == nil || *r.Panes[2].Agent != "newagent" {
		t.Errorf("agent = %v", r.Panes[2].Agent)
	}
	for i := 2; i <= 5; i++ {
		if r.Panes[i].State != StateUnknown {
			t.Errorf("pane%d state = %q", i, r.Panes[i].State)
		}
	}

	if len(r.Attention) != 3 || r.Attention[2].Kind != "future_kind" || r.Attention[1].Pane != "" {
		t.Errorf("attention = %+v", r.Attention)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Code != "layout_changed" {
		t.Errorf("warnings = %+v", r.Warnings)
	}

	h := r.Host
	if !h.CPU.Available || h.CPU.UsedPercent != 42 || !h.Swap.Available || h.Swap.TotalGB != 8 || len(h.Disk) != 2 ||
		h.Disk[0].Mount != "/" || h.Disk[0].UsedPercent != 73 || h.Disk[1].Available || h.Disk[1].UsedPercent != 0 {
		t.Errorf("host = %+v", h)
	}

	u := r.Usage
	if u.Status != UsageOK || u.Reason != "" || u.FetchedAt == nil || len(u.Claude) != 1 || len(u.Cursor) != 2 || len(u.Codex) != 1 {
		t.Fatalf("usage = %+v", u)
	}
	c := u.Claude[0]
	if c.Account != "acct-a" || c.FiveHour.Available || c.FiveHour.Used != nil || c.FiveHour.ResetsAt != nil ||
		!c.SevenDay.Available || c.SevenDay.Used == nil || *c.SevenDay.Used != 48 || c.SevenDay.Unit != "percent" || c.SevenDay.ResetsAt == nil {
		t.Errorf("claude = %+v", c)
	}
	if u.Cursor[0].Window == nil || *u.Cursor[0].Window != "monthly" || u.Cursor[1].Window != nil ||
		u.Cursor[1].Models.Unit != "usd" || UsageTone(u.Cursor[1].Models) != ToneError {
		t.Errorf("cursor = %+v", u.Cursor)
	}
	// seven_day の代わりに weekly が来ても無視（枠は零値＝取得不可）
	if u.Codex[0].SevenDay.Available || UsageTone(u.Codex[0].FiveHour) != ToneError {
		t.Errorf("codex = %+v", u.Codex[0])
	}

	if len(r.Auth.Agents) != 2 || r.Auth.Agents[0].ExpiresAt != nil || r.Auth.Agents[1].ExpiresAt == nil ||
		r.Auth.Agents[1].Status != "expiring_soon" {
		t.Errorf("auth = %+v", r.Auth)
	}
}

func TestParseReportMissingAndNull(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"空オブジェクト": `{}`,
		"全部 null": `{"at":null,"layout":null,"panes":null,"attention":null,"usage":null,"host":null,"warnings":null,"auth":null}`,
		"null 要素": `{"panes":[null],"attention":[null],"warnings":[null]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := ParseReport([]byte(in))
			if err != nil {
				t.Fatal(err)
			}
			if !r.At.IsZero() || r.Layout.Valid(0) || r.Usage.Status != UsageUnavailable ||
				r.Host.CPU.Available || r.Usage.FetchedAt != nil {
				t.Errorf("zero value expected: %+v", r)
			}
			for _, p := range r.Panes {
				if p.State != StateUnknown {
					t.Errorf("null ペインの state = %q", p.State)
				}
			}
		})
	}
}

func TestParseReportUsageStatus(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		`"ok"`: UsageOK, `"stale"`: UsageStale, `"pending"`: UsagePending, `"disabled"`: UsageDisabled,
		`"unavailable"`: UsageUnavailable, `"future"`: UsageUnavailable, `null`: UsageUnavailable,
	} {
		r, err := ParseReport([]byte(`{"usage":{"status":` + in + `,"reason":"timeout","claude":[],"codex":[],"cursor":[]}}`))
		if err != nil {
			t.Fatal(err)
		}
		if r.Usage.Status != want || r.Usage.Reason != "timeout" {
			t.Errorf("status %s -> %q (reason %q); want %q", in, r.Usage.Status, r.Usage.Reason, want)
		}
	}
}

func TestParseReportAvailableFalseSlots(t *testing.T) {
	t.Parallel()
	const in = `{"usage":{"status":"ok","claude":[{"account":"a","five_hour":{"available":false,"used":null,"unit":null,"status":null,"resets_at":null}}],
	"codex":[{"account":"b"}],"cursor":[{"account":"c","window":null,"models":null}]},
	"host":{"cpu":{"available":false,"used_percent":null},"swap":{"available":false,"total_gb":null,"used_gb":null,"used_percent":null}}}`
	r, err := ParseReport([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []Slot{r.Usage.Claude[0].FiveHour, r.Usage.Claude[0].SevenDay, r.Usage.Codex[0].FiveHour, r.Usage.Cursor[0].Models} {
		if s.Available || UsageTone(s) != ToneNormal {
			t.Errorf("slot = %+v", s)
		}
	}
	if r.Host.CPU.Available || r.Host.Swap.Available {
		t.Errorf("host = %+v", r.Host)
	}
}

func TestParseReportAccountReason(t *testing.T) {
	t.Parallel()
	const in = `{"usage":{"status":"ok",
	"claude":[{"account":"a","reason":"without_usage"},{"account":"b","reason":null},{"account":"c"}],
	"codex":[{"account":"d","reason":"no_email"}],
	"cursor":[{"account":"e","reason":"x"}]}}`
	r, err := ParseReport([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Usage.Claude[0].Reason; got == nil || *got != "without_usage" {
		t.Errorf("claude[0].Reason = %v", got)
	}
	if r.Usage.Claude[1].Reason != nil || r.Usage.Claude[2].Reason != nil {
		t.Errorf("null・欠落（古い swarm-console）は nil: %v %v", r.Usage.Claude[1].Reason, r.Usage.Claude[2].Reason)
	}
	if got := r.Usage.Codex[0].Reason; got == nil || *got != "no_email" {
		t.Errorf("codex[0].Reason = %v", got)
	}
	if got := r.Usage.Cursor[0].Reason; got == nil || *got != "x" {
		t.Errorf("cursor[0].Reason = %v", got)
	}
}

func TestParseReportHostMem(t *testing.T) {
	t.Parallel()
	r, err := ParseReport([]byte(`{"host":{"mem":{"available":true,"total_gb":128,"used_gb":41.3,"used_percent":32.3}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Mem{Available: true, TotalGB: 128, UsedGB: 41.3, UsedPercent: 32.3}); r.Host.Mem != want {
		t.Errorf("mem = %+v, want %+v", r.Host.Mem, want)
	}

	// host.mem が無い古い swarm-console・null・available: false は Available が false。
	for name, in := range map[string]string{
		"欠落":    `{"host":{"cpu":{"available":true,"used_percent":1}}}`,
		"null":  `{"host":{"mem":null}}`,
		"無効":    `{"host":{"mem":{"available":false,"total_gb":null,"used_gb":null,"used_percent":null}}}`,
		"host なし": `{}`,
	} {
		r, err := ParseReport([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.Host.Mem != (Mem{}) {
			t.Errorf("%s: mem = %+v", name, r.Host.Mem)
		}
	}
}

func TestParseReportSwapWithoutSwap(t *testing.T) {
	t.Parallel()
	r, err := ParseReport([]byte(`{"host":{"swap":{"available":true,"total_gb":0,"used_gb":0,"used_percent":0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Host.Swap.Available || r.Host.Swap.TotalGB != 0 {
		t.Errorf("swap = %+v", r.Host.Swap)
	}
}

func TestParseReportInvalid(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"空":           ``,
		"空白":          `  `,
		"null":        `null`,
		"配列":          `[]`,
		"文字列":         `"x"`,
		"数値":          `1`,
		"壊れた JSON":    `{"panes":[`,
		"panes が型違い":  `{"panes":"x"}`,
		"at が型違い":     `{"at":"not-a-time"}`,
		"layout が型違い": `{"layout":[]}`,
		"テキスト":        `Error: boom`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseReport([]byte(in))
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("err = %v", err)
			}
			if len(in) > 8 && strings.Contains(err.Error(), in) {
				t.Fatalf("エラーに入力が含まれる: %v", err)
			}
		})
	}
}

func TestParsePaneState(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]PaneState{
		"working": StateWorking, "asking": StateAsking, "idle": StateIdle, "stalled": StateStalled,
		"shell": StateShell, "unknown": StateUnknown,
		"": StateUnknown, "Asking": StateUnknown, " asking": StateUnknown, "done": StateUnknown,
	} {
		if got := ParsePaneState(in); got != want {
			t.Errorf("ParsePaneState(%q) = %q; want %q", in, got, want)
		}
	}
	if !StateAsking.Notifiable() || !StateStalled.Notifiable() {
		t.Error("asking・stalled は通知対象")
	}
	for _, s := range []PaneState{StateWorking, StateIdle, StateShell, StateUnknown, ""} {
		if s.Notifiable() {
			t.Errorf("%q は通知対象外", s)
		}
	}
}

func TestLayoutValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		layout Layout
		panes  int
		want   bool
	}{
		{"2x3", Layout{Rows: 2, Cols: []int{3, 3}}, 6, true},
		{"不揃い", Layout{Rows: 2, Cols: []int{4, 2}}, 6, true},
		{"1 段", Layout{Rows: 1, Cols: []int{1}}, 1, true},
		{"ペイン数が多い", Layout{Rows: 2, Cols: []int{3, 3}}, 7, false},
		{"ペイン数が少ない", Layout{Rows: 2, Cols: []int{3, 3}}, 5, false},
		{"零値", Layout{}, 0, false},
		{"rows 0", Layout{Rows: 0, Cols: []int{}}, 0, false},
		{"rows と cols 長の不一致", Layout{Rows: 3, Cols: []int{3, 3}}, 6, false},
		{"cols が rows より長い", Layout{Rows: 1, Cols: []int{3, 3}}, 6, false},
		{"空の段", Layout{Rows: 2, Cols: []int{6, 0}}, 6, false},
		{"負の列", Layout{Rows: 2, Cols: []int{7, -1}}, 6, false},
		{"負の rows", Layout{Rows: -1, Cols: nil}, 0, false},
		{"cols nil", Layout{Rows: 1}, 0, false},
	}
	for _, tt := range tests {
		if got := tt.layout.Valid(tt.panes); got != tt.want {
			t.Errorf("%s: Valid(%d) = %v; want %v", tt.name, tt.panes, got, tt.want)
		}
	}
}
