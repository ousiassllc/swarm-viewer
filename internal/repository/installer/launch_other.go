//go:build !windows

package installer

import (
	"os/exec"
	"strconv"
	"syscall"
)

// shellExecute は Windows 専用。
func shellExecute(string) error { return ErrUnsupported }

// relaunchScript は pid のプロセスが消えるまで待ち、app を開き、cleanup（差し替え前の .app を含む作業用フォルダ）を消す。
// 値はすべて引数（$1〜$3）で渡し、シェルの文字列に埋め込まない。
const relaunchScript = `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done; /usr/bin/open "$2"; rm -rf "$3"`

// startRelaunchHelper はアプリの終了を待って開き直す補助プロセスを、親（このアプリ）から切り離して起動する。
// 単一インスタンスのロックがあるため、終了を待たずに開くと 2 つ目の起動として転送されて終わってしまう。
func startRelaunchHelper(pid int, app, cleanup string) error {
	cmd := exec.Command("/bin/sh", "-c", relaunchScript, "sh", strconv.Itoa(pid), app, cleanup) //nolint:gosec // 引数はパスと pid だけで、シェルの文字列には展開しない
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
