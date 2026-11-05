package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/web/atoms"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
)

// CSRFHeaderName は CSRF トークンを運ぶリクエストヘッダ名（layout.CSRFHeader と同じ）。
const CSRFHeaderName = layout.CSRFHeader

// ContentSecurityPolicy は全応答に付ける CSP（NFR-SEC-9）。unsafe-eval・unsafe-inline を含まない。
const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'"

// NewCSRFToken はプロセス起動時に 1 回呼ぶ CSRF トークン（256 bit、base64url）を生成する。
func NewCSRFToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("middleware: csrf token generation failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// CSRF は状態変更ルート（GET・HEAD・OPTIONS 以外）で X-CSRF-Token を検証する。
// 比較は定数時間。トークンが無い・一致しないときは 403 の汎用フラグメントで中断する。
// token が空のときは常に拒否する（未設定のまま全許可になることを防ぐ）。
func CSRF(token string) gin.HandlerFunc {
	expected := []byte(token)
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		got := []byte(c.GetHeader(CSRFHeaderName))
		if len(expected) == 0 || subtle.ConstantTimeCompare(got, expected) != 1 {
			renderAlert(c, http.StatusForbidden, "リクエストを確認できませんでした。画面を開き直してください")
			return
		}
		c.Next()
	}
}

// NoStore は全応答に `Cache-Control: no-store` を付ける（スナップショットのキャッシュ防止）。
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// CSP は全応答に Content-Security-Policy ヘッダを付ける。
func CSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", ContentSecurityPolicy)
		c.Next()
	}
}

// renderAlert は固定文言のエラーフラグメントを status で返して中断する。
func renderAlert(c *gin.Context, status int, message string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	c.Abort()
	// 書き込み失敗（クライアント切断）は握りつぶす。応答は既に中断済みで、呼び出し元に返せる処理がない。
	_ = atoms.Alert(atoms.AlertProps{Kind: atoms.AlertDanger, Message: message}).Render(c.Request.Context(), c.Writer)
}
