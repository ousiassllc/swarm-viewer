// Package theme はデスクトップのウィンドウが使う色の定数（Wails に依存しないので Linux の CI でもテストできる）。
package theme

// RGB は不透明な sRGB の色。
type RGB struct{ R, G, B uint8 }

// Bone はウィンドウ背景の色。static/src/css/app.css の --bone（ライト・ダーク）と同じ値にする
// （WebView が最初の描画をする前、リサイズ中に見える面を、ページの背景と揃えるため。ガイド C5）。
// 値の一致は theme_test.go が app.css を読んで確かめる。
func Bone(dark bool) RGB {
	if dark {
		return RGB{0x05, 0x05, 0x05}
	}
	return RGB{0xF5, 0xF5, 0xF4}
}
