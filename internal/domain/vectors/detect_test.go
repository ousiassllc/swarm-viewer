package vectors_test

import (
	"reflect"
	"testing"

	. "github.com/ousiassllc/swarm-viewer/internal/domain"
)

const (
	minDetectCases = 20
	minCycleCases  = 2
)

type vectorKey struct {
	PaneID string `json:"pane_id"`
	State  string `json:"state"`
}

func (k vectorKey) key() NotifiedKey {
	return NotifiedKey{PaneID: k.PaneID, State: ParsePaneState(k.State)}
}

func vectorKeys(in []vectorKey) []NotifiedKey {
	var out []NotifiedKey
	for _, k := range in {
		out = append(out, k.key())
	}
	return out
}

func TestVectorsDetect(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		Cases       []struct {
			Name  string      `json:"name"`
			Known []vectorKey `json:"known"`
			Panes []struct {
				ID    string `json:"id"`
				Pos   string `json:"pos"`
				State string `json:"state"`
			} `json:"panes"`
			Baselining bool        `json:"baselining"`
			Notify     []vectorKey `json:"notify"`
			Insert     []vectorKey `json:"insert"`
			Delete     []vectorKey `json:"delete"`
		}
		Cycles []struct {
			Name  string `json:"name"`
			Steps []struct {
				State  string `json:"state"`
				Notify int    `json:"notify"`
			} `json:"steps"`
		} `json:"detect_cycles"`
	}
	loadVectors(t, "detect.json", &f)
	requireMinCases(t, "detect.json の cases", len(f.Cases), minDetectCases)
	requireMinCases(t, "detect.json の detect_cycles", len(f.Cycles), minCycleCases)

	for _, c := range f.Cases {
		known := vectorKeys(c.Known)
		var panes []Pane
		for _, p := range c.Panes {
			panes = append(panes, Pane{ID: p.ID, Pos: p.Pos, State: ParsePaneState(p.State)})
		}
		knownBefore := append([]NotifiedKey(nil), known...)
		panesBefore := append([]Pane(nil), panes...)

		got := Detect(known, panes, c.Baselining)

		var notified []NotifiedKey
		for _, e := range got.Notify {
			if e.State != e.Pane.State {
				t.Errorf("%s: 通知の state と pane.state が違う: %+v", c.Name, e)
			}
			notified = append(notified, NotifiedKey{PaneID: e.Pane.ID, State: e.State})
		}
		if !reflect.DeepEqual(notified, vectorKeys(c.Notify)) {
			t.Errorf("%s: notify = %v; want %v", c.Name, notified, vectorKeys(c.Notify))
		}
		if !reflect.DeepEqual(got.Insert, vectorKeys(c.Insert)) {
			t.Errorf("%s: insert = %v; want %v", c.Name, got.Insert, vectorKeys(c.Insert))
		}
		if !reflect.DeepEqual(got.Delete, vectorKeys(c.Delete)) {
			t.Errorf("%s: delete = %v; want %v", c.Name, got.Delete, vectorKeys(c.Delete))
		}
		if !reflect.DeepEqual(append([]NotifiedKey(nil), known...), knownBefore) ||
			!reflect.DeepEqual(append([]Pane(nil), panes...), panesBefore) {
			t.Errorf("%s: 入力を変更してはならない", c.Name)
		}
	}

	for _, c := range f.Cycles {
		var known []NotifiedKey
		for i, s := range c.Steps {
			d := Detect(known, []Pane{{ID: "%1", Pos: "T1", State: ParsePaneState(s.State)}}, false)
			if len(d.Notify) != s.Notify {
				t.Fatalf("%s: step %d (%s): notify = %d; want %d", c.Name, i, s.State, len(d.Notify), s.Notify)
			}
			known = applyDiff(known, d)
		}
	}
}

// applyDiff は Diff を通知済み記録に適用する（delete を除き、insert を足す）。
func applyDiff(known []NotifiedKey, d Diff) []NotifiedKey {
	deleted := map[NotifiedKey]bool{}
	for _, k := range d.Delete {
		deleted[k] = true
	}
	var out []NotifiedKey
	for _, k := range known {
		if !deleted[k] {
			out = append(out, k)
		}
	}
	return append(out, d.Insert...)
}
