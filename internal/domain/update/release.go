package update

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Release の読み取りの失敗。
var (
	// ErrIgnoredRelease は draft または prerelease のリリース。更新の対象にしない（エラーではない）。
	ErrIgnoredRelease = errors.New("update: release is a draft or a prerelease")
	// ErrBadRelease は JSON として読めない、または tag_name が vX.Y.Z でない。
	ErrBadRelease = errors.New("update: unreadable release")
	// ErrAssetMissing はリリースに目的のアセットが無い（同名が複数あるときも含む）。
	ErrAssetMissing = errors.New("update: release asset is missing or ambiguous")
)

// Asset はリリースに添付されたファイル 1 つ。
type Asset struct {
	Name string
	// URL は browser_download_url。取得の前に CheckURL で許可するホストの https か確かめる。
	URL string
	// Size は GitHub が報告するバイト数。0 は不明。
	Size int64
}

// Release は更新の元になるリリース。
type Release struct {
	// Tag は tag_name（vX.Y.Z）。
	Tag     string
	Version Version
	Assets  []Asset
}

type releaseJSON struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// ParseRelease は releases/latest の JSON を読む。draft・prerelease は ErrIgnoredRelease（GitHub の latest は除外済みだが二重に守る）。
// JSON が壊れている・tag_name が vX.Y.Z でない（v なし・余分な成分・先頭の 0・文字列）ときは ErrBadRelease。
func ParseRelease(body []byte) (Release, error) {
	var j releaseJSON
	if err := json.Unmarshal(body, &j); err != nil {
		return Release{}, fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	if j.Draft || j.Prerelease {
		return Release{}, ErrIgnoredRelease
	}
	v, ok := Parse(j.TagName)
	if !ok {
		return Release{}, ErrBadRelease
	}
	r := Release{Tag: j.TagName, Version: v}
	for _, a := range j.Assets {
		r.Assets = append(r.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return r, nil
}

// Asset は名前がちょうど name のアセットを返す。無い・同名が複数あるときは ErrAssetMissing。
func (r Release) Asset(name string) (Asset, error) {
	var found []Asset
	for _, a := range r.Assets {
		if a.Name == name {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		return Asset{}, ErrAssetMissing
	}
	return found[0], nil
}
