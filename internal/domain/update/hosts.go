package update

import (
	"errors"
	"net/url"
	"slices"
	"strings"
)

// 本番の接続先（コードに固定する。設定・環境変数では変えられない）。
const (
	// LatestReleaseURL は最新リリースの取得先。
	LatestReleaseURL = "https://api.github.com/repos/ousiassllc/swarm-viewer/releases/latest"
)

// AllowedHosts は GitHub への通信で許可するホスト（API と Releases の配信元）。リダイレクト先にも同じ検査をする。
var allowedHosts = []string{
	"api.github.com",
	"github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
}

// AllowedHosts は許可するホストのコピー。
func AllowedHosts() []string { return slices.Clone(allowedHosts) }

// ErrDisallowedURL は許可されない URL（https でない・許可外のホスト・ポート指定・認証情報つき）。
var ErrDisallowedURL = errors.New("update: url is not allowed")

// CheckURL は u が許可するホストへの https だけかを検査する。ホストは完全一致（サブドメイン・接尾辞は不可）で、
// ポート指定・userinfo は不可。tailnet・プライベート・ローカルのホストはどれも許可リストに無いので通らない。
func CheckURL(u *url.URL) error {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ErrDisallowedURL
	}
	if !slices.Contains(allowedHosts, strings.ToLower(u.Hostname())) || u.Opaque != "" {
		return ErrDisallowedURL
	}
	return nil
}
