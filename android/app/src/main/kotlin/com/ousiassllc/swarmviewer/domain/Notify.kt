package com.ousiassllc.swarmviewer.domain

/** 通知済みの `(ペイン ID, 状態)`。接続・セッションの軸は呼び出し側が持つ。 */
data class NotifiedKey(
    val paneId: String,
    val state: PaneState,
)

/** 通知する遷移 1 件。 */
data class NotifyEvent(
    val pane: Pane,
    val state: PaneState,
)

/** [detect] の結果。[notify] は OS 通知する遷移、[insert]・[delete] は通知済み状態（notified_states）への操作。 */
data class Diff(
    val notify: List<NotifyEvent> = emptyList(),
    val insert: List<NotifiedKey> = emptyList(),
    val delete: List<NotifiedKey> = emptyList(),
)

/**
 * 通知済み記録 [known] と、取得に成功したセッションの [panes] から通知判定を行う（ER §3.4 の更新規則）。
 * 純粋関数で、引数は変更しない。
 *
 * ペイン P（ID 単位）、現在の状態 S について:
 *  - S が asking / stalled で (P, S) が known に無い → insert し、notify（[baselining] のときは notify しない）
 *  - S が asking / stalled で (P, S) が known にある → 何もしない
 *  - S が asking / stalled で P の別の状態の行がある → その行を delete（S が新規なら insert・notify）
 *  - S が通知対象外 → P の行をすべて delete。通知しない
 *  - known にあるが panes に無い P → その行を delete。通知しない
 *
 * 取得に失敗したときは呼ばないこと（呼び出し側が記録を変更しない）。
 * ID が空のペインは追跡できないため無視する。同じ ID が複数あれば最初のものを使う。
 * 結果の順序は notify・insert が panes の順、delete が known の順。
 * デスクトップ版の Detect と同じ結果になる（testdata/vectors/detect.json）。
 */
fun detect(
    known: List<NotifiedKey>,
    panes: List<Pane>,
    baselining: Boolean,
): Diff {
    val recorded = known.toSet()
    val notify = ArrayList<NotifyEvent>()
    val insert = ArrayList<NotifiedKey>()
    val now = HashMap<String, PaneState>(panes.size) // ペイン ID → 現在の状態（最初の出現）
    for (pane in panes) {
        if (pane.id.isEmpty() || pane.id in now) continue
        now[pane.id] = pane.state
        val key = NotifiedKey(pane.id, pane.state)
        if (!pane.state.notifiable || key in recorded) continue // 通知対象外の行は下の known 走査で delete される
        insert.add(key)
        if (!baselining) notify.add(NotifyEvent(pane, pane.state))
    }

    val seen = HashSet<NotifiedKey>(known.size)
    val delete = ArrayList<NotifiedKey>()
    for (key in known) {
        if (!seen.add(key)) continue
        val current = now[key.paneId]
        if (current != null && current.notifiable && current == key.state) continue // 現在も同じ状態。記録を維持する
        delete.add(key)
    }
    return Diff(notify, insert, delete)
}
