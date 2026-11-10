//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// 補助プロセスは、親（pid）の終了を待ってから後始末をする。待っている間は消さない。
func TestRelaunchScriptWaitsForTheParentThenCleansUp(t *testing.T) {
	cleanup := filepath.Join(t.TempDir(), "staging")
	if err := os.MkdirAll(filepath.Join(cleanup, "old.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	parent := exec.Command("sleep", "30")
	if err := parent.Start(); err != nil {
		t.Skipf("sleep が使えない: %v", err)
	}
	defer func() { _ = parent.Process.Kill(); _ = parent.Wait() }()

	helper := exec.Command("/bin/sh", "-c", relaunchScript, "sh", strconv.Itoa(parent.Process.Pid), filepath.Join(t.TempDir(), "no.app"), cleanup) //nolint:gosec // テスト
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- helper.Wait() }()

	// 親が生きている間は終わらず、消しもしない（上限付きの確認。固定の待ちで判定しない）。
	select {
	case err := <-done:
		t.Fatalf("親が生きているのに補助が終わった: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := os.Stat(cleanup); err != nil {
		t.Fatalf("親が生きているのに後始末した: %v", err)
	}

	_ = parent.Process.Kill()
	_ = parent.Wait()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("親の終了後も補助が終わらない")
	}
	if _, err := os.Stat(cleanup); !os.IsNotExist(err) {
		t.Fatalf("後始末されていない: %v", err)
	}
}
