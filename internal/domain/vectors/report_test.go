package vectors_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/ousiassllc/swarm-viewer/internal/domain"
)

const (
	minReportCases        = 20
	minReportInvalidCases = 10
	minSessionCases       = 3
	minSessionInvalid     = 8
	minLayoutCases        = 10
)

// normalizedReport は Report を testdata/vectors/README.md の「正規化した Report」にする。
func normalizedReport(r Report) map[string]any {
	panes := make([]any, 0, len(r.Panes))
	for _, p := range r.Panes {
		var note any
		if p.Note != nil {
			note = map[string]any{"from": p.Note.From, "repo": p.Note.Repo, "items": strs(p.Note.Items), "next": p.Note.Next}
		}
		panes = append(panes, map[string]any{
			"pos": p.Pos, "id": p.ID, "agent": ptr(p.Agent), "state": string(p.State), "title": p.Title, "cwd": p.CWD,
			"elapsed": p.Elapsed, "activity": p.Activity, "truncated": p.ActivityTruncated, "changed_ago": p.ChangedAgo, "note": note,
		})
	}
	attention := make([]any, 0, len(r.Attention))
	for _, a := range r.Attention {
		attention = append(attention, map[string]any{"kind": a.Kind, "pane": a.Pane, "detail": a.Detail})
	}
	warnings := make([]any, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		warnings = append(warnings, map[string]any{"code": w.Code, "message": w.Message})
	}
	disks := make([]any, 0, len(r.Host.Disk))
	for _, d := range r.Host.Disk {
		disks = append(disks, map[string]any{
			"mount": d.Mount, "available": d.Available, "free_gb": d.FreeGB, "total_gb": d.TotalGB, "used_percent": d.UsedPercent,
		})
	}
	agents := make([]any, 0, len(r.Auth.Agents))
	for _, a := range r.Auth.Agents {
		agents = append(agents, map[string]any{"agent": a.Agent, "status": a.Status, "expires_at": utc(a.ExpiresAt)})
	}
	return map[string]any{
		"at":        utc(&r.At),
		"layout":    map[string]any{"rows": r.Layout.Rows, "cols": ints(r.Layout.Cols)},
		"panes":     panes,
		"attention": attention,
		"warnings":  warnings,
		"host": map[string]any{
			"cpu":  map[string]any{"available": r.Host.CPU.Available, "used_percent": r.Host.CPU.UsedPercent},
			"mem":  memMap(r.Host.Mem.Available, r.Host.Mem.TotalGB, r.Host.Mem.UsedGB, r.Host.Mem.UsedPercent),
			"swap": memMap(r.Host.Swap.Available, r.Host.Swap.TotalGB, r.Host.Swap.UsedGB, r.Host.Swap.UsedPercent),
			"disk": disks,
		},
		"usage": usageMap(r.Usage),
		"auth":  map[string]any{"agents": agents},
	}
}

func usageMap(u Usage) map[string]any {
	accounts := func(in []AgentAccount) []any {
		out := make([]any, 0, len(in))
		for _, a := range in {
			out = append(out, map[string]any{
				"account": a.Account, "five_hour": slotMap(a.FiveHour), "seven_day": slotMap(a.SevenDay), "reason": ptr(a.Reason),
			})
		}
		return out
	}
	cursor := make([]any, 0, len(u.Cursor))
	for _, c := range u.Cursor {
		cursor = append(cursor, map[string]any{"account": c.Account, "window": ptr(c.Window), "models": slotMap(c.Models), "reason": ptr(c.Reason)})
	}
	return map[string]any{
		"status": u.Status, "reason": u.Reason, "fetched_at": utc(u.FetchedAt),
		"claude": accounts(u.Claude), "codex": accounts(u.Codex), "cursor": cursor,
	}
}

func slotMap(s Slot) map[string]any {
	var used any
	if s.Used != nil {
		used = *s.Used
	}
	return map[string]any{"available": s.Available, "used": used, "unit": s.Unit, "status": s.Status, "resets_at": utc(s.ResetsAt)}
}

func memMap(available bool, total, used, percent float64) map[string]any {
	return map[string]any{"available": available, "total_gb": total, "used_gb": used, "used_percent": percent}
}

func utc(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func ptr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func strs(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

func ints(in []int) []any {
	out := make([]any, 0, len(in))
	for _, n := range in {
		out = append(out, float64(n))
	}
	return out
}

// subsetMismatch は want が got の部分木でないとき、その場所と理由を返す（一致なら空）。
func subsetMismatch(path string, want, got any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return path + ": オブジェクトでない"
		}
		for k, wv := range w {
			gv, found := g[k]
			if !found {
				return path + "/" + k + ": キーが無い"
			}
			if m := subsetMismatch(path+"/"+k, wv, gv); m != "" {
				return m
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return path + ": 配列の長さが違う"
		}
		for i := range w {
			if m := subsetMismatch(path+"/"+strconv.Itoa(i), w[i], g[i]); m != "" {
				return m
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(want, got) {
			return path + ": " + toJSON(got) + " != " + toJSON(want)
		}
		return ""
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// toGeneric は JSON を any に読み直す（数値は float64）。
func toGeneric(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVectorsReport(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		ParseReport []struct {
			Name   string          `json:"name"`
			Input  json.RawMessage `json:"input"`
			Expect json.RawMessage `json:"expect"`
		} `json:"parse_report"`
		Invalid []struct {
			Name     string `json:"name"`
			InputRaw string `json:"input_raw"`
		} `json:"parse_report_invalid"`
		ParseSessions []struct {
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
			Want  []string        `json:"want"`
		} `json:"parse_sessions"`
		SessionsInvalid []struct {
			Name     string `json:"name"`
			InputRaw string `json:"input_raw"`
		} `json:"parse_sessions_invalid"`
		Layouts []struct {
			Name      string `json:"name"`
			Rows      int    `json:"rows"`
			Cols      []int  `json:"cols"`
			PaneCount int    `json:"pane_count"`
			Want      bool   `json:"want"`
		} `json:"layout_valid"`
		StateLabels []struct {
			State string `json:"state"`
			Label string `json:"label"`
		} `json:"state_labels"`
	}
	loadVectors(t, "report.json", &f)
	requireMinCases(t, "report.json の parse_report", len(f.ParseReport), minReportCases)
	requireMinCases(t, "report.json の parse_report_invalid", len(f.Invalid), minReportInvalidCases)
	requireMinCases(t, "report.json の parse_sessions", len(f.ParseSessions), minSessionCases)
	requireMinCases(t, "report.json の parse_sessions_invalid", len(f.SessionsInvalid), minSessionInvalid)
	requireMinCases(t, "report.json の layout_valid", len(f.Layouts), minLayoutCases)
	// state_labels は表示層の internal/web/format が検証する。ここでは件数の下限だけ確かめる。
	requireMinCases(t, "report.json の state_labels", len(f.StateLabels), minStateLabels)

	for _, c := range f.ParseReport {
		r, err := ParseReport(c.Input)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if m := subsetMismatch("", toGeneric(t, c.Expect), toGeneric(t, toJSONRaw(normalizedReport(r)))); m != "" {
			t.Errorf("%s: %s", c.Name, m)
		}
	}
	for _, c := range f.Invalid {
		_, err := ParseReport([]byte(c.InputRaw))
		assertInvalidResponse(t, c.Name, c.InputRaw, err)
	}
	for _, c := range f.ParseSessions {
		got, err := ParseSessions(c.Input)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		ids := make([]string, 0, len(got))
		for _, s := range got {
			if !s.Allowed {
				t.Errorf("%s: allowed でないセッションが残った: %+v", c.Name, s)
			}
			ids = append(ids, s.ID)
		}
		if !reflect.DeepEqual(ids, c.Want) || got == nil {
			t.Errorf("%s: ids = %v (nil=%v); want %v", c.Name, ids, got == nil, c.Want)
		}
	}
	for _, c := range f.SessionsInvalid {
		_, err := ParseSessions([]byte(c.InputRaw))
		assertInvalidResponse(t, c.Name, c.InputRaw, err)
	}
	for _, c := range f.Layouts {
		if got := (Layout{Rows: c.Rows, Cols: c.Cols}).Valid(c.PaneCount); got != c.Want {
			t.Errorf("%s: Valid(%d) = %v; want %v", c.Name, c.PaneCount, got, c.Want)
		}
	}
}

const minStateLabels = 9

func toJSONRaw(v any) json.RawMessage { return json.RawMessage(toJSON(v)) }

func assertInvalidResponse(t *testing.T, name, input string, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("%s: err = %v; want ErrInvalidResponse", name, err)
		return
	}
	if len(input) > 8 && strings.Contains(err.Error(), input) {
		t.Errorf("%s: エラーに入力が含まれる: %v", name, err)
	}
}
