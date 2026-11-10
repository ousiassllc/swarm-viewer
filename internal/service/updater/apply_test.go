package updater

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

// (f) 正常系: SHA256SUMS の行と一致したファイルだけを、インストーラへちょうど 1 回渡し、そのあとアプリを終了する。
func TestApplyHappyPathInstallsOnceAndQuits(t *testing.T) {
	for goos, asset := range map[string]string{"windows": winAsset, "darwin": macAsset} {
		e := newEnv(t, "0.1.9", goos, true)
		e.src.latest = relOf(testTag, asset, update.SumsName)
		e.src.withFiles(asset, payloadBody)
		e.svc.Check(t.Context())
		if err := e.svc.Apply(t.Context()); err != nil {
			t.Fatal(err)
		}
		files := e.inst.calls()
		if len(files) != 1 || !strings.HasSuffix(files[0], asset) || e.inst.bodies[0] != payloadBody {
			t.Fatalf("%s: install calls = %v bodies = %q", goos, files, e.inst.bodies)
		}
		recv(t, e.quits, "終了")
		if len(e.quits) != 0 {
			t.Fatalf("%s: 終了が複数回呼ばれた", goos)
		}
		got := e.src.downloaded()
		if len(got) != 2 || got[0] != update.SumsName || got[1] != asset {
			t.Fatalf("%s: 取得したもの = %v（SHA256SUMS とこの OS のアセットだけ）", goos, got)
		}
		if st := e.svc.Status(); !st.Updating || st.ApplyFailed != FailNone {
			t.Fatalf("%s: status = %+v（終了までは更新中のまま）", goos, st)
		}
		_, kept := os.Stat(files[0])
		if (goos == "windows") != (kept == nil) {
			t.Errorf("%s: インストーラのファイルの扱い（Windows は残す・macOS は消す）: stat err = %v", goos, kept)
		}
	}
}

// 失敗の共通の確認: インストーラは呼ばれず、終了せず、取得したファイルは消え、状態は「失敗」になって更新を押し直せる。
func assertNotInstalled(t *testing.T, e *env, want FailKind) {
	t.Helper()
	if n := len(e.inst.calls()); n != 0 {
		t.Errorf("インストーラが %d 回呼ばれた", n)
	}
	if len(e.quits) != 0 {
		t.Error("アプリが終了した")
	}
	if n := e.leftovers(t); n != 0 {
		t.Errorf("取得したファイルのフォルダが %d 個残っている", n)
	}
	st := e.svc.Status()
	if st.Updating || st.ApplyFailed != want || !st.Available {
		t.Errorf("status = %+v, want 失敗 %q・更新中でない・通知は残る", st, want)
	}
}

func prepared(t *testing.T) *env {
	t.Helper()
	e := newEnv(t, "0.1.9", "windows", true)
	e.src.latest = relOf(testTag, winAsset, update.SumsName)
	e.src.withFiles(winAsset, payloadBody)
	e.svc.Check(t.Context())
	return e
}

// (a) ハッシュの不一致: インストールせず、ファイルを消す。
func TestApplyHashMismatchDoesNotInstallAndDeletesFile(t *testing.T) {
	e := prepared(t)
	e.src.files[winAsset] = fileResult{body: "tampered-bytes"} // SHA256SUMS は元の本文のハッシュ
	if err := e.svc.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertNotInstalled(t, e, FailVerify)
}

// (b) SHA256SUMS に該当行が無い・書式が壊れている・重複: インストールしない。
func TestApplyBadSumsDoesNotInstall(t *testing.T) {
	for name, sums := range map[string]string{
		"該当行が無い（別のファイルの行だけ）": sumOf(payloadBody) + "  swarm-viewer_v0.1.13_darwin_universal.zip\n",
		"空":              "",
		"書式が壊れている":       "<html>not sums</html>\n",
		"ハッシュが短い":        "abc  " + winAsset + "\n",
		"名前の前後に空白":       sumOf(payloadBody) + "   " + winAsset + "\n",
		"同名の行が重複":        sumOf(payloadBody) + "  " + winAsset + "\n" + sumOf("x") + "  " + winAsset + "\n",
		"ファイル名の大文字小文字違い": sumOf(payloadBody) + "  " + strings.ToUpper(winAsset) + "\n",
	} {
		e := prepared(t)
		e.src.files[update.SumsName] = fileResult{body: sums}
		if err := e.svc.Apply(t.Context()); err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) { assertNotInstalled(t, e, FailVerify) })
	}
}

// アセットや SHA256SUMS がリリースに無い（または同名が複数）なら、取得もせずインストールしない。
func TestApplyMissingAssetDoesNotInstall(t *testing.T) {
	for name, assets := range map[string][]string{
		"この OS のアセットが無い": {update.SumsName, macAsset},
		"SHA256SUMS が無い": {winAsset},
		"アセットが重複":        {winAsset, winAsset, update.SumsName},
		"SHA256SUMS が重複": {winAsset, update.SumsName, update.SumsName},
		"アセットが 1 つも無い":   {},
		"名前が少し違う（x86 版）": {strings.Replace(winAsset, "amd64", "x86", 1), update.SumsName},
	} {
		e := newEnv(t, "0.1.9", "windows", true)
		e.src.latest = relOf(testTag, assets...)
		e.src.withFiles(winAsset, payloadBody)
		e.svc.Check(t.Context())
		if err := e.svc.Apply(t.Context()); err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			assertNotInstalled(t, e, FailVerify)
			if d := e.src.downloaded(); len(d) != 0 {
				t.Errorf("取得を始めた: %v", d)
			}
		})
	}
}

// (d) オフライン・HTTP エラー・途中で切れた取得: インストールせず、途中のファイルを消す。画面に出すのは小さな失敗の表示だけ。
func TestApplyDownloadFailuresDoNotInstall(t *testing.T) {
	cut := errors.New("unexpected EOF")
	for name, mod := range map[string]func(e *env){
		"オフライン（アセット）":          func(e *env) { e.src.files[winAsset] = fileResult{err: errors.New("offline")} },
		"アセットが 404":            func(e *env) { delete(e.src.files, winAsset) },
		"アセットが途中で切れた（先頭の半分だけ）": func(e *env) { e.src.files[winAsset] = fileResult{body: payloadBody, partial: true, err: cut} },
		"SHA256SUMS が取得できない":   func(e *env) { delete(e.src.files, update.SumsName) },
		"SHA256SUMS が途中で切れた":   func(e *env) { e.src.files[update.SumsName] = fileResult{body: "x", err: cut} },
	} {
		e := prepared(t)
		mod(e)
		if err := e.svc.Apply(t.Context()); err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) { assertNotInstalled(t, e, FailDownload) })
	}
}

// 途中で切れて本文が半分でも、その半分が SHA256SUMS と偶然一致することは無いので、切れた取得は必ずエラーとして扱われる（エラーを返さない偽物でも、ハッシュで落ちる）。
func TestApplyTruncatedBodyWithoutErrorIsCaughtByHash(t *testing.T) {
	e := prepared(t)
	e.src.files[winAsset] = fileResult{body: payloadBody, partial: true} // エラーなしで半分だけ書く偽物
	if err := e.svc.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertNotInstalled(t, e, FailVerify)
}

// 検証を通ってもインストーラの起動に失敗したら、終了せず、ファイルを消し、失敗として表示する。
func TestApplyInstallerFailure(t *testing.T) {
	e := prepared(t)
	e.inst.err = errors.New("uac declined")
	if err := e.svc.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(e.inst.calls()) != 1 {
		t.Fatalf("install calls = %v", e.inst.calls())
	}
	assertNotInstalledAfterInstallAttempt(t, e)
}

func assertNotInstalledAfterInstallAttempt(t *testing.T, e *env) {
	t.Helper()
	if len(e.quits) != 0 {
		t.Error("アプリが終了した")
	}
	if n := e.leftovers(t); n != 0 {
		t.Errorf("フォルダが %d 個残っている", n)
	}
	if st := e.svc.Status(); st.Updating || st.ApplyFailed != FailInstall {
		t.Errorf("status = %+v", st)
	}
}

// 失敗のあと、もう一度押せる（失敗の表示は消える）。
func TestApplyRetryAfterFailure(t *testing.T) {
	e := prepared(t)
	good := e.src.files[winAsset]
	e.src.files[winAsset] = fileResult{body: "tampered"}
	_ = e.svc.Apply(t.Context())
	assertNotInstalled(t, e, FailVerify)
	e.src.files[winAsset] = good
	if err := e.svc.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(e.inst.calls()) != 1 {
		t.Fatalf("install calls = %v", e.inst.calls())
	}
	recv(t, e.quits, "終了")
}

// 新しい版が無いときに更新を押しても、何もしない（ErrNotAvailable）。
func TestApplyWithoutUpdate(t *testing.T) {
	e := newEnv(t, "0.1.13", "windows", true)
	e.src.latest = relOf(testTag, winAsset, update.SumsName)
	e.svc.Check(t.Context())
	if err := e.svc.Apply(t.Context()); !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("err = %v", err)
	}
	if len(e.src.downloaded()) != 0 || len(e.inst.calls()) != 0 {
		t.Fatal("何かを取得・適用した")
	}
}

// 更新中に重ねて押しても、二重に更新しない。更新中は確認も走らない。ApplyAsync は終わりを知らせる。
func TestApplyAsyncIsSingleFlight(t *testing.T) {
	e := prepared(t)
	gate := make(chan struct{})
	e.src.files[winAsset] = fileResult{body: payloadBody}
	blocking := &blockingSource{fakeSource: e.src, gate: gate, started: make(chan struct{}, 4)}
	e.svc.opts.Source = blocking

	done, err := e.svc.ApplyAsync()
	if err != nil {
		t.Fatal(err)
	}
	recv(t, blocking.started, "取得の開始")
	if _, err := e.svc.ApplyAsync(); !errors.Is(err, ErrBusy) {
		t.Fatalf("2 つ目: err = %v, want ErrBusy", err)
	}
	if err := e.svc.Apply(t.Context()); !errors.Is(err, ErrBusy) {
		t.Fatalf("Apply: err = %v, want ErrBusy", err)
	}
	e.svc.Check(t.Context()) // 更新中は確認しない
	if n := e.src.latestCalls(); n != 1 {
		t.Fatalf("更新中に確認した: %d", n)
	}
	if !e.svc.Status().Updating {
		t.Fatal("更新中の状態でない")
	}
	close(gate)
	recv(t, done, "更新の終わり")
	if len(e.inst.calls()) != 1 {
		t.Fatalf("install calls = %v", e.inst.calls())
	}
	recv(t, e.quits, "終了")
}

// blockingSource は gate が開くまで 2 つ目（アセット）の取得を止める。
type blockingSource struct {
	*fakeSource
	gate    chan struct{}
	started chan struct{}
}

func (b *blockingSource) Download(ctx context.Context, a update.Asset, dst io.Writer) error {
	if a.Name != update.SumsName {
		b.started <- struct{}{}
		<-b.gate
	}
	return b.fakeSource.Download(ctx, a, dst)
}

// 取得の進み具合（バイト数と全体）が状態に出る。
func TestApplyReportsProgress(t *testing.T) {
	e := prepared(t)
	e.src.latest.Assets[0].Size = int64(len(payloadBody))
	e.svc.Check(t.Context()) // Size の入ったリリースを取り込み直す
	probe := &progressProbe{fakeSource: e.src, svc: e.svc}
	e.svc.opts.Source = probe
	if err := e.svc.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if probe.during.Total != int64(len(payloadBody)) || probe.during.Done != int64(len(payloadBody)) {
		t.Fatalf("取得後の状態 = %+v", probe.during)
	}
}

type progressProbe struct {
	*fakeSource
	svc    *Service
	during Status
}

func (p *progressProbe) Download(ctx context.Context, a update.Asset, dst io.Writer) error {
	err := p.fakeSource.Download(ctx, a, dst)
	if a.Name != update.SumsName {
		p.during = p.svc.Status()
	}
	return err
}
