// Package update は新しい版の確認と自動アップデートの純粋なロジック（版の比較・Release JSON・SHA256SUMS・許可するホスト）。
// ネットワーク・ファイル・OS に触れない（Issue #109。docs/architecture/overview.md §4.2 の domain）。
package update

import (
	"strconv"
	"strings"
)

// Version は vMAJOR.MINOR.PATCH の版。
type Version struct{ Major, Minor, Patch int }

// maxComponentDigits は 1 成分の桁数の上限（int に収まり、悪意のある巨大な数で壊れないため）。
const maxComponentDigits = 9

// Parse は "vX.Y.Z"（数字だけ・先頭の 0 なし・成分は 3 つ・v 必須）を読む。形が違えば ok=false。
// リリースのタグ（tag_name）はこの形だけを受け付ける。現在の版も "v" を付けてこの関数で読む。
func Parse(tag string) (Version, bool) {
	rest, found := strings.CutPrefix(tag, "v")
	if !found {
		return Version{}, false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, ok := component(p)
		if !ok {
			return Version{}, false
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, true
}

// component は 10 進の数字だけ（先頭の 0 は "0" 自身のみ）の成分を読む。符号・空・空白は不可。
func component(p string) (int, bool) {
	if p == "" || len(p) > maxComponentDigits || (len(p) > 1 && p[0] == '0') {
		return 0, false
	}
	for _, r := range p {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(p)
	return n, err == nil
}

// String は "vX.Y.Z"。
func (v Version) String() string {
	return "v" + strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// Compare は成分ごとに数値で比べる（v0.1.9 < v0.1.13）。a<b は -1、等しければ 0、a>b は 1。
func Compare(a, b Version) int {
	for _, d := range [3]int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}

// Newer は latest が current より新しいかを返す。どちらかが読めなければ false（通知しない）。
func Newer(current, latest string) bool {
	cur, ok := Parse(current)
	if !ok {
		return false
	}
	lat, ok := Parse(latest)
	if !ok {
		return false
	}
	return Compare(lat, cur) > 0
}
