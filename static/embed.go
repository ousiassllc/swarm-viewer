// Package static は、ビルド成果物（static/dist/）をバイナリへ埋め込む。
//
// dist/ は生成物でコミットしない。go build / vet / test / lint の前に
// `task generate` で作ること（docs/environment/setup.md §3.2）。
package static

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

// FS は dist/ を根とする埋め込みアセットを返す。
// `/static/js/app.js` は FS 上では `js/app.js` に対応する。
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// "dist" は固定の正当なパスなので、ここには到達しない。
		panic("static: dist の部分木を取り出せない: " + err.Error())
	}
	return sub
}
