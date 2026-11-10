package update

import (
	"net/url"
	"slices"
	"testing"
)

// 許可するのは GitHub の API と Releases の配信元への https だけ。tailnet・プライベート・ローカルは通らない。
func TestCheckURL(t *testing.T) {
	for _, raw := range []string{
		LatestReleaseURL,
		"https://github.com/ousiassllc/swarm-viewer/releases/download/v0.1.13/SHA256SUMS",
		"https://objects.githubusercontent.com/github-production-release-asset/1/2?x=y",
		"https://release-assets.githubusercontent.com/github-production-release-asset/1/2?x=y",
		"https://API.GitHub.com/repos/x",
	} {
		if err := CheckURL(mustURL(t, raw)); err != nil {
			t.Errorf("CheckURL(%q) = %v, want nil", raw, err)
		}
	}
	for _, raw := range []string{
		"http://api.github.com/repos/x",                // https でない
		"http://github.com/x",                          // https でない
		"https://evil.example/x",                       // 許可外
		"https://github.com.evil.example/x",            // 接尾辞だけ一致
		"https://evilgithub.com/x",                     // 接頭辞だけ一致
		"https://raw.githubusercontent.com/x",          // 許可外の githubusercontent
		"https://gist.github.com/x",                    // サブドメインは不可
		"https://api.github.com:8443/x",                // ポート指定
		"https://user:pass@api.github.com/x",           // userinfo
		"https://100.64.52.167/report",                 // tailnet
		"https://100.64.52.167:48120/ops",              // tailnet（console の形）
		"https://192.168.0.1/x",                        // プライベート
		"https://localhost/x",                          // ローカル
		"https://127.0.0.1/x",                          // ローカル
		"https://[::1]/x",                              // ローカル
		"ftp://github.com/x",                           // 別のスキーム
		"file:///etc/passwd",                           // ファイル
		"//github.com/x",                               // スキームなし
		"https:github.com/x",                           // Opaque
		"https://100.64.52.167@api.github.com@x.com/y", // userinfo の入れ子
	} {
		u, err := url.Parse(raw)
		if err != nil {
			continue // 読めない URL はそもそも要求できない
		}
		if err := CheckURL(u); err == nil {
			t.Errorf("CheckURL(%q) = nil, want ErrDisallowedURL", raw)
		}
	}
	if CheckURL(nil) == nil {
		t.Error("nil は不許可")
	}
}

func TestAllowedHostsIsExactlyGitHub(t *testing.T) {
	want := []string{"api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"}
	got := AllowedHosts()
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("AllowedHosts = %v, want %v", got, want)
	}
	got[0] = "evil.example" // コピーを書き換えても許可リストは変わらない
	if CheckURL(mustURL(t, "https://evil.example/")) == nil {
		t.Fatal("AllowedHosts のコピーの書き換えが許可リストに届いた")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
