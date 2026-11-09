package format

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// Dash は利用できない値（available: false）の表示。
const Dash = "—"

// StateLabel はペイン状態の表示ラベルを返す（機能要件 §3）。未知の値は unknown と同じ「判定できない」。
func StateLabel(s domain.PaneState) string {
	switch s {
	case domain.StateWorking:
		return "作業中"
	case domain.StateAsking:
		return "答え待ち"
	case domain.StateIdle:
		return "指示待ち"
	case domain.StateStalled:
		return "止まってるかも"
	case domain.StateShell:
		return "空き"
	default:
		return "判定できない"
	}
}

// Percent は使用率を表示用にする。小数第 1 位まで、末尾の 0 は落とす（80 → "80%"、79.9 → "79.9%"）。
// NaN・無限大は Dash。
func Percent(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Dash
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + "%"
}

// USD は金額を「$12.50」の形にする。NaN・無限大は Dash。
func USD(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Dash
	}
	return "$" + strconv.FormatFloat(v, 'f', 2, 64)
}

// BarValue は <progress> の value に渡す値（0〜100 に丸めた使用率）を返す。NaN は 0。
func BarValue(percent float64) float64 {
	if math.IsNaN(percent) || percent < 0 {
		return 0
	}
	return math.Min(percent, 100)
}

// GB は容量（GB）を小数第 1 位で「12.3 GB」の形にする。
func GB(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Dash
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + " GB"
}

// ClockTime は時刻をローカル時刻の「HH:MM」にする。零値は Dash。
func ClockTime(t time.Time) string {
	if t.IsZero() {
		return Dash
	}
	return t.Local().Format("15:04")
}

// DateTime は時刻をローカル時刻の「MM/DD HH:MM」にする。零値は Dash。
func DateTime(t time.Time) string {
	if t.IsZero() {
		return Dash
	}
	return t.Local().Format("01/02 15:04")
}

// DateTimeWithYear は時刻をローカル時刻の「YYYY/MM/DD HH:MM」にする。零値は Dash。
// 年をまたぐ期限を誤読させたくない箇所（ログイン期限）で使う。
func DateTimeWithYear(t time.Time) string {
	if t.IsZero() {
		return Dash
	}
	return t.Local().Format("2006/01/02 15:04")
}

// Until は t までの残り時間を「あと N 分／時間／日」の形にする（機能要件 §5.2）。
// 1 分未満・過去は「まもなく」。端数は切り捨てる。now は呼び出し側が渡す（テストで固定できる）。
func Until(t, now time.Time) string {
	d := t.Sub(now)
	switch {
	case d < time.Minute:
		return "まもなく"
	case d < time.Hour:
		return "あと " + strconv.Itoa(int(d/time.Minute)) + " 分"
	case d < 24*time.Hour:
		return "あと " + strconv.Itoa(int(d/time.Hour)) + " 時間"
	default:
		return "あと " + strconv.Itoa(int(d/(24*time.Hour))) + " 日"
	}
}

// AttentionLabel は attention.kind の表示ラベルを返す（機能要件 §7）。未知の kind は文字列のまま返す。
func AttentionLabel(kind string) string {
	switch kind {
	case "asking":
		return "答え待ち"
	case "stalled":
		return "止まってるかも"
	case "report_waiting":
		return "報告待ち"
	case "auth_expired":
		return "ログイン期限切れ"
	case "auth_expiring":
		return "ログイン期限間近"
	default:
		return kind
	}
}

// AttentionTone は attention.kind の表示色を返す。未知の kind は muted。
func AttentionTone(kind string) Tone {
	switch kind {
	case "asking", "auth_expiring":
		return ToneWarning
	case "stalled", "auth_expired":
		return ToneError
	case "report_waiting":
		return ToneAccent
	default:
		return ToneMuted
	}
}

// UsageStatusLabel は usage.status ごとの本文（ok 以外）を返す。ok は空文字。
// 未知の status は unavailable として扱う（domain.NormalizeUsageStatus）。
func UsageStatusLabel(status string) string {
	switch domain.NormalizeUsageStatus(status) {
	case domain.UsageOK:
		return ""
	case domain.UsageStale:
		return "古いデータ"
	case domain.UsagePending:
		return "取得中"
	case domain.UsageDisabled:
		return "利用上限の取得は無効です"
	default:
		return "取得できません"
	}
}

// UsageReasonLabel は利用上限のアカウントの reason（枠を取れなかった理由）を表示文言にする。
// 原因は swarm-console からは確かには分からないため「可能性」と書く。nil・知らない値は理由なし（空文字）。
func UsageReasonLabel(reason *string) string {
	if reason == nil {
		return ""
	}
	switch *reason {
	case "without_usage":
		return "ログイン切れの可能性"
	case "no_email":
		return "アカウントを特定できません（ログイン切れの可能性）"
	default:
		return ""
	}
}

// CursorWindowLabel は Cursor の window を表示名にする。nil・空は空文字。未知の値はそのまま返す。
func CursorWindowLabel(w *string) string {
	if w == nil {
		return ""
	}
	switch *w {
	case "monthly":
		return "月間"
	case "weekly":
		return "週間"
	default:
		return *w
	}
}

// NoteSummary はメモ要約（repo と items 先頭 3 件、残りの件数）を返す（機能要件 §4.4）。
func NoteSummary(n *domain.Note) (repo string, items []string, more int) {
	if n == nil {
		return "", nil, 0
	}
	const maxItems = 3
	items = n.Items
	if len(items) > maxItems {
		more = len(items) - maxItems
		items = items[:maxItems]
	}
	return n.Repo, items, more
}

// FetchErrorMessage は取得失敗の固定文言を返す（機能要件 §10、API 仕様 §1.5）。
// swarm-console のエラー本文（message・details）は画面に出さない。nil は空文字。
func FetchErrorMessage(e *domain.FetchError) string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case domain.ErrUnreachable:
		return "ホストに届きません。Tailscale 接続を確認してください"
	case domain.ErrUnauthorized:
		return "認証できません。再ペアしてください"
	case domain.ErrForbiddenScope:
		return "このトークンでは接続できません（report 用が必要）"
	case domain.ErrSessionRequired:
		return "セッションを選んでください"
	case domain.ErrSessionForbidden:
		return "このセッションは許可されていません"
	case domain.ErrSessionMissing:
		return "セッションが存在しません"
	case domain.ErrHostError:
		return "ホスト側でエラーが発生しました"
	case domain.ErrOperation:
		return OperationMessage(e.Code)
	default:
		return "接続先がこのアプリと互換ではありません"
	}
}

// NeedsRepair は再ペアリングが必要な失敗（401・403）かを返す。ポーリングを止める判断に使う（API 仕様 §4.4）。
func NeedsRepair(e *domain.FetchError) bool {
	return e != nil && (e.Kind == domain.ErrUnauthorized || e.Kind == domain.ErrForbiddenScope)
}

// PairingErrorMessage は接続文字列の検証失敗の固定文言を返す（API 仕様 §2.2）。
// 入力値（token を含みうる）はエラーにも画面にも含めない。nil は空文字、想定外のエラーは形式不正と同じ文言。
func PairingErrorMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, domain.ErrPairingVersion):
		return "アプリを更新してください"
	case errors.Is(err, domain.ErrInvalidURL):
		return "接続先が許可された範囲外です（Tailscale の IP のみ）"
	case errors.Is(err, domain.ErrPairingToken):
		return "token が含まれていません"
	default:
		return "接続文字列の形式が正しくありません"
	}
}

// JoinNonEmpty は空でない要素を区切り文字で連結する。
func JoinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
