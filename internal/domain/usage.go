package domain

import "time"

// usage.status の値。これ以外は UsageUnavailable として扱う。
const (
	UsageOK          = "ok"
	UsageStale       = "stale"
	UsageUnavailable = "unavailable"
	UsagePending     = "pending"
	UsageDisabled    = "disabled"
)

// 枠の単位（Slot.Unit）と状態（Slot.Status）。
const (
	UnitPercent = "percent"
	UnitUSD     = "usd"

	SlotStatusExhausted = "exhausted"
)

// 利用上限のしきい値（%）。固定値で設定項目にしない（機能要件 §5.3）。
const (
	WarnPercent     = 80.0
	CriticalPercent = 95.0
)

// NormalizeUsageStatus は未知・空の usage.status を UsageUnavailable にする。
func NormalizeUsageStatus(s string) string {
	switch s {
	case UsageOK, UsageStale, UsageUnavailable, UsagePending, UsageDisabled:
		return s
	default:
		return UsageUnavailable
	}
}

// Slot は利用上限の 1 枠。Available が false のとき、ほかの項目は零値（表示は「—」）。
type Slot struct {
	Available bool       `json:"available"`
	Used      *float64   `json:"used"`
	Unit      string     `json:"unit"`
	Status    string     `json:"status"`
	ResetsAt  *time.Time `json:"resets_at"`
}

// AgentAccount は Claude / Codex のアカウント 1 件。
// Reason は枠を取れなかった理由のコード（without_usage・no_email など）。無い古い swarm-console では nil。
type AgentAccount struct {
	Account  string  `json:"account"`
	FiveHour Slot    `json:"five_hour"`
	SevenDay Slot    `json:"seven_day"`
	Reason   *string `json:"reason"`
}

// CursorAccount は Cursor のアカウント 1 件。Window は monthly・weekly など。枠が無ければ nil。
// Reason は AgentAccount と同じ。viewer は Cursor では使わない（枠が 1 つで「—」で足りるため）。
type CursorAccount struct {
	Account string  `json:"account"`
	Window  *string `json:"window"`
	Models  Slot    `json:"models"`
	Reason  *string `json:"reason"`
}

// Usage は利用上限パネルの値（ホスト全体）。
type Usage struct {
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	FetchedAt *time.Time      `json:"fetched_at"`
	Claude    []AgentAccount  `json:"claude"`
	Codex     []AgentAccount  `json:"codex"`
	Cursor    []CursorAccount `json:"cursor"`
}

// Tone は使用率の色の意味。実際の色（Tailwind クラス）は web 層が決める。
type Tone string

// Tone の値。
const (
	ToneNormal  Tone = "normal"
	ToneWarning Tone = "warning"
	ToneError   Tone = "error"
)

// PercentTone は使用率（%）のしきい値判定。80 未満は通常、80 以上 95 未満は警告、95 以上はエラー。
// NaN は通常として扱う。ホスト負荷（CPU・スワップ・ディスク）にも同じ規則を使う。
func PercentTone(percent float64) Tone {
	switch {
	case percent >= CriticalPercent:
		return ToneError
	case percent >= WarnPercent:
		return ToneWarning
	default:
		return ToneNormal
	}
}

// UsageTone は枠の色を返す。
//   - 取得できない枠（Available が false）は ToneNormal
//   - Status が exhausted なら Used・Unit に関わらず ToneError
//   - Unit が percent で Used があるときだけしきい値で判定する。usd などはしきい値なし
func UsageTone(slot Slot) Tone {
	if !slot.Available {
		return ToneNormal
	}
	if slot.Status == SlotStatusExhausted {
		return ToneError
	}
	if slot.Unit != UnitPercent || slot.Used == nil {
		return ToneNormal
	}
	return PercentTone(*slot.Used)
}
