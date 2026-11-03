package domain

import (
	"reflect"
	"testing"
)

func pn(id string, s PaneState) Pane { return Pane{Pos: "T" + id, ID: id, State: s} }

func key(id string, s PaneState) NotifiedKey { return NotifiedKey{PaneID: id, State: s} }

func events(keys ...NotifiedKey) []NotifyEvent {
	var out []NotifyEvent
	for _, k := range keys {
		out = append(out, NotifyEvent{Pane: pn(k.PaneID, k.State), State: k.State})
	}
	return out
}

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		known      []NotifiedKey
		panes      []Pane
		baselining bool
		wantNotify []NotifyEvent
		wantInsert []NotifiedKey
		wantDelete []NotifiedKey
	}{
		// ER §3.4 の各行
		{
			name:       "asking で行なし: Insert し通知する",
			panes:      []Pane{pn("%1", StateAsking)},
			wantNotify: events(key("%1", StateAsking)),
			wantInsert: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "stalled で行なし: Insert し通知する",
			panes:      []Pane{pn("%1", StateStalled)},
			wantNotify: events(key("%1", StateStalled)),
			wantInsert: []NotifiedKey{key("%1", StateStalled)},
		},
		{
			name:  "asking → asking: 何もしない（再通知なし）",
			known: []NotifiedKey{key("%1", StateAsking)},
			panes: []Pane{pn("%1", StateAsking)},
		},
		{
			name:  "stalled → stalled: 何もしない",
			known: []NotifiedKey{key("%1", StateStalled)},
			panes: []Pane{pn("%1", StateStalled)},
		},
		{
			name:       "通知対象外へ離脱: 行をすべて Delete",
			known:      []NotifiedKey{key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateWorking)},
			wantDelete: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "離脱後の再突入は再通知: 離脱で Delete、次の観測で Insert・通知",
			known:      nil, // 離脱の Delete 適用後
			panes:      []Pane{pn("%1", StateAsking)},
			wantNotify: events(key("%1", StateAsking)),
			wantInsert: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "asking → stalled: 別状態の行を Delete し、新状態を Insert・通知",
			known:      []NotifiedKey{key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateStalled)},
			wantNotify: events(key("%1", StateStalled)),
			wantInsert: []NotifiedKey{key("%1", StateStalled)},
			wantDelete: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "stalled → asking も通知",
			known:      []NotifiedKey{key("%1", StateStalled)},
			panes:      []Pane{pn("%1", StateAsking)},
			wantNotify: events(key("%1", StateAsking)),
			wantInsert: []NotifiedKey{key("%1", StateAsking)},
			wantDelete: []NotifiedKey{key("%1", StateStalled)},
		},
		{
			name:       "ペインが消えた: Delete・通知なし",
			known:      []NotifiedKey{key("%1", StateAsking), key("%2", StateStalled)},
			panes:      []Pane{pn("%2", StateStalled)},
			wantDelete: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "panes が空: 全行 Delete",
			known:      []NotifiedKey{key("%1", StateAsking), key("%2", StateStalled)},
			wantDelete: []NotifiedKey{key("%1", StateAsking), key("%2", StateStalled)},
		},
		{
			name:  "known・panes ともに空",
			panes: nil,
		},
		// 通知対象外の状態
		{
			name:  "idle / shell / unknown / working は記録も通知もしない",
			panes: []Pane{pn("%1", StateIdle), pn("%2", StateShell), pn("%3", StateUnknown), pn("%4", StateWorking)},
		},
		{
			name:       "unknown へ離脱も Delete",
			known:      []NotifiedKey{key("%1", StateStalled)},
			panes:      []Pane{pn("%1", StateUnknown)},
			wantDelete: []NotifiedKey{key("%1", StateStalled)},
		},
		// baselining
		{
			name:       "baselining: Insert のみで通知しない",
			panes:      []Pane{pn("%1", StateAsking), pn("%2", StateStalled), pn("%3", StateWorking)},
			baselining: true,
			wantInsert: []NotifiedKey{key("%1", StateAsking), key("%2", StateStalled)},
		},
		{
			name:       "baselining: 状態変化も通知せず Insert・Delete",
			known:      []NotifiedKey{key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateStalled)},
			baselining: true,
			wantInsert: []NotifiedKey{key("%1", StateStalled)},
			wantDelete: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "baselining: 記録済みは何もしない",
			known:      []NotifiedKey{key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateAsking)},
			baselining: true,
		},
		// 複合・エッジ
		{
			name:       "複数ペインの混在",
			known:      []NotifiedKey{key("%1", StateAsking), key("%2", StateAsking), key("%3", StateStalled)},
			panes:      []Pane{pn("%1", StateAsking), pn("%2", StateIdle), pn("%4", StateStalled), pn("%3", StateAsking)},
			wantNotify: events(key("%4", StateStalled), key("%3", StateAsking)),
			wantInsert: []NotifiedKey{key("%4", StateStalled), key("%3", StateAsking)},
			wantDelete: []NotifiedKey{key("%2", StateAsking), key("%3", StateStalled)},
		},
		{
			name:  "ID が空のペインは無視",
			panes: []Pane{pn("", StateAsking)},
		},
		{
			name:       "同じ ID が重複したら最初を採用",
			panes:      []Pane{pn("%1", StateAsking), pn("%1", StateStalled)},
			wantNotify: events(key("%1", StateAsking)),
			wantInsert: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "known の重複は Delete も 1 回",
			known:      []NotifiedKey{key("%1", StateAsking), key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateIdle)},
			wantDelete: []NotifiedKey{key("%1", StateAsking)},
		},
		{
			name:       "known に通知対象外の不正な行があれば Delete",
			known:      []NotifiedKey{key("%1", StateIdle), key("%1", StateAsking)},
			panes:      []Pane{pn("%1", StateAsking)},
			wantDelete: []NotifiedKey{key("%1", StateIdle)},
		},
		{
			name:  "pos が変わっても ID が同じなら再通知しない",
			known: []NotifiedKey{key("%1", StateAsking)},
			panes: []Pane{{Pos: "B3", ID: "%1", State: StateAsking}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			knownBefore := append([]NotifiedKey(nil), tt.known...)
			panesBefore := append([]Pane(nil), tt.panes...)

			got := Detect(tt.known, tt.panes, tt.baselining)

			want := Diff{Notify: tt.wantNotify, Insert: tt.wantInsert, Delete: tt.wantDelete}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got  %+v\nwant %+v", got, want)
			}
			if !reflect.DeepEqual(append([]NotifiedKey(nil), tt.known...), knownBefore) ||
				!reflect.DeepEqual(append([]Pane(nil), tt.panes...), panesBefore) {
				t.Fatal("入力を変更してはならない")
			}
		})
	}
}

// 通知対象への遷移のサイクル: Diff を known に適用して次の観測へ渡し、通知が 1 回ずつになることを確認する。
func TestDetectCycle(t *testing.T) {
	t.Parallel()
	apply := func(known []NotifiedKey, d Diff) []NotifiedKey {
		del := map[NotifiedKey]bool{}
		for _, k := range d.Delete {
			del[k] = true
		}
		var out []NotifiedKey
		for _, k := range known {
			if !del[k] {
				out = append(out, k)
			}
		}
		return append(out, d.Insert...)
	}
	steps := []struct {
		state      PaneState
		wantNotify int
	}{
		{StateAsking, 1},  // 初回遷移
		{StateAsking, 0},  // 継続
		{StateAsking, 0},  // 継続（再起動後も記録が残る想定）
		{StateWorking, 0}, // 離脱
		{StateAsking, 1},  // 再突入
		{StateStalled, 1}, // asking → stalled
		{StateStalled, 0},
		{StateIdle, 0},
		{StateStalled, 1},
	}
	var known []NotifiedKey
	for i, s := range steps {
		d := Detect(known, []Pane{pn("%1", s.state)}, false)
		if len(d.Notify) != s.wantNotify {
			t.Fatalf("step %d (%s): notify = %d; want %d", i, s.state, len(d.Notify), s.wantNotify)
		}
		known = apply(known, d)
	}
}
