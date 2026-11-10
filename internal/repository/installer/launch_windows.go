//go:build windows

package installer

import "golang.org/x/sys/windows"

// shellExecute はインストーラを ShellExecute で起動する。NSIS のインストーラは管理者権限を要求することがあり、
// CreateProcess（os/exec）では昇格できないため、昇格の確認を OS が出す ShellExecute を使う。起動後は親プロセスから切り離される。
func shellExecute(file string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

// startRelaunchHelper は macOS 専用。
func startRelaunchHelper(int, string, string) error { return ErrUnsupported }
