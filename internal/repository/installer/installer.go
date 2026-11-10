// Package installer は検証済みの更新ファイルを OS ごとに適用する（Issue #109）。
//
//   - Windows: ダウンロードした NSIS インストーラを起動する（ShellExecute。管理者権限の確認は OS が出す）。起動したら呼び出し側がアプリを終了する
//   - macOS: zip を実行中の .app と同じボリュームへ展開して検証し、.app を差し替え、アプリの終了を待って開き直す補助プロセスを起動する
//   - それ以外（Linux など）: 更新ファイルが無いので Install は ErrUnsupported
//
// ハッシュの検証は呼び出し側（service/updater）が済ませてから渡す。ここではファイルシステムと OS の起動を、注入できる関数の裏に置いて
// Linux でもテストできるようにしている（実際のインストーラ起動・実機の .app の差し替えは各 OS でしか確認できない）。
package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
)

// ErrUnsupported はこの OS には更新の適用が無い。
var ErrUnsupported = errors.New("installer: no update installer for this OS")

// Installer は更新ファイルの適用。
type Installer struct {
	goos string
	// launchWindows はインストーラを切り離して起動する（Windows の ShellExecute）。
	launchWindows func(file string) error
	// executable は実行中のバイナリのパス（os.Executable）。
	executable func() (string, error)
	// relaunch は macOS で、アプリ（pid）の終了を待って app を開き、cleanup を消す補助プロセスを起動する。
	relaunch func(pid int, app, cleanup string) error
	// rename は .app の差し替えに使う（テストで失敗を作る）。
	rename func(oldPath, newPath string) error
}

// New は実行中の OS 向けの Installer。
func New() *Installer {
	return &Installer{
		goos:          runtime.GOOS,
		launchWindows: shellExecute,
		executable:    os.Executable,
		relaunch:      startRelaunchHelper,
		rename:        os.Rename,
	}
}

// Install は検証済みの file を適用する。成功したら、呼び出し側がアプリを終了する。失敗しても実行中のアプリは変更されない
// （macOS は差し替えの途中で失敗したら元へ戻す）。
func (i *Installer) Install(ctx context.Context, file string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch i.goos {
	case "windows":
		return i.installWindows(file)
	case "darwin":
		return i.installMacOS(file)
	default:
		return ErrUnsupported
	}
}

func (i *Installer) installWindows(file string) error {
	if st, err := os.Stat(file); err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("installer: the installer file is not available: %w", errOrNotRegular(err))
	}
	if err := i.launchWindows(file); err != nil {
		return fmt.Errorf("installer: launching the installer: %w", err)
	}
	return nil
}

func errOrNotRegular(err error) error {
	if err != nil {
		return err
	}
	return errors.New("not a regular file")
}
