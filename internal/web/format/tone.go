// Package format は画面表示のための書式（ラベル・色・相対時間・数値）を持つ。
//
// 状態の表示ラベルと色、しきい値判定の結果から Tailwind クラスへの対応は presentation の関心事のため
// domain ではなくここに置く（docs/components/overview.md §6.3）。時刻を扱う関数は now を引数に取る。
package format

import "github.com/ousiassllc/swarm-viewer/internal/domain"

// Tone は表示色の意味。色だけに依存せず、必ずラベルを併記する（機能要件 §3）。
type Tone string

// Tone の値。
const (
	ToneAccent  Tone = "accent"
	ToneWarning Tone = "warning"
	ToneSuccess Tone = "success"
	ToneError   Tone = "error"
	ToneMuted   Tone = "muted"
)

// FromDomainTone は使用率の Tone（normal / warning / error）を表示色に対応づける。
// normal は accent。未知の値は muted にする。
func FromDomainTone(t domain.Tone) Tone {
	switch t {
	case domain.ToneNormal:
		return ToneAccent
	case domain.ToneWarning:
		return ToneWarning
	case domain.ToneError:
		return ToneError
	default:
		return ToneMuted
	}
}

// StateTone はペイン状態の表示色を返す（機能要件 §3）。
// working=accent、asking=warning、idle=success、stalled=error、shell・unknown・未知=muted。
func StateTone(s domain.PaneState) Tone {
	switch s {
	case domain.StateWorking:
		return ToneAccent
	case domain.StateAsking:
		return ToneWarning
	case domain.StateIdle:
		return ToneSuccess
	case domain.StateStalled:
		return ToneError
	default:
		return ToneMuted
	}
}

// PercentTone は使用率（%）の表示色を返す。しきい値は domain.PercentTone（80 / 95）に従う。
func PercentTone(percent float64) Tone {
	return FromDomainTone(domain.PercentTone(percent))
}

// SlotTone は利用上限の枠の表示色を返す。しきい値・exhausted の判定は domain.UsageTone に従う。
func SlotTone(slot domain.Slot) Tone {
	return FromDomainTone(domain.UsageTone(slot))
}

// BadgeClass はバッジ（背景・文字・リング）の Tailwind クラスを返す。未知の Tone は muted。
// 色はガイドのトークン（accent は無彩色の ink、状態色は success / warning / danger）。背景・リングはトークンの低 alpha。
func BadgeClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "bg-ink/10 text-ink ring-ink/30"
	case ToneWarning:
		return "bg-warning/15 text-warning ring-warning/40"
	case ToneSuccess:
		return "bg-success/15 text-success ring-success/40"
	case ToneError:
		return "bg-danger/15 text-danger ring-danger/40"
	default:
		return "bg-mute/15 text-mute ring-mute/40"
	}
}

// BarClass は <progress> の塗り部分（WebKit / Gecko 擬似要素）の Tailwind クラスを返す。
// 幅は value / max 属性で表し、style 属性は使わない（CSP の style-src 'self'）。未知の Tone は muted。
func BarClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "[&::-webkit-progress-value]:bg-accent [&::-moz-progress-bar]:bg-accent"
	case ToneWarning:
		return "[&::-webkit-progress-value]:bg-warning [&::-moz-progress-bar]:bg-warning"
	case ToneSuccess:
		return "[&::-webkit-progress-value]:bg-success [&::-moz-progress-bar]:bg-success"
	case ToneError:
		return "[&::-webkit-progress-value]:bg-danger [&::-moz-progress-bar]:bg-danger"
	default:
		return "[&::-webkit-progress-value]:bg-mute [&::-moz-progress-bar]:bg-mute"
	}
}

// TextClass は文字色の Tailwind クラスを返す。未知の Tone は muted。
func TextClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "text-ink"
	case ToneWarning:
		return "text-warning"
	case ToneSuccess:
		return "text-success"
	case ToneError:
		return "text-danger"
	default:
		return "text-mute"
	}
}
