package buildinfo

import "testing"

// リポジトリの版は開発ビルド（"dev"）。リリースのビルドだけが set-version.sh で書き換える。
func TestVersionIsDevInTheRepository(t *testing.T) {
	if Version != DevVersion {
		t.Fatalf("Version = %q, want %q（リリースの版はコミットしない。set-version.sh がビルド時だけ書き換える）", Version, DevVersion)
	}
}
