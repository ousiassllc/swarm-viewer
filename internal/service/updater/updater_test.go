package updater

import (
	"errors"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

// 新しい版があれば Available（通知の対象）になる。比較は数値（v0.1.9 < v0.1.13）。
func TestCheckSetsAvailableWhenNewer(t *testing.T) {
	for _, tc := range []struct {
		name, current, latest string
		want                  bool
	}{
		{"v0.1.9 → v0.1.13", "0.1.9", "v0.1.13", true},
		{"同じ版", "0.1.13", "v0.1.13", false},
		{"古い版", "0.1.14", "v0.1.13", false},
		{"v なしのタグ（読めない）", "0.1.0", "0.1.13", false},
		{"ゴミのタグ", "0.1.0", "nightly", false},
	} {
		e := newEnv(t, tc.current, "windows", true)
		e.src.latest = relOf(tc.latest, winAsset, update.SumsName)
		e.svc.Check(t.Context())
		if got := e.svc.Status(); got.Available != tc.want || (tc.want && got.Latest != tc.latest) {
			t.Errorf("%s: status = %+v", tc.name, got)
		}
	}
}

// draft・prerelease・不正なタグ・リリース無しは、エラー表示にもならず、通知もしない。
func TestCheckIgnoresUnusableReleases(t *testing.T) {
	for _, err := range []error{update.ErrIgnoredRelease, update.ErrBadRelease} {
		e := newEnv(t, "0.1.0", "windows", true)
		e.src.latestErr = err
		e.svc.Check(t.Context())
		if got := e.svc.Status(); got.Available || got.CheckFailed {
			t.Errorf("%v: status = %+v", err, got)
		}
	}
}

// (d) オフライン・HTTP エラーは、小さな「確認失敗」の状態になるだけで、更新も通知もしない。次の成功で消える。
func TestCheckFailureIsSmallAndRecovers(t *testing.T) {
	e := newEnv(t, "0.1.0", "windows", true)
	e.src.latestErr = errors.New("offline")
	e.svc.Check(t.Context())
	if got := e.svc.Status(); !got.CheckFailed || got.Available {
		t.Fatalf("status = %+v", got)
	}
	e.src.latestErr = nil
	e.src.latest = relOf(testTag, winAsset, update.SumsName)
	e.svc.Check(t.Context())
	if got := e.svc.Status(); got.CheckFailed || !got.Available {
		t.Fatalf("status = %+v", got)
	}
}

// 開発ビルド（版が dev）・読めない版・更新ファイルの無い OS（Linux）は、確認もしないし通知もしない。
func TestDisabledEnvironmentsNeverCheck(t *testing.T) {
	for _, tc := range []struct{ name, version, goos string }{
		{"dev ビルド", "dev", "windows"},
		{"空の版", "", "darwin"},
		{"読めない版", "1.2", "windows"},
		{"v 付きの版（埋め込みは v なし）", "v0.1.0", "windows"},
		{"Linux", "0.1.0", "linux"},
	} {
		e := newEnv(t, tc.version, tc.goos, true)
		e.src.latest = relOf("v9.9.9", winAsset, macAsset, update.SumsName)
		if e.svc.Enabled() {
			t.Errorf("%s: Enabled", tc.name)
		}
		e.svc.Check(t.Context())
		stop := e.runInBackground(t)
		stop()
		if e.src.latestCalls() != 0 || e.svc.Status().Available {
			t.Errorf("%s: 通信した・通知した（calls=%d）", tc.name, e.src.latestCalls())
		}
		if err := e.svc.Apply(t.Context()); !errors.Is(err, ErrNotAvailable) {
			t.Errorf("%s: Apply err = %v", tc.name, err)
		}
	}
}

// (g) 自動で確認する = オフ: 起動時も定期（ticker）もネットワークに出ない。オンへ変えると、すぐ 1 回確認する。
func TestAutoCheckOffMakesNoNetworkCall(t *testing.T) {
	e := newEnv(t, "0.1.0", "windows", false)
	e.src.latest = relOf(testTag, winAsset, update.SumsName)
	stop := e.runInBackground(t)
	waitTickers(t, e) // Run が起動の確認を済ませて ticker を作った
	tk := e.clock.Ticker(0)
	if tk.Interval() != CheckInterval || CheckInterval != 6*time.Hour {
		t.Fatalf("interval = %v", tk.Interval())
	}
	for range 3 { // バッファ 1 なので、3 回目が返るとき、少なくとも 2 回は受け取り済み
		tk.Tick()
	}
	stop() // Run の終了まで待つ
	if n := e.src.latestCalls(); n != 0 {
		t.Fatalf("オフなのに %d 回確認した", n)
	}
	if e.svc.Status().Available {
		t.Fatal("オフなのに通知の状態になった")
	}
}

// 自動で確認する = オン: 起動時に 1 回、以降は 6 時間ごと。オフにすると止まり、オンへ戻すとすぐ確認する。
func TestAutoCheckOnChecksAtStartupAndEverySixHours(t *testing.T) {
	e := newEnv(t, "0.1.0", "windows", true)
	e.src.latest = relOf(testTag, winAsset, update.SumsName)
	stop := e.runInBackground(t)
	defer stop()
	recv(t, e.src.latestHit, "起動時の確認")
	waitTickers(t, e)
	tk := e.clock.Ticker(0)

	tk.Tick()
	recv(t, e.src.latestHit, "定期の確認")
	if !e.svc.Status().Available {
		t.Fatal("通知の状態にならない")
	}

	e.settings.Push(false)
	for range 3 {
		tk.Tick()
	}
	if len(e.src.latestHit) != 0 {
		t.Fatal("オフにしたのに確認した")
	}

	e.settings.Push(true) // オフ → オン
	recv(t, e.src.latestHit, "オンに戻したときの確認")
}

func waitTickers(t *testing.T, e *env) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for e.clock.TickerCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("ticker が作られない")
		case <-time.After(time.Millisecond):
		}
	}
}

func recv[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("%s を待ちきれない", what)
		panic("unreachable")
	}
}
