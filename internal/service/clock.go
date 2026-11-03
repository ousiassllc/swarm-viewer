package service

import "time"

// SystemClock は実時間の Clock。
type SystemClock struct{}

// Now は現在時刻を返す。
func (SystemClock) Now() time.Time { return time.Now() }

// NewTicker は time.Ticker を包んだ Ticker を返す。
func (SystemClock) NewTicker(d time.Duration) Ticker { return systemTicker{t: time.NewTicker(d)} }

type systemTicker struct{ t *time.Ticker }

func (s systemTicker) C() <-chan time.Time   { return s.t.C }
func (s systemTicker) Reset(d time.Duration) { s.t.Reset(d) }
func (s systemTicker) Stop()                 { s.t.Stop() }
