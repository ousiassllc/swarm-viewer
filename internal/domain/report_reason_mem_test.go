package domain

import "testing"

// usage の reason と host.mem（Issue #10・#11）。

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
		"欠落":      `{"host":{"cpu":{"available":true,"used_percent":1}}}`,
		"null":    `{"host":{"mem":null}}`,
		"無効":      `{"host":{"mem":{"available":false,"total_gb":null,"used_gb":null,"used_percent":null}}}`,
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
