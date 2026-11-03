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

	StaticCSS            = "/static/css/app.css"
	StaticHTMX           = "/static/js/htmx.min.js"
	StaticResponseTarget = "/static/js/response-targets.min.js"
	StaticAppJS          = "/static/js/app.js"
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
