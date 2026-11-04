package servicetest

import (
	"sync"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/service"
)

// Clock は手動で進める service.Clock。ticker は Tick で明示的に発火する。
type Clock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*Ticker
}

// NewClock は固定の起点時刻の Clock を作る。
func NewClock() *Clock {
	return &Clock{now: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *Clock) NewTicker(d time.Duration) service.Ticker {
	t := &Ticker{c: make(chan time.Time, 1), clock: c, interval: d, Resets: make(chan time.Duration, 16)}
	c.mu.Lock()
	c.tickers = append(c.tickers, t)
	c.mu.Unlock()
	return t
}

// TickerCount は作られた ticker の数。
func (c *Clock) TickerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tickers)
}

func (c *Clock) Ticker(i int) *Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tickers[i]
}

type Ticker struct {
	c     chan time.Time
	clock *Clock

	mu       sync.Mutex
	interval time.Duration
	stopped  bool
	Resets   chan time.Duration // Reset の引数を通知する
}

func (t *Ticker) C() <-chan time.Time { return t.c }

func (t *Ticker) Reset(d time.Duration) {
	t.mu.Lock()
	t.interval = d
	t.mu.Unlock()
	t.Resets <- d
}

func (t *Ticker) Stop() {
	t.mu.Lock()
	t.stopped = true
	t.mu.Unlock()
}

func (t *Ticker) Interval() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.interval
}

func (t *Ticker) Stopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}

// Tick は ticker を 1 回発火する（テストが一巡の完了を待つ前提。バッファは 1）。
func (t *Ticker) Tick() { t.c <- t.clock.Now() }
