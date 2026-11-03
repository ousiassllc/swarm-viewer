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
func BadgeClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "bg-sky-500/15 text-sky-300 ring-sky-500/40"
	case ToneWarning:
		return "bg-amber-500/15 text-amber-300 ring-amber-500/40"
	case ToneSuccess:
		return "bg-emerald-500/15 text-emerald-300 ring-emerald-500/40"
	case ToneError:
		return "bg-red-500/15 text-red-300 ring-red-500/40"
	default:
		return "bg-slate-500/15 text-slate-300 ring-slate-500/40"
	}
}

// BarClass は <progress> の塗り部分（WebKit / Gecko 擬似要素）の Tailwind クラスを返す。
// 幅は value / max 属性で表し、style 属性は使わない（CSP の style-src 'self'）。未知の Tone は muted。
func BarClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "[&::-webkit-progress-value]:bg-sky-500 [&::-moz-progress-bar]:bg-sky-500"
	case ToneWarning:
		return "[&::-webkit-progress-value]:bg-amber-500 [&::-moz-progress-bar]:bg-amber-500"
	case ToneSuccess:
		return "[&::-webkit-progress-value]:bg-emerald-500 [&::-moz-progress-bar]:bg-emerald-500"
	case ToneError:
		return "[&::-webkit-progress-value]:bg-red-500 [&::-moz-progress-bar]:bg-red-500"
	default:
		return "[&::-webkit-progress-value]:bg-slate-500 [&::-moz-progress-bar]:bg-slate-500"
	}
}

// TextClass は文字色の Tailwind クラスを返す。未知の Tone は muted。
func TextClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "text-sky-400"
	case ToneWarning:
		return "text-amber-400"
	case ToneSuccess:
		return "text-emerald-400"
	case ToneError:
		return "text-red-400"
	default:
		return "text-slate-400"
	}
}

// BorderClass は枠線色の Tailwind クラスを返す。未知の Tone は muted。
func BorderClass(t Tone) string {
	switch t {
	case ToneAccent:
		return "border-sky-500/60"
	case ToneWarning:
		return "border-amber-500/60"
	case ToneSuccess:
		return "border-emerald-500/60"
	case ToneError:
		return "border-red-500/60"
	default:
		return "border-slate-600"
	}
}
