// Package report は swarm-console の `/report`・`/ops` エンドポイント（MCP Streamable HTTP）のクライアント。
//
// 見るだけの接続（`/report`）は sessions_list・report・pane_tail・ask_card、操作の接続（`/ops`）はさらに ask_pick・send_text・
// send_escape・pane_start・pane_close・dir_list を呼べる。接続の種類は接続先 URL の path で決まる（NFR-SEC-1）。token は
// secret.Store から接続 ID で解決し、戻り値・エラー・ログに出さない（NFR-SEC-2・11、NFR-LOG-3）。失敗は常に *domain.FetchError で返す。
//
// swarm-console はステートレスな Streamable HTTP（POST のみ。GET・DELETE は 405）。公式 go-sdk を、
// 呼び出しごとにセッションを開閉し、常駐 GET（DisableStandaloneSSE）と server/discover を使わない設定で使う。
package report

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/repository/secret"
)

const (
	// requestTimeout は 1 回の呼び出し（接続・応答の合計）の上限（NFR-PERF-4）。
	requestTimeout = 10 * time.Second
	// mcpProtocolVersion は initialize で提示する MCP の版。SDK 既定（2026-07-28）は server/discover を先に
	// 試すため、旧来の initialize ハンドシェイクに固定する。サーバーが別の対応版へ下げるのは許容する。
	mcpProtocolVersion = "2025-06-18"
	clientName         = "swarm-viewer"
	clientVersion      = "0.1.0"
)

// TokenStore は接続の token の取得元。*secret.Store が満たす。
type TokenStore interface {
	Get(connID string) (string, error)
}

// Client は swarm-console `/report` のクライアント。並行利用できる（呼び出しごとに状態を持たない）。
type Client struct {
	tokens  TokenStore
	logger  *slog.Logger
	base    http.RoundTripper
	timeout time.Duration
}

// NewClient は Client を返す。logger が nil ならログを出さない。
// 環境変数のプロキシは使わない（token を意図しない中継へ流さない）。
func NewClient(tokens TokenStore, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	return &Client{tokens: tokens, logger: logger, base: base, timeout: requestTimeout}
}

// Sessions は許可されたセッションの一覧を返す（`sessions_list`。返却順のまま）。
func (c *Client) Sessions(ctx context.Context, conn domain.Connection) ([]domain.Session, error) {
	token := func() (string, error) { return c.tokens.Get(conn.ID) }
	text, err := c.run(ctx, conn.ID, conn.URL, token, toolSessionsList, map[string]any{})
	if err != nil {
		return nil, err
	}
	return c.parseSessions(conn.ID, text)
}

// Report は 1 セッションのスナップショットを返す（`report`、format=json）。
// session は常に明示して送る。空なら通信せず session_required を返す。
func (c *Client) Report(ctx context.Context, conn domain.Connection, session string) (domain.Report, error) {
	if session == "" {
		return domain.Report{}, &domain.FetchError{Kind: domain.ErrSessionRequired, Code: "session_required"}
	}
	token := func() (string, error) { return c.tokens.Get(conn.ID) }
	args := map[string]any{"format": "json", "session": session}
	text, err := c.run(ctx, conn.ID, conn.URL, token, toolReport, args)
	if err != nil {
		return domain.Report{}, err
	}
	rep, perr := domain.ParseReport([]byte(text))
	if perr != nil {
		return domain.Report{}, c.fail(conn.ID, toolReport, domain.Classify(perr, 0, ""))
	}
	return rep, nil
}

// Probe はペアリングの疎通確認。保存前なので token を直接受け取る。許可されたセッションの一覧を返す。
func (c *Client) Probe(ctx context.Context, url, token string) ([]domain.Session, error) {
	text, err := c.run(ctx, "", url, func() (string, error) { return token, nil }, toolSessionsList, map[string]any{})
	if err != nil {
		return nil, err
	}
	return c.parseSessions("", text)
}

func (c *Client) parseSessions(connID, text string) ([]domain.Session, error) {
	sessions, err := domain.ParseSessions([]byte(text))
	if err != nil {
		return nil, c.fail(connID, toolSessionsList, domain.Classify(err, 0, ""))
	}
	return sessions, nil
}

// run は 1 回のツール呼び出しを行い、失敗を分類・記録して返す。nil でない error は必ず *domain.FetchError。
func (c *Client) run(ctx context.Context, connID, rawURL string, token func() (string, error), tool string, args map[string]any) (string, error) {
	text, fe := c.callTool(ctx, rawURL, token, tool, args)
	if fe != nil {
		return "", c.fail(connID, tool, fe)
	}
	return text, nil
}

// fail は失敗をログへ残して返す。ログは接続 ID・ツール名・種別・コードだけ（URL・token・本文は出さない）。
func (c *Client) fail(connID, tool string, fe *domain.FetchError) error {
	attrs := []any{"tool", tool, "kind", fe.Kind.String(), "code", fe.Code}
	if connID != "" {
		attrs = append([]any{"connection_id", connID}, attrs...)
	}
	c.logger.Warn("swarm-console request failed", attrs...)
	return fe
}

// callTool はセッションを開き、ツールを 1 回呼び、閉じる。
// 接続の種類は検証済みの接続先 URL の path から決め（`/report` は見るだけ、`/ops` は操作）、種類の許可リストにないツールは通信せずに拒否する。
func (c *Client) callTool(ctx context.Context, rawURL string, token func() (string, error), tool string, args map[string]any) (string, *domain.FetchError) {
	endpoint, err := domain.ValidateURL(rawURL)
	if err != nil {
		return "", &domain.FetchError{Kind: domain.ErrProtocol, Code: "invalid_url"}
	}
	if !toolAllowed(domain.KindOfURL(endpoint), tool) {
		return "", &domain.FetchError{Kind: domain.ErrProtocol, Code: "tool_not_allowed"}
	}
	timeout := c.timeoutFor(tool)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	obs := &observation{}
	hc := &http.Client{
		Transport: &roundTripper{base: c.base, endpoint: endpoint, token: token, obs: obs},
		Timeout:   timeout,
		// リダイレクトは追従しない。token を別の宛先へ送らない（3xx はそのまま応答として返る）。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	text, err := invoke(ctx, hc, endpoint, tool, args)
	if err == nil {
		return text, nil
	}
	var fe *domain.FetchError
	if errors.As(err, &fe) { // ツールが isError を返した
		return "", fe
	}
	return "", classifyTransport(ctx, obs, err)
}

// invoke は SDK のセッションを開いて tools/call を 1 回行う。返る error の文面は呼び出し側で使わない。
func invoke(ctx context.Context, hc *http.Client, endpoint, tool string, args map[string]any) (string, error) {
	client := mcp.NewClient(
		&mcp.Implementation{Name: clientName, Version: clientVersion},
		&mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}},
	)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           hc,
		MaxRetries:           -1,   // 再試行しない（次の周期で取り直す。NFR-AVL-2）
		DisableStandaloneSSE: true, // 常駐 GET を張らない（swarm-console は GET を 405 で拒否する）
	}
	session, err := client.Connect(ctx, transport, &mcp.ClientSessionOptions{ProtocolVersion: mcpProtocolVersion})
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "", err
	}
	return toolText(res, paneScoped(tool))
}

// toolText は結果の先頭のテキストを返す。isError のときは error.code を分類した *domain.FetchError を返す。
// perPane（ペイン・入力を引数に取るツール）の invalid_args は、details.field が session のときだけ session_missing、
// それ以外は operation にする。
func toolText(res *mcp.CallToolResult, perPane bool) (string, error) {
	text := ""
	if res != nil {
		for _, content := range res.Content {
			if t, ok := content.(*mcp.TextContent); ok {
				text = t.Text
				break
			}
		}
	}
	if res != nil && res.IsError {
		code, field := errorDetail([]byte(text))
		fe := domain.Classify(nil, http.StatusOK, code)
		if perPane {
			fe = ops.Failure(code, field)
		}
		if fe != nil {
			return "", fe
		}
		return "", &domain.FetchError{Kind: domain.ErrProtocol}
	}
	if text == "" {
		return "", domain.ErrInvalidResponse
	}
	return text, nil
}

// classifyTransport は HTTP・ネットワークレベルの失敗を分類する。SDK のエラー文は使わない。
func classifyTransport(ctx context.Context, obs *observation, err error) *domain.FetchError {
	status, code, rejected, tokenErr := obs.snapshot()
	switch {
	case tokenErr != nil:
		if errors.Is(tokenErr, secret.ErrNotFound) {
			return &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "token_missing"}
		}
		return &domain.FetchError{Kind: domain.ErrUnreachable, Code: "secret_store"}
	case rejected:
		return &domain.FetchError{Kind: domain.ErrProtocol, Code: "request_rejected"}
	case status >= http.StatusBadRequest:
		return domain.Classify(err, status, code)
	case status >= http.StatusMultipleChoices: // リダイレクトの拒否
		return &domain.FetchError{Kind: domain.ErrProtocol, Code: "http_" + strconv.Itoa(status)}
	case status == 0 || ctx.Err() != nil:
		return domain.Classify(err, 0, "") // 接続失敗・タイムアウト → unreachable
	default:
		return domain.Classify(domain.ErrInvalidResponse, 0, "") // 応答は届いたが MCP として解釈できない
	}
}
