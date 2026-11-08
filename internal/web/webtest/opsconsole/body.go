package opsconsole

import (
	"bytes"
	"io"
	"net/http"
)

// readBody はリクエスト本文を読み、サーバー実装が読めるよう戻す。
func readBody(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b
}
