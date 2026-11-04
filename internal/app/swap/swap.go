// Package swap は実行中に差し替えられる http.Handler を提供する。
//
// Wails のアセットハンドラは起動時に 1 度だけ渡す。SQLite が壊れていて recovery モードで始まり、
// 初期化に成功したら通常モードへ切り替えるため、ハンドラの差し替えを goroutine-safe にする。
package swap

import (
	"net/http"
	"sync/atomic"
)

// Handler は atomic に差し替えられる http.Handler。ゼロ値は「未設定」（503）。
type Handler struct {
	cur atomic.Pointer[http.Handler]
}

// Set は以後のリクエストを h に流す。処理中のリクエストは元のハンドラで完了する。
func (s *Handler) Set(h http.Handler) {
	if h == nil {
		s.cur.Store(nil)
		return
	}
	s.cur.Store(&h)
}

// ServeHTTP は現在のハンドラへ委譲する。未設定なら 503。
func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := s.cur.Load()
	if h == nil {
		http.Error(w, "starting", http.StatusServiceUnavailable)
		return
	}
	(*h).ServeHTTP(w, r)
}
