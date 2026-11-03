package service

// テスト用に内部のフック・状態を公開する（service_test から使う）。

// SetAfterRound は通知用ポーリングの一巡が終わるたびに呼ぶフックを設定する。Run を起動する前に呼ぶ。
func SetAfterRound(s *NotificationService, f func()) { s.afterRound = f }

// SetAfterJoin は Get が single-flight に合流した直後に呼ぶフックを設定する。Get を呼ぶ前に設定する。
func SetAfterJoin(s *SnapshotService, f func()) { s.afterJoin = f }

// HasSnapshotCache は (接続, セッション) の直近値がメモリにあるかを返す。
func HasSnapshotCache(s *SnapshotService, connID, session string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.cache[snapshotKey(connID, session)]
	return ok
}

// TouchObserveLock は接続の Observe 用 mutex を作る。
func TouchObserveLock(s *NotificationService, connID string) { s.lockFor(connID) }

// HasObserveLock は接続の Observe 用 mutex が残っているかを返す。
func HasObserveLock(s *NotificationService, connID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.locks[connID]
	return ok
}
