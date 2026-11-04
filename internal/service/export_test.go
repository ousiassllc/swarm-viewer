package service

// テスト用に内部のフック・状態を公開する（service_test から使う）。

// SetAfterRound は通知用ポーリングの一巡が終わるたびに呼ぶフックを設定する。Run を起動する前に呼ぶ。
func SetAfterRound(s *NotificationService, f func()) { s.afterRound = f }

// SetAfterJoin は Get が single-flight に合流した直後に呼ぶフックを設定する。Get を呼ぶ前に設定する。
func SetAfterJoin(s *SnapshotService, f func()) { s.afterJoin = f }

// DraftHeld は下書き（token を含む）がメモリに残っているかを返す。
func DraftHeld(s *PairingService) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.draft != nil
}
