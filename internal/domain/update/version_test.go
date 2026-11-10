package update

import "testing"

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Version
		ok   bool
	}{
		{"v0.1.9", Version{0, 1, 9}, true},
		{"v0.1.13", Version{0, 1, 13}, true},
		{"v10.20.30", Version{10, 20, 30}, true},
		{"v0.0.0", Version{0, 0, 0}, true},
		{"0.1.9", Version{}, false},      // v なし
		{"V0.1.9", Version{}, false},     // 大文字
		{"v0.1", Version{}, false},       // 成分が足りない
		{"v0.1.9.1", Version{}, false},   // 余分な成分
		{"v01.1.9", Version{}, false},    // 先頭の 0
		{"v0.1.09", Version{}, false},    // 先頭の 0
		{"v0.1.9-rc1", Version{}, false}, // プレリリース形
		{"v0.1.9+meta", Version{}, false},
		{"v0.1.-1", Version{}, false},
		{"v0.1.+1", Version{}, false},
		{"v0. 1.9", Version{}, false},
		{"v0.1.", Version{}, false},
		{"v..", Version{}, false},
		{"v0.1.1000000000", Version{}, false}, // 桁あふれ
		{"v99999999999999999999.0.0", Version{}, false},
		{"latest", Version{}, false},
		{"", Version{}, false},
		{"v", Version{}, false},
		{"v0.1.９", Version{}, false}, // 全角数字
	} {
		got, ok := Parse(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("Parse(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := (Version{0, 1, 13}).String(); got != "v0.1.13" {
		t.Fatalf("String = %q", got)
	}
}

// 版の比較表（数値で比べる。v0.1.9 は v0.1.13 より古い）。読めない版は通知しない。
func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		name, current, latest string
		want                  bool
	}{
		{"v0.1.9 → v0.1.13 は新しい（数値比較。文字列比較なら逆）", "v0.1.9", "v0.1.13", true},
		{"v0.1.13 → v0.1.9 は古い", "v0.1.13", "v0.1.9", false},
		{"同じ版は通知しない", "v0.1.13", "v0.1.13", false},
		{"パッチが上がる", "v0.1.0", "v0.1.1", true},
		{"マイナーが上がる（パッチは下がる）", "v0.1.9", "v0.2.0", true},
		{"メジャーが上がる", "v0.9.9", "v1.0.0", true},
		{"メジャーが下がる", "v1.0.0", "v0.9.9", false},
		{"マイナーが桁上がりしても数値で比べる", "v0.9.0", "v0.10.0", true},
		{"最新のタグが v なし（読めない）", "v0.1.0", "0.1.1", false},
		{"最新のタグがゴミ", "v0.1.0", "nightly", false},
		{"最新のタグがプレリリース形", "v0.1.0", "v0.2.0-rc1", false},
		{"最新のタグが空", "v0.1.0", "", false},
		{"現在の版が dev（読めない）なら通知しない", "dev", "v9.9.9", false},
		{"現在の版が空なら通知しない", "", "v9.9.9", false},
	} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("%s: Newer(%q, %q) = %v, want %v", tc.name, tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestCompare(t *testing.T) {
	a, b := Version{0, 1, 9}, Version{0, 1, 13}
	if Compare(a, b) != -1 || Compare(b, a) != 1 || Compare(a, a) != 0 {
		t.Fatalf("Compare: %d %d %d", Compare(a, b), Compare(b, a), Compare(a, a))
	}
}
