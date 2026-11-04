// Package domain は swarm-viewer の純粋なドメインモデルとロジックを持つ。
//
// 他層に依存せず、副作用（ネットワーク・ファイル・時計・ログ出力）を持たない。
// 標準ライブラリだけを使う。
package domain

import (
	"bytes"
	"encoding/json"
	"time"
)

// PaneState はペインの状態（swarm-console の `panes[].state`）。
type PaneState string

// ペインの状態。未知の値は StateUnknown に正規化する。
const (
	StateWorking PaneState = "working"
	StateAsking  PaneState = "asking"
	StateIdle    PaneState = "idle"
	StateStalled PaneState = "stalled"
	StateShell   PaneState = "shell"
	StateUnknown PaneState = "unknown"
)

// ParsePaneState は文字列を PaneState に変換する。既知の 6 値以外（空文字を含む）は StateUnknown。
func ParsePaneState(s string) PaneState {
	switch p := PaneState(s); p {
	case StateWorking, StateAsking, StateIdle, StateStalled, StateShell:
		return p
	default:
		return StateUnknown
	}
}

// Notifiable は通知対象の状態（asking / stalled）かを返す。
func (s PaneState) Notifiable() bool {
	return s == StateAsking || s == StateStalled
}

// UnmarshalJSON は未知の値・文字列以外・null を StateUnknown に正規化し、エラーにしない（NFR-CMP-1・2）。
func (s *PaneState) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		*s = StateUnknown
		return nil
	}
	*s = ParsePaneState(raw)
	return nil
}

// Note はペインのメモ要約。viewer は Repo と Items だけ表示する。
type Note struct {
	From  string   `json:"from"`
	Repo  string   `json:"repo"`
	Items []string `json:"items"`
	Next  string   `json:"next"`
}

// Pane はペイン 1 つの状態。null・欠落の文字列項目は空文字になる。
type Pane struct {
	Pos               string    `json:"pos"`
	ID                string    `json:"id"` // tmux ペイン ID（例 %12）。通知の同一性判定に使う
	Agent             *string   `json:"agent"`
	State             PaneState `json:"state"`
	Title             string    `json:"title"`
	CWD               string    `json:"cwd"`
	Elapsed           string    `json:"elapsed"`
	Activity          string    `json:"activity"`
	ActivityTruncated bool      `json:"truncated"`
	ChangedAgo        string    `json:"changed_ago"`
	Note              *Note     `json:"note"`
}

// Layout はペイングリッドの形。Cols は各段の列数。
type Layout struct {
	Rows int   `json:"rows"`
	Cols []int `json:"cols"`
}

// Valid は paneCount 個のペインがこのレイアウトのグリッドとして成立するかを返す。
// 段数が 1 以上、Cols の長さが Rows と一致、各段の列数が 1 以上、列数の合計が paneCount と一致するとき true。
func (l Layout) Valid(paneCount int) bool {
	if l.Rows < 1 || len(l.Cols) != l.Rows {
		return false
	}
	total := 0
	for _, c := range l.Cols {
		if c < 1 {
			return false
		}
		total += c
	}
	return total == paneCount
}

// Attention は要対応の 1 件。Kind は未知の値もそのまま保持する。
type Attention struct {
	Kind   string `json:"kind"`
	Pane   string `json:"pane"`
	Detail string `json:"detail"`
}

// Warning は警告 1 件。
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CPU は host.cpu。Available が false のとき UsedPercent は 0。
type CPU struct {
	Available   bool    `json:"available"`
	UsedPercent float64 `json:"used_percent"`
}

// Mem は host.mem。Swap と同じ形。host.mem を返さない古い swarm-console では零値（Available が false）。
type Mem struct {
	Available   bool    `json:"available"`
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	UsedPercent float64 `json:"used_percent"`
}

// Swap は host.swap。TotalGB が 0 でも Available なら「スワップ無し」。
type Swap struct {
	Available   bool    `json:"available"`
	TotalGB     float64 `json:"total_gb"`
	UsedGB      float64 `json:"used_gb"`
	UsedPercent float64 `json:"used_percent"`
}

// Disk は host.disk[] の 1 マウント。
type Disk struct {
	Mount       string  `json:"mount"`
	Available   bool    `json:"available"`
	FreeGB      float64 `json:"free_gb"`
	TotalGB     float64 `json:"total_gb"`
	UsedPercent float64 `json:"used_percent"`
}

// Host はホスト全体の負荷。load1・mem_free_gb は使わない（メモリは Mem の使用率を使う）。
type Host struct {
	CPU  CPU    `json:"cpu"`
	Mem  Mem    `json:"mem"`
	Swap Swap   `json:"swap"`
	Disk []Disk `json:"disk"`
}

// AuthAgent は auth.agents[] の 1 件。ExpiresAt は不明のとき nil。
type AuthAgent struct {
	Agent     string     `json:"agent"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// Auth は report の auth。
type Auth struct {
	Agents []AuthAgent `json:"agents"`
}

// Report は swarm-console `report`（format: json）から viewer が使う項目だけを持つ。
// スナップショット（snapshots.report_json）の JSON 形式でもある。
type Report struct {
	At        time.Time   `json:"at"`
	Layout    Layout      `json:"layout"`
	Panes     []Pane      `json:"panes"`
	Attention []Attention `json:"attention"`
	Usage     Usage       `json:"usage"`
	Host      Host        `json:"host"`
	Warnings  []Warning   `json:"warnings"`
	Auth      Auth        `json:"auth"`
}

// Snapshot は取得済みの Report と、viewer が取得に成功した時刻。Stale は古い値を再表示しているとき true。
type Snapshot struct {
	Report    Report
	FetchedAt time.Time
	Stale     bool
}

// ParseReport は `report` 応答（JSON）を Report に変換する。
// 未知のキーは無視し、欠落・null は零値、未知の state は StateUnknown、未知の usage.status は unavailable にする。
// JSON として壊れている・オブジェクトでない・型が想定外のときは ErrInvalidResponse を返す。
// エラーに応答本文は含めない（NFR-LOG-3）。
func ParseReport(data []byte) (Report, error) {
	var r Report
	if !isJSONObject(data) || json.Unmarshal(data, &r) != nil {
		return Report{}, ErrInvalidResponse
	}
	for i := range r.Panes {
		// キー欠落の state は零値の空文字。UnmarshalJSON を通らないため正規化する。
		r.Panes[i].State = ParsePaneState(string(r.Panes[i].State))
	}
	r.Usage.Status = NormalizeUsageStatus(r.Usage.Status)
	return r, nil
}

func isJSONObject(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("{"))
}
