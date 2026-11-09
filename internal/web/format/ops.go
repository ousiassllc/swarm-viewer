package format

import (
	"strconv"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

// 操作モード（Issue #97）の固定文言。画面には swarm-console の message を出さず、ここの文言を出す。
const (
	// BusyMessage は同じペインの前の操作がまだ終わっていないとき（viewer 側の二重送信防止）。
	BusyMessage = "前の操作がまだ終わっていません"
	// ViewOnlyMessage は見るだけの接続で操作を求められたとき。
	ViewOnlyMessage = "この接続では操作できません（見るだけの接続です）"
	// OpsWarning・OpsLeakNote は操作の接続のペアリング確認画面の警告文。
	OpsWarning  = "この接続はペインに送信・中断・起動・終了できます"
	OpsLeakNote = "token が漏れると、この Tailscale 内からペインを操作されます。紛失したらホストで `swarm-console token revoke <名前>` を実行してください"
	// ViewOnlyAskNote は見るだけの接続の質問カードに添える 1 行。
	ViewOnlyAskNote = "この質問には、操作できる接続から答えられます"
	// ViewOnlyPromptNote は見るだけの接続のペイン詳細に添える 1 行（指示の入力欄が無い理由と、出すための方法）。
	ViewOnlyPromptNote = "この接続は見るだけです。ペインへの指示は、操作の接続（/ops）で登録すると入力できます"
)

// operationMessages は ErrOperation の Code ごとの固定文言（機能要件 §10）。
var operationMessages = map[string]string{
	"invalid_state":          "ペインの状態が変わったため、操作できませんでした。最新の状態を確認してください",
	"not_asking":             "この質問はすでに閉じています",
	"option_out_of_range":    "その選択肢はありません。最新の質問を読み込み直してください",
	"custom_not_allowed":     "この質問は自由入力に対応していません",
	"cursor_mismatch":        "選択肢を合わせられませんでした（何も送っていません）。もう一度お試しください",
	"confirm_required":       "確認が必要です",
	"forbidden_control_char": "使えない制御文字が含まれています（改行とタブだけ使えます）",
	"text_too_long":          "長すぎます（" + strconv.Itoa(ops.MaxSendChars) + " 字まで）",
	"forbidden_path":         "このフォルダは選べません",
	"path_forbidden":         "このフォルダは選べません",
	"file_unreadable":        "ファイルを読めませんでした",
	"unsupported_agent":      "このペインは対応していないエージェントです",
	"pane_not_found":         "ペインが見つかりません。画面を更新してください",
	"busy_timeout":           "同じペインの別の操作がまだ終わっていません。しばらくしてからお試しください",
	"exit_timeout":           "終了が間に合いませんでした（もう一度試せます）",
	"not_found":              "フォルダが見つかりません",
	"unknown_tool":           "この接続では使えない操作です（ホストの swarm-console が古い可能性があります）",
	"invalid_args":           "入力内容を受け付けられませんでした",
}

// OperationMessage は ErrOperation の Code の固定文言。知らないコードは invalid_args と同じ文言（本文は出さない）。
func OperationMessage(code string) string {
	if m, ok := operationMessages[code]; ok {
		return m
	}
	return operationMessages["invalid_args"]
}

// OperationFailureMessage は操作の失敗（*domain.FetchError）の固定文言。
func OperationFailureMessage(e *domain.FetchError) string {
	if e != nil && e.Kind == domain.ErrOperation {
		return OperationMessage(e.Code)
	}
	return FetchErrorMessage(e)
}

// KindLabel は接続の種類のバッジの文言（色だけに頼らない）。
func KindLabel(k domain.ConnectionKind) string {
	if k == domain.KindOps {
		return "操作"
	}
	return "見るだけ"
}

// KindTone は種類のバッジの色。操作は注意（ペインに送れる）、見るだけは中立。
func KindTone(k domain.ConnectionKind) Tone {
	if k == domain.KindOps {
		return ToneWarning
	}
	return ToneMuted
}

// 操作の通知（トースト）の文言（機能要件 §10）。
const (
	toastSent = "送りました"
)

// PickToast は ask_pick の結果の通知。ok かつ card_closed のときだけ成功。
func PickToast(r ops.PickResult) string {
	if r.OK && r.CardClosed {
		return toastSent
	}
	return "選択を確定できませんでした。もう一度お試しください"
}

// SendToast は send_text の結果の通知。
func SendToast(r ops.SendResult) string {
	if r.Accepted {
		return toastSent
	}
	return "受け付けられませんでした"
}

// EscapeToast は send_escape の結果の通知。
func EscapeToast(r ops.EscapeResult) string {
	if r.OK && r.Interrupted {
		return "中断しました"
	}
	return "状態が変わりませんでした"
}

// StartToast は pane_start の結果の通知。
func StartToast(r ops.StartResult) string {
	if r.OK {
		return "起動しました"
	}
	return "起動を確認できませんでした（状態: " + r.State + "）"
}

// CloseToast は pane_close の結果の通知。
func CloseToast(r ops.CloseResult) string {
	if r.OK {
		return "終了しました"
	}
	return "片付けを確認できませんでした（状態: " + r.State + "）"
}

// CanSend は send_text の入力欄を使える状態か。idle・stalled だけ。
func CanSend(s domain.PaneState) bool { return s == domain.StateIdle || s == domain.StateStalled }

// SendBlockedReason は入力欄を使えない状態の理由（1 行）。使える状態は空文字。
func SendBlockedReason(s domain.PaneState) string {
	switch s {
	case domain.StateWorking:
		return "作業中のため送れません。終わるまで待つか、中断してください"
	case domain.StateAsking:
		return "質問に答えてから送れます"
	case domain.StateShell:
		return "エージェントが動いていません。「始める」から起動してください"
	case domain.StateIdle, domain.StateStalled:
		return ""
	default:
		return "このペインには送れません"
	}
}

// CanEscape は中断（Esc）を出せる状態か。working・stalled・asking・idle。
func CanEscape(s domain.PaneState) bool {
	return s == domain.StateWorking || s == domain.StateStalled || s == domain.StateAsking || s == domain.StateIdle
}

// CanClose は終了（pane_close）を出せる状態か。idle・shell だけ。
func CanClose(s domain.PaneState) bool { return s == domain.StateIdle || s == domain.StateShell }
