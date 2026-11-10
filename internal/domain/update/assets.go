package update

// アセット名の接頭辞（TAG は tag_name）。release.yml が付ける名前と同じ。
const (
	assetPrefix = "swarm-viewer_"
	// SumsName は SHA256SUMS（sha256sum 形式。他のアセットのハッシュも載る）。
	SumsName = "SHA256SUMS"
)

// AssetName は goos 用の更新ファイル名。Windows は NSIS インストーラ、macOS はユニバーサルの .app の zip。
// 更新ファイルが無い OS（Linux など）は ok=false（更新を案内しない）。
func AssetName(tag, goos string) (name string, ok bool) {
	switch goos {
	case "windows":
		return assetPrefix + tag + "_windows_amd64_installer.exe", true
	case "darwin":
		return assetPrefix + tag + "_darwin_universal.zip", true
	default:
		return "", false
	}
}
