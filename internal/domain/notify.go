package domain

// NotifiedKey は通知済みの `(ペイン ID, 状態)`。接続・セッションの軸は呼び出し側（service）が持つ。
type NotifiedKey struct {
	PaneID string
	State  PaneState
}

// NotifyEvent は通知する遷移 1 件。
type NotifyEvent struct {
	Pane  Pane
	State PaneState
}

// Diff は Detect の結果。Notify は OS 通知する遷移、Insert・Delete は通知済み記録（notified_states）への操作。
type Diff struct {
	Notify []NotifyEvent
	Insert []NotifiedKey
	Delete []NotifiedKey
}

// Detect は通知済み記録 known と、取得に成功したセッションの panes から通知判定を行う（ER §3.4 の更新規則）。
// 純粋関数で、known・panes は変更しない。
//
// ペイン P（ID 単位）、現在の状態 S について:
//   - S が asking / stalled で (P, S) が known に無い → Insert し、Notify（baselining のときは Notify しない）
//   - S が asking / stalled で (P, S) が known にある → 何もしない
//   - S が asking / stalled で P の別の状態の行がある → その行を Delete（S が新規なら Insert・Notify）
//   - S が通知対象外 → P の行をすべて Delete。通知しない
//   - known にあるが panes に無い P → その行を Delete。通知しない
//
// 取得に失敗したときは Detect を呼ばないこと（呼び出し側が記録を変更しない）。
// ID が空のペインは追跡できないため無視する。同じ ID が複数あれば最初のものを使う。
// 結果の順序は Notify・Insert が panes の順、Delete が known の順。操作が無いスライスは nil。
func Detect(known []NotifiedKey, panes []Pane, baselining bool) Diff {
	recorded := make(map[NotifiedKey]struct{}, len(known))
	for _, k := range known {
		recorded[k] = struct{}{}
	}

	var d Diff
	now := make(map[string]PaneState, len(panes)) // ペイン ID → 現在の状態（最初の出現）
	for _, p := range panes {
		if _, dup := now[p.ID]; dup || p.ID == "" {
			continue
		}
		now[p.ID] = p.State
		key := NotifiedKey{PaneID: p.ID, State: p.State}
		if !p.State.Notifiable() {
			continue // 行は下の known 走査で Delete される
		}
		if _, ok := recorded[key]; ok {
			continue
		}
		d.Insert = append(d.Insert, key)
		if !baselining {
			d.Notify = append(d.Notify, NotifyEvent{Pane: p, State: p.State})
		}
	}

	seen := make(map[NotifiedKey]struct{}, len(known))
	for _, k := range known {
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if s, ok := now[k.PaneID]; ok && s.Notifiable() && s == k.State {
			continue // 現在も同じ状態。記録を維持する
		}
		d.Delete = append(d.Delete, k)
	}
	return d
}
