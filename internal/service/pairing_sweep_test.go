package service_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	st "github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
)

// eventually は cond が真になるまで、上限付きで待つ（固定時間の待ちではない）。
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(st.WaitDeadline)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		runtime.Gosched()
	}
}

// 期限切れの下書き（token を含む）は、誰も呼ばなくてもメモリから消える。
func TestPairingExpiredDraftIsPurgedWithoutAnyCall(t *testing.T) {
	e := st.NewEnv(t)
	id, err := e.Pairing.Parse(st.PairingStr)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "sweeper ticker", func() bool { return e.Clock.TickerCount() == 1 })
	tk := e.Clock.Ticker(0)

	// 期限前の tick では消えず、期限に合わせて待ち直す。
	e.Clock.Advance(service.DraftTTL / 2)
	tk.Tick()
	if got := st.Recv(t, tk.Resets, "sweeper reset"); got != service.DraftTTL/2 {
		t.Fatalf("next sweep in %v, want %v", got, service.DraftTTL/2)
	}
	if !service.DraftHeld(e.Pairing) {
		t.Fatal("draft purged before its expiry")
	}
	if _, err := e.Pairing.Draft(id); err != nil {
		t.Fatalf("draft must still be usable: %v", err)
	}

	e.Clock.Advance(service.DraftTTL / 2)
	tk.Tick()
	eventually(t, "expired draft to be purged", func() bool { return !service.DraftHeld(e.Pairing) })
	if _, err := e.Pairing.Draft(id); !errors.Is(err, service.ErrDraftExpired) {
		t.Fatalf("Draft after purge = %v", err)
	}
}

// 下書きを作り直しても（古い下書きの掃除が済んだ後でも）新しい下書きは期限まで使え、期限後に消える。
func TestPairingSweeperRestartsForNewDraft(t *testing.T) {
	e := st.NewEnv(t)
	if _, err := e.Pairing.Parse(st.PairingStr); err != nil {
		t.Fatal(err)
	}
	eventually(t, "first sweeper", func() bool { return e.Clock.TickerCount() == 1 })
	e.Clock.Advance(service.DraftTTL)
	e.Clock.Ticker(0).Tick()
	eventually(t, "first draft purged", func() bool { return !service.DraftHeld(e.Pairing) })
	eventually(t, "first sweeper stopped", func() bool { return e.Clock.Ticker(0).Stopped() })

	if _, err := e.Pairing.Parse(st.PairingStr); err != nil {
		t.Fatal(err)
	}
	eventually(t, "second sweeper", func() bool { return e.Clock.TickerCount() == 2 })
	e.Clock.Advance(service.DraftTTL)
	e.Clock.Ticker(1).Tick()
	eventually(t, "second draft purged", func() bool { return !service.DraftHeld(e.Pairing) })
}

// ペアリング確定の直後に、新しい接続の最初の通知用ポーリングが（周期を待たずに）回り、baselined_at が記録される。
func TestPairingConfirmStartsFirstPollRoundImmediately(t *testing.T) {
	e := st.NewEnv(t)
	rounds := startRun(t, e)
	st.Recv(t, rounds, "startup round (no connections)")

	id, err := e.Pairing.Parse(st.PairingStr)
	if err != nil {
		t.Fatal(err)
	}
	connID := "id-1" // st.IDs の最初の ID
	e.Report.SetSessions(connID, "0")
	e.Report.SetReport(connID, "0", st.ReportOf(st.Pane("%1", "T1", domain.StateAsking)))

	conn, err := e.Pairing.Confirm(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if conn.ID != connID {
		t.Fatalf("conn id = %q", conn.ID)
	}
	st.Recv(t, rounds, "immediate round for the new connection")

	saved, _ := e.Store.Conn(conn.ID)
	if saved.BaselinedAt.IsZero() {
		t.Fatal("baselined_at not recorded by the immediate round")
	}
	if n := len(e.Notifier.Sent()); n != 0 {
		t.Fatalf("notified %d times for the pre-existing state of a new connection", n)
	}
	if got := e.Store.NotifiedSet(conn.ID, "0"); !got[asking("%1")] {
		t.Fatalf("current state not recorded: %v", got)
	}
}

// 通知がオフなら、確定直後でも通知用の取得はしない（通常の一巡と同じ）。
func TestPairingConfirmDoesNotPollWhileNotificationsAreDisabled(t *testing.T) {
	e := st.NewEnv(t)
	e.Store.Cfg.NotificationsEnabled = false
	rounds := startRun(t, e)
	st.Recv(t, rounds, "startup round")

	id, err := e.Pairing.Parse(st.PairingStr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pairing.Confirm(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	st.Recv(t, rounds, "wake round")
	if e.Report.ReportCount() != 0 {
		t.Fatal("fetched while notifications are disabled")
	}
}

// 依頼の前に削除された接続の取得はしない。
func TestWakeForDeletedConnectionFetchesNothing(t *testing.T) {
	e := st.NewEnv(t)
	e.AddConn("c1", "devbox", st.URL1, false)
	e.Report.SetSessions("c1", "0")
	rounds := startRun(t, e)
	st.Recv(t, rounds, "startup round")
	before := e.Report.ReportCount()

	if err := e.Conns.Delete(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	e.Notif.Wake("c1")
	st.Recv(t, rounds, "wake round")

	if e.Report.ReportCount() != before {
		t.Fatal("fetched a deleted connection")
	}
}
