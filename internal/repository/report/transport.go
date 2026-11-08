package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
)

const (
	// maxRequestBytes はリクエスト本文の検査上限。viewer の引数は小さく、swarm-console 側の上限（256 KiB）より十分小さい。
	maxRequestBytes = 64 << 10
	// maxResponseBytes は応答本文の読み取り上限。超えた分は切り捨て、JSON が壊れた扱い（protocol）になる。
	maxResponseBytes = 4 << 20
	// errorSnippetBytes は HTTP エラー本文から `error.code` を読むために見る先頭バイト数。
	errorSnippetBytes = 4 << 10
)

var (
	errURLNotAllowed   = errors.New("report: request url is not allowed")
	errMethodNotAllow  = errors.New("report: http method is not allowed")
	errRPCNotAllowed   = errors.New("report: json-rpc method is not allowed")
	errToolNotAllowed  = errors.New("report: tool is not allowed")
	errRequestTooLarge = errors.New("report: request body too large")
)

// 送ってよい JSON-RPC メソッド（NFR-SEC-1）。`tools/list` など他のメソッドは通さない。
// `tools/call` は更にツール名を、接続先 URL の path が決める種類の許可リスト（tools.go）で絞る。
var allowedRPCMethods = map[string]struct{}{
	"initialize":                {},
	"notifications/initialized": {},
	"notifications/cancelled":   {}, // タイムアウト・キャンセル時に SDK が送る
	"tools/call":                {},
}

// observation は 1 回の呼び出しで観測した HTTP の結果。RoundTripper が書き、呼び出し側が分類に使う。
// SDK のエラー文には HTTP ステータス以外が混ざる（URL など）ため、エラー文は分類にも出力にも使わない。
type observation struct {
	mu       sync.Mutex
	status   int    // 最後の POST 応答のステータス。応答が無ければ 0
	code     string // HTTP エラー本文の error.code（信頼できない値。分類側で検証される）
	rejected bool   // 送信前の検査（URL・メソッド・許可リスト）で止めた
	tokenErr error  // token の解決に失敗した場合
}

func (o *observation) setResponse(status int, code string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status, o.code = status, code
}

func (o *observation) setTokenErr(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tokenErr = err
}

func (o *observation) setRejected() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rejected = true
}

func (o *observation) snapshot() (status int, code string, rejected bool, tokenErr error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.status, o.code, o.rejected, o.tokenErr
}

// roundTripper は Bearer の付与・URL の再検証・送信内容の許可リスト検査を行う（NFR-SEC-1・4・5）。
// 1 回の呼び出しごとに作る。
type roundTripper struct {
	base     http.RoundTripper
	endpoint string                 // ValidateURL で正規化済みの接続先。これ以外へは送らない
	token    func() (string, error) // 送信ごとに解決する
	obs      *observation
}

// RoundTrip は検査を通ったリクエストだけを base へ渡す。エラーは URL・token・本文を含めない。
func (t *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.checkURL(req); err != nil {
		t.obs.setRejected()
		return nil, err
	}
	out, err := t.prepare(req)
	if err != nil {
		t.obs.setRejected()
		return nil, err
	}
	token, err := t.token()
	if err != nil {
		t.obs.setTokenErr(err)
		return nil, errNoToken
	}
	out.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.base.RoundTrip(out)
	if err != nil {
		if req.Method == http.MethodPost {
			t.obs.setResponse(0, "") // 直前の成功応答を引きずらない（接続失敗は unreachable）
		}
		return nil, err
	}
	if req.Method == http.MethodPost {
		t.observe(resp)
	}
	resp.Body = &limitedBody{Reader: io.LimitReader(resp.Body, maxResponseBytes), closer: resp.Body}
	return resp, nil
}

// errNoToken は token を解決できなかったことを表す（原因は observation に残す）。
var errNoToken = errors.New("report: token is not available")

// checkURL は毎回 domain.ValidateURL を通し、呼び出し開始時の接続先と一致することを確かめる。
func (t *roundTripper) checkURL(req *http.Request) error {
	if _, err := domain.ValidateURL(req.URL.String()); err != nil || req.URL.String() != t.endpoint {
		return errURLNotAllowed
	}
	return nil
}

// prepare は HTTP メソッドと JSON-RPC の内容を検査し、元のリクエストを変更せずに複製して返す。
func (t *roundTripper) prepare(req *http.Request) (*http.Request, error) {
	switch req.Method {
	case http.MethodDelete:
		return req.Clone(req.Context()), nil // 終了通知。405 でも失敗にしない（SDK が応答を見ない）
	case http.MethodPost:
	default:
		return nil, errMethodNotAllow // GET（常駐 SSE）は開かない
	}
	body, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}
	if err := checkRPC(domain.KindOfURL(req.URL.String()), body); err != nil {
		return nil, err
	}
	out := req.Clone(req.Context())
	out.Body = io.NopCloser(bytes.NewReader(body))
	out.ContentLength = int64(len(body))
	out.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return out, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, errRPCNotAllowed
	}
	defer func() { _ = req.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRequestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxRequestBytes {
		return nil, errRequestTooLarge
	}
	return body, nil
}

// checkRPC は JSON-RPC リクエストが許可されたメソッド（`tools/call` は kind の許可リストのツール）かを確かめる。
// kind は送信先 URL の path（`/report` なら見るだけの 4 つ、`/ops` なら 10 個）。
// バッチ・応答メッセージ・不明なメソッドは通さない。
func checkRPC(kind domain.ConnectionKind, body []byte) error {
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return errRPCNotAllowed
	}
	if _, ok := allowedRPCMethods[msg.Method]; !ok {
		return errRPCNotAllowed
	}
	if msg.Method == "tools/call" && !toolAllowed(kind, msg.Params.Name) {
		return errToolNotAllowed
	}
	return nil
}

// observe は POST 応答のステータスと、エラー本文の error.code を記録する。本文は SDK のために戻す。
func (t *roundTripper) observe(resp *http.Response) {
	code := ""
	if resp.StatusCode >= http.StatusBadRequest {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errorSnippetBytes))
		code = errorCode(snippet)
		resp.Body = &limitedBody{Reader: io.MultiReader(bytes.NewReader(snippet), resp.Body), closer: resp.Body}
	}
	t.obs.setResponse(resp.StatusCode, code)
}

// errorCode は `{"error": {"code": "<code>"}}` の code を返す。形が違えば空文字。
func errorCode(body []byte) string {
	code, _ := errorDetail(body)
	return code
}

// errorDetail は `{"error": {"code": "<code>", "details": {"field": "<name>"}}}` の code と details.field を返す。
// 形が違う項目は空文字。どちらも応答由来の信頼できない値で、分類の判定にだけ使い、画面・ログには出さない。
func errorDetail(body []byte) (code, field string) {
	var e struct {
		Error struct {
			Code    json.RawMessage `json:"code"`
			Details json.RawMessage `json:"details"` // 型が想定外でも code は読む
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil {
		return "", ""
	}
	_ = json.Unmarshal(e.Error.Code, &code)
	var d struct {
		Field string `json:"field"`
	}
	if json.Unmarshal(e.Error.Details, &d) == nil {
		field = d.Field
	}
	return code, field
}

type limitedBody struct {
	io.Reader
	closer io.Closer
}

func (b *limitedBody) Close() error { return b.closer.Close() }
