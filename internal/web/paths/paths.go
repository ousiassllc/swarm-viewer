// Package paths は画面ルートのパスと静的アセットのパスを組み立てる（docs/api/endpoints.md §4.2）。
//
// Templ コンポーネントとハンドラが同じ文字列を共有するための小さな置き場。
// 可変部分（{cid}・{sid}・{did}）は url.PathEscape でエスケープする。
package paths

import "net/url"

// 固定パス。
const (
	Home          = "/"
	Pairing       = "/pairing"
	Connections   = "/connections"
	Settings      = "/settings"
	RecoveryReset = "/recovery/reset"

	// PairingLinkParam・PairingLinkInvalid は、ディープリンクの解析に失敗したことを /pairing へ知らせるクエリ。
	// 受け取った文字列は載せない（token を URL 履歴に残さない）。
	PairingLinkParam   = "link"
	PairingLinkInvalid = "invalid"
	// PairingInvalidLink は解析に失敗したディープリンクの後に開く `/pairing`（固定文言のエラー付き）。
	PairingInvalidLink = Pairing + "?" + PairingLinkParam + "=" + PairingLinkInvalid

	StaticCSS            = "/static/css/app.css"
	StaticHTMX           = "/static/js/htmx.min.js"
	StaticResponseTarget = "/static/js/response-targets.min.js"
	StaticAppJS          = "/static/js/app.js"
	StaticModalJS        = "/static/js/modal.js"
)

// Connection は `/connections/{cid}`。
func Connection(cid string) string {
	return Connections + "/" + url.PathEscape(cid)
}

// Session は `/connections/{cid}/sessions/{sid}`。
func Session(cid, sid string) string {
	return Connection(cid) + "/sessions/" + url.PathEscape(sid)
}

// Snapshot は `/connections/{cid}/sessions/{sid}/snapshot`。
func Snapshot(cid, sid string) string {
	return Session(cid, sid) + "/snapshot"
}

// Pane は `/connections/{cid}/sessions/{sid}/panes/{pos}`（ペイン詳細）。pos は report の panes[].pos。
func Pane(cid, sid, pos string) string {
	return Session(cid, sid) + "/panes/" + url.PathEscape(pos)
}

// PaneLive は `…/panes/{pos}/live`（ペイン詳細の本文の部分描画。5 秒ごとに読み直す）。
func PaneLive(cid, sid, pos string) string { return Pane(cid, sid, pos) + "/live" }

// PanePick・PaneSend・PaneEscape・PaneClose は操作（POST。操作の接続のみ）。
func PanePick(cid, sid, pos string) string   { return Pane(cid, sid, pos) + "/pick" }
func PaneSend(cid, sid, pos string) string   { return Pane(cid, sid, pos) + "/send" }
func PaneEscape(cid, sid, pos string) string { return Pane(cid, sid, pos) + "/escape" }
func PaneClose(cid, sid, pos string) string  { return Pane(cid, sid, pos) + "/close" }

// PaneStart は `…/panes/{pos}/start`（新しいセッション。GET が画面、POST が起動）。
func PaneStart(cid, sid, pos string) string { return Pane(cid, sid, pos) + "/start" }

// PaneStartDirs は `…/panes/{pos}/start/dirs`（フォルダピッカーの部分描画。クエリ path が一覧するフォルダ）。
func PaneStartDirs(cid, sid, pos string) string { return PaneStart(cid, sid, pos) + "/dirs" }

// PaneStartSkills は `…/panes/{pos}/start/skills`（スキルの選択部品の部分描画。クエリ agent が対象のエージェント）。
func PaneStartSkills(cid, sid, pos string) string { return PaneStart(cid, sid, pos) + "/skills" }

// Draft は `/pairing/drafts/{did}`。
func Draft(did string) string {
	return Pairing + "/drafts/" + url.PathEscape(did)
}

// DraftConfirm は `/pairing/drafts/{did}/confirm`。
func DraftConfirm(did string) string {
	return Draft(did) + "/confirm"
}

// DraftDiscard は `/pairing/drafts/{did}/discard`。
func DraftDiscard(did string) string {
	return Draft(did) + "/discard"
}
