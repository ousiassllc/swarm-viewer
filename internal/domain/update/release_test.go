package update

import (
	"errors"
	"testing"
)

const goodRelease = `{
  "tag_name": "v0.1.13", "draft": false, "prerelease": false,
  "assets": [
    {"name": "swarm-viewer_v0.1.13_darwin_universal.zip", "browser_download_url": "https://github.com/o/r/releases/download/v0.1.13/swarm-viewer_v0.1.13_darwin_universal.zip", "size": 1234},
    {"name": "SHA256SUMS", "browser_download_url": "https://github.com/o/r/releases/download/v0.1.13/SHA256SUMS", "size": 100}
  ]
}`

func TestParseRelease(t *testing.T) {
	r, err := ParseRelease([]byte(goodRelease))
	if err != nil {
		t.Fatal(err)
	}
	if r.Tag != "v0.1.13" || r.Version != (Version{0, 1, 13}) || len(r.Assets) != 2 {
		t.Fatalf("release = %+v", r)
	}
	a, err := r.Asset("swarm-viewer_v0.1.13_darwin_universal.zip")
	if err != nil || a.Size != 1234 || a.URL == "" {
		t.Fatalf("asset = %+v, %v", a, err)
	}
}

func TestParseReleaseRejects(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"draft は対象外", `{"tag_name":"v0.1.13","draft":true}`, ErrIgnoredRelease},
		{"prerelease は対象外", `{"tag_name":"v0.1.13","prerelease":true}`, ErrIgnoredRelease},
		{"v なしのタグ", `{"tag_name":"0.1.13"}`, ErrBadRelease},
		{"余分な成分のタグ", `{"tag_name":"v0.1.13.1"}`, ErrBadRelease},
		{"ゴミのタグ", `{"tag_name":"nightly"}`, ErrBadRelease},
		{"タグが無い", `{}`, ErrBadRelease},
		{"JSON でない", `<html>`, ErrBadRelease},
		{"空", ``, ErrBadRelease},
		{"タグが文字列でない", `{"tag_name":13}`, ErrBadRelease},
	} {
		if _, err := ParseRelease([]byte(tc.body)); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestReleaseAssetMissingOrAmbiguous(t *testing.T) {
	r, _ := ParseRelease([]byte(goodRelease))
	if _, err := r.Asset("swarm-viewer_v0.1.13_windows_amd64_installer.exe"); !errors.Is(err, ErrAssetMissing) {
		t.Fatalf("missing: err = %v", err)
	}
	r.Assets = append(r.Assets, r.Assets[0])
	if _, err := r.Asset(r.Assets[0].Name); !errors.Is(err, ErrAssetMissing) {
		t.Fatalf("duplicate: err = %v", err)
	}
}

func TestAssetName(t *testing.T) {
	for goos, want := range map[string]string{
		"windows": "swarm-viewer_v0.1.13_windows_amd64_installer.exe",
		"darwin":  "swarm-viewer_v0.1.13_darwin_universal.zip",
	} {
		if got, ok := AssetName("v0.1.13", goos); !ok || got != want {
			t.Errorf("AssetName(%s) = %q, %v; want %q", goos, got, ok, want)
		}
	}
	if name, ok := AssetName("v0.1.13", "linux"); ok || name != "" {
		t.Errorf("linux には更新ファイルが無い: %q, %v", name, ok)
	}
}
