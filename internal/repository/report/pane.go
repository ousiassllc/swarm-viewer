package report

import (
	"context"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
)

// このファイルはペインを見る・操作するメソッド（API 仕様 §5.1・§5.2）。接続の種類（URL の path）に合わない
// ツールは、通信せず protocol / tool_not_allowed で拒否する（見るだけの接続は pane_tail・ask_card まで）。
// session は常に明示し、pane は report の panes[].pos（セッション内の位置）。送信本文・応答本文はログに出さない。

// PaneTail は 1 ペインの最後の lines 行を返す（`pane_tail`。lines は 1〜200 に収め、0 以下は 30。raw は付けない）。
func (c *Client) PaneTail(ctx context.Context, conn domain.Connection, session, pane string, lines int) (ops.PaneTail, error) {
	args, err := paneArgs(session, pane)
	if err != nil {
		return ops.PaneTail{}, c.fail(conn.ID, toolPaneTail, err)
	}
	args["lines"] = ops.NormalizeTailLines(lines)
	return callParsed(ctx, c, conn, toolPaneTail, args, ops.ParsePaneTail)
}

// AskCard は 1 ペインの質問カードを返す（`ask_card`）。
func (c *Client) AskCard(ctx context.Context, conn domain.Connection, session, pane string) (ops.AskCard, error) {
	args, err := paneArgs(session, pane)
	if err != nil {
		return ops.AskCard{}, c.fail(conn.ID, toolAskCard, err)
	}
	return callParsed(ctx, c, conn, toolAskCard, args, ops.ParseAskCard)
}

// AskPick は質問カードの選択肢（番号）か「その他」の文字列を送る（`ask_pick`。n と text は同時に送らない）。
func (c *Client) AskPick(ctx context.Context, conn domain.Connection, session, pane string, pick ops.Pick) (ops.PickResult, error) {
	args, err := paneArgs(session, pane)
	if err == nil {
		if code := pick.Check(); code != "" {
			err = ops.Error(code)
		}
	}
	if err != nil {
		return ops.PickResult{}, c.fail(conn.ID, toolAskPick, err)
	}
	if pick.N != 0 {
		args["n"] = pick.N
	} else {
		args["text"] = pick.Text
	}
	return callParsed(ctx, c, conn, toolAskPick, args, ops.ParsePick)
}

// SendText はペインへ指示を貼り付けて送る（`send_text`。1〜8000 字、改行とタブ以外の制御文字は通信前に拒否）。
func (c *Client) SendText(ctx context.Context, conn domain.Connection, session, pane, text string) (ops.SendResult, error) {
	args, err := paneArgs(session, pane)
	if err == nil {
		if code := ops.CheckText(text); code != "" {
			err = ops.Error(code)
		}
	}
	if err != nil {
		return ops.SendResult{}, c.fail(conn.ID, toolSendText, err)
	}
	args["text"] = text
	return callParsed(ctx, c, conn, toolSendText, args, ops.ParseSend)
}

// SendEscape はペインへ Esc を送って作業を中断する（`send_escape`）。confirmed が false なら通信せず confirm_required を返す
// （確認ダイアログを経た呼び出しだけが true を渡す。送る引数は常に confirm: true）。
func (c *Client) SendEscape(ctx context.Context, conn domain.Connection, session, pane string, confirmed bool) (ops.EscapeResult, error) {
	args, err := paneArgs(session, pane)
	if err == nil && !confirmed {
		err = ops.Error("confirm_required")
	}
	if err != nil {
		return ops.EscapeResult{}, c.fail(conn.ID, toolSendEscape, err)
	}
	args["confirm"] = true
	return callParsed(ctx, c, conn, toolSendEscape, args, ops.ParseEscape)
}

// PaneStart は shell のペインでエージェントを起動する（`pane_start`）。入力は通信前に検査する（title の前後の空白は除く）。
// 許可フォルダかどうかは swarm-console が判定する（forbidden_path）。
func (c *Client) PaneStart(ctx context.Context, conn domain.Connection, session, pane string, req ops.StartRequest) (ops.StartResult, error) {
	args, err := paneArgs(session, pane)
	req, code := req.Normalize()
	if err == nil && code != "" {
		err = ops.Error(code)
	}
	if err != nil {
		return ops.StartResult{}, c.fail(conn.ID, toolPaneStart, err)
	}
	args["agent"], args["cwd"], args["title"] = req.Agent, req.Cwd, req.Title
	return callParsed(ctx, c, conn, toolPaneStart, args, ops.ParseStart)
}

// PaneClose はペインのエージェントを終了して片付ける（`pane_close`）。
func (c *Client) PaneClose(ctx context.Context, conn domain.Connection, session, pane string) (ops.CloseResult, error) {
	args, err := paneArgs(session, pane)
	if err != nil {
		return ops.CloseResult{}, c.fail(conn.ID, toolPaneClose, err)
	}
	return callParsed(ctx, c, conn, toolPaneClose, args, ops.ParseClose)
}

// DirList は pane_start の cwd に選べるフォルダを 1 階層ずつ返す（`dir_list`）。path が空なら許可フォルダの根（allowed_roots）。
// session は取らず、limit は指定しない。
func (c *Client) DirList(ctx context.Context, conn domain.Connection, path string) (ops.DirList, error) {
	args := map[string]any{}
	if path != "" {
		args["path"] = path
	}
	return callParsed(ctx, c, conn, toolDirList, args, ops.ParseDirList)
}

// SkillList は新しいセッションの最初のタスクに選べるスキルを返す（`skill_list`。仮称で swarm-console 側は未実装。
// 未実装のホストは unknown_tool を返す）。session は取らず、cwd も渡さない。agent は omp・claude・codex のどれか（通信前に検査）。
// スキル名・description・invoke はログにもエラーにも出さない。
func (c *Client) SkillList(ctx context.Context, conn domain.Connection, agent string) (ops.SkillList, error) {
	if !ops.IsAgent(agent) {
		return ops.SkillList{}, c.fail(conn.ID, toolSkillList, ops.Error("invalid_args"))
	}
	return callParsed(ctx, c, conn, toolSkillList, map[string]any{"agent": agent}, ops.ParseSkillList)
}

// paneArgs は session・pane を必須にした引数。空なら通信せずに失敗させる（session は常に明示する）。
func paneArgs(session, pane string) (map[string]any, *domain.FetchError) {
	if session == "" {
		return nil, &domain.FetchError{Kind: domain.ErrSessionRequired, Code: "session_required"}
	}
	if pane == "" {
		return nil, ops.Error("invalid_args")
	}
	return map[string]any{"session": session, "pane": pane}, nil
}

// callParsed はツールを 1 回呼び、結果の JSON を parse で型に直す。解析できなければ protocol。
func callParsed[T any](ctx context.Context, c *Client, conn domain.Connection, tool string, args map[string]any, parse func([]byte) (T, error)) (T, error) {
	var zero T
	token := func() (string, error) { return c.tokens.Get(conn.ID) }
	text, err := c.run(ctx, conn.ID, conn.URL, token, tool, args)
	if err != nil {
		return zero, err
	}
	v, perr := parse([]byte(text))
	if perr != nil {
		return zero, c.fail(conn.ID, tool, domain.Classify(perr, 0, ""))
	}
	return v, nil
}
