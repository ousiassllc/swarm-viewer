//go:build devupdate

package release

import (
	"errors"
	"net/http"
	"net/url"
)

// NewDevClient はローカルの偽の Release サーバー（実機・画面の確認用）へ向けたクライアント。devupdate タグを付けたビルドにだけ存在し、
// 本番のビルド（タグなし）には含まれない。許可するのは latestURL と同じホスト（ポート込み）への http / https だけ。
func NewDevClient(latestURL string, base http.RoundTripper) (*Client, error) {
	root, err := url.Parse(latestURL)
	if err != nil || root.Host == "" || (root.Scheme != "http" && root.Scheme != "https") {
		return nil, errors.New("release: dev client needs an http(s) url")
	}
	allowed := func(u *url.URL) error {
		if u == nil || u.Host != root.Host || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("release: dev client only talks to " + root.Host)
		}
		return nil
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return newClient(latestURL, allowed, base, "swarm-viewer/dev"), nil
}
