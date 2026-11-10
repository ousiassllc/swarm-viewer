// Package buildinfo はビルドに埋め込む版（アプリ自身の版）を持つ。
//
// リポジトリの値は開発ビルドを表す "dev" で、リリースのビルドだけが .github/scripts/set-version.sh（release.yml の build-windows・build-macos）で
// タグの X.Y.Z に書き換える（build/config.yml の info.version と同じ流れ。タグが版の唯一の元）。
// "dev" のビルド（`wails3 dev`・手元の `go build`）は、新しい版の確認も自動アップデートもしない（Issue #109）。
package buildinfo

// DevVersion は版が埋め込まれていないビルド（開発ビルド）の値。
const DevVersion = "dev"

// Version はアプリの版。リリースのビルドでは "X.Y.Z"（v なし）。set-version.sh が次の 1 行を書き換えるので、形（var Version = "…"）を変えないこと。
var Version = "dev"
