package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// genericErrorMessage は Recover が返す固定文言。panic 値は画面に出さない。
const genericErrorMessage = "内部エラーが発生しました。もう一度お試しください"

// Recover は panic を捕捉する。スタックと panic 値の型だけをログに残し（値の文字列化はしない。
// token・本文が混じりうるため）、画面には汎用エラーフラグメント（500）を返す。
// 応答を書き始めていた場合は上書きせず中断する。
func Recover(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// ハンドラの中断（http.ErrAbortHandler）は net/http の規約どおり呼び出し元へ戻す。
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			logger.Error("panic recovered",
				slog.String("method", c.Request.Method),
				slog.String("route", routeOf(c)),
				slog.String("panic_type", fmt.Sprintf("%T", rec)),
				slog.String("stack", string(debug.Stack())),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			renderAlert(c, http.StatusInternalServerError, genericErrorMessage)
		}()
		c.Next()
	}
}

// RequestLog はリクエストのメソッド・パス・ステータス・所要時間だけを記録する。
// クエリ・本文・ヘッダは出さない。パスはルート定義（`/connections/:cid` の形）で、ID を含む実パスは出さない。
// 通常は Debug、5xx は Warn（既定のログレベルでは警告のみ残る。NFR-LOG-2）。
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		level := slog.LevelDebug
		if status >= http.StatusInternalServerError {
			level = slog.LevelWarn
		}
		logger.LogAttrs(c.Request.Context(), level, "request",
			slog.String("method", c.Request.Method),
			slog.String("path", routeOf(c)),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		)
	}
}

// routeOf は一致したルート定義。一致しなかったときは任意の入力を含みうる実パスを出さず固定値にする。
func routeOf(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "(unmatched)"
}
