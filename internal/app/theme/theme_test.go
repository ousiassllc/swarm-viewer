package theme

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

// Bone が app.css の --bone（:root のライトと、prefers-color-scheme: dark のダーク）と同じ値であること。
func TestBoneMatchesAppCSS(t *testing.T) {
	css, err := os.ReadFile("../../../static/src/css/app.css")
	if err != nil {
		t.Fatalf("app.css を読めない: %v", err)
	}
	// 宣言は ライト → ダーク の順に 1 回ずつ（--bone: #xxxxxx;）。
	matches := regexp.MustCompile(`(?m)^\s*--bone:\s*#([0-9a-fA-F]{6});`).FindAllSubmatch(css, -1)
	if len(matches) != 2 {
		t.Fatalf("app.css の --bone 宣言が %d 件（ライトとダークの 2 件を想定）", len(matches))
	}
	for i, dark := range []bool{false, true} {
		c := Bone(dark)
		got := fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B)
		if want := string(matches[i][1]); got != want {
			t.Errorf("Bone(dark=%v) = #%s、app.css の --bone は #%s", dark, got, want)
		}
	}
}
