package report

import (
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

// 呼び出すツール名（API 仕様 §1.2）。ツール名は定数で、外部から渡す経路を公開しない（NFR-SEC-1）。
const (
	toolSessionsList = "sessions_list"
	toolReport       = "report"
	toolPaneTail     = "pane_tail"
	toolAskCard      = "ask_card"
	toolAskPick      = "ask_pick"
	toolSendText     = "send_text"
	toolSendEscape   = "send_escape"
	toolPaneStart    = "pane_start"
	toolPaneClose    = "pane_close"
	toolDirList      = "dir_list"
)

// viewTools は見るだけの接続（`/report`）で呼んでよいツール。
var viewTools = map[string]struct{}{
	toolSessionsList: {}, toolReport: {}, toolPaneTail: {}, toolAskCard: {},
}

// opsTools は操作の接続（`/ops`）で呼んでよいツール（見るだけの 4 つ ＋ 操作の 6 つ）。
var opsTools = map[string]struct{}{
	toolSessionsList: {}, toolReport: {}, toolPaneTail: {}, toolAskCard: {},
	toolAskPick: {}, toolSendText: {}, toolSendEscape: {}, toolPaneStart: {}, toolPaneClose: {}, toolDirList: {},
}

// toolAllowed は接続の種類ごとの許可リストに name があるか。callTool が送信前に、roundTripper が送信内容に対して
// 二重に検査する。種類は接続先 URL の path から決まる（domain.KindOfURL）。範囲外（`panes_status`・`dispatch`・
// `note_set`・`file_get`・`tools/list` など）はどちらの種類でも通らない。
func toolAllowed(kind domain.ConnectionKind, name string) bool {
	set := viewTools
	if kind == domain.KindOps {
		set = opsTools
	}
	_, ok := set[name]
	return ok
}

// paneScoped は sessions_list・report 以外のツール。invalid_args を session の不在（session_missing）に決め打ちせず、
// details.field で見分ける（domain.OperationFailure）。
func paneScoped(name string) bool { return name != toolSessionsList && name != toolReport }

// toolTimeoutScale は 1 回の呼び出しの上限（Client.timeout）に掛ける倍率。操作ツールは画面の確認を待つため長い
// （swarm-console の既定: accept_timeout 10 秒・start_timeout 30 秒。pane_start・pane_close は複数の手順を踏む）。載っていないツールは 1 倍。
var toolTimeoutScale = map[string]time.Duration{
	toolAskPick: 3, toolSendText: 3, toolSendEscape: 3, toolPaneClose: 5, toolPaneStart: 5,
}

// timeoutFor は tool の 1 回の呼び出し（接続・応答の合計）の上限。
func (c *Client) timeoutFor(tool string) time.Duration {
	if n, ok := toolTimeoutScale[tool]; ok {
		return c.timeout * n
	}
	return c.timeout
}
