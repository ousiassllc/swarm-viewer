package static_test

import (
	"regexp"
	"testing"
)

var muteRE = regexp.MustCompile(`--mute:\s*(#[0-9a-fA-F]{3,8})`)

// --mute はライトとダークで値が違う（ガイド C6: Light #6b6b6e / Dark #8b8b8e）。
// ライトの #8b8b8e は bone・surface 上で通常テキストの AA（4.5:1）に届かない。
func TestAppCSS_MuteDiffersBetweenLightAndDark(t *testing.T) {
	t.Parallel()

	dark, rest := darkBlocksAndRest(t, readAppCSS(t))
	light := muteRE.FindStringSubmatch(rest)
	if light == nil || light[1] != "#6b6b6e" {
		t.Errorf("ライトの --mute は #6b6b6e: %v", light)
	}
	darkValue := muteRE.FindStringSubmatch(dark)
	if darkValue == nil || darkValue[1] != "#8b8b8e" {
		t.Errorf("ダークの --mute は #8b8b8e: %v", darkValue)
	}
}
