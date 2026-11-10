// Package release は GitHub Releases からの新しい版の取得（最新リリースの確認とアセットのダウンロード）。Issue #109。
//
// swarm-console への接続（repository/report。tailnet の許可リスト・Bearer 付与）とは完全に分けた独自の http.Client を使う。
// 通信先は api.github.com と Releases の配信元（domain/update.AllowedHosts）への https だけで、リダイレクト先にも同じ検査をする。
// Authorization・Cookie・Proxy-Authorization は、どの経路でも付けない・付いていても送る前に外す（console の token を GitHub へ渡さない）。
// 本番のクライアントは接続先をコードに固定する（NewClient）。別の接続先へ向けられるのはテスト（newClient）と、devupdate タグのビルドだけ。
package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

const (
	// latestTimeout は最新リリースの確認 1 回の上限。
	latestTimeout = 30 * time.Second
	// maxLatestBytes は releases/latest の JSON の読み取り上限。
	maxLatestBytes = 4 << 20
	// MaxAssetBytes はアセット 1 つの取得上限（インストーラ・zip）。超えたら失敗にする。
	MaxAssetBytes = 512 << 20
	// maxRedirects はリダイレクトの追従回数の上限。
	maxRedirects = 5
)

// 取得の失敗。呼び出し側は errors.Is で種類を分ける（原因の詳細・URL は画面に出さない）。
var (
	// ErrNoRelease は最新リリースが無い（404）。エラー扱いにせず「新しい版なし」にする。
	ErrNoRelease = errors.New("release: no release")
	// ErrStatus は 200 以外の応答。
	ErrStatus = errors.New("release: unexpected http status")
	// ErrTruncated は本文が Content-Length・報告されたサイズより短い（途中で切れた）。
	ErrTruncated = errors.New("release: download is truncated")
	// ErrTooLarge は本文が上限を超えた。
	ErrTooLarge = errors.New("release: response is too large")
)

// Client は GitHub Releases への取得。
type Client struct {
	hc        *http.Client
	latestURL string
	allowed   func(*url.URL) error
	userAgent string
}

// NewClient は本番のクライアント。接続先は update.LatestReleaseURL・update.AllowedHosts に固定し、
// プライベート・ループバック・tailnet（100.64.0.0/10）のアドレスには接続しない。userAgent は "swarm-viewer/<版>" の形。
func NewClient(userAgent string) *Client {
	return newClient(update.LatestReleaseURL, update.CheckURL, newTransport(), userAgent)
}

// newClient は接続先・許可の判定・トランスポートを指定して作る。本番は NewClient だけを使う（テストと devupdate が使う）。
func newClient(latestURL string, allowed func(*url.URL) error, base http.RoundTripper, userAgent string) *Client {
	c := &Client{latestURL: latestURL, allowed: allowed, userAgent: userAgent}
	c.hc = &http.Client{
		// 全リクエスト（リダイレクト先を含む）を許可リストで検査し、資格情報を外す。Jar は持たない（Cookie を保存しない）。
		Transport: &guard{base: base, allowed: allowed},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("release: too many redirects")
			}
			stripCredentials(req.Header)
			return allowed(req.URL)
		},
	}
	return c
}

// Latest は最新リリースを取得する。draft・prerelease は update.ErrIgnoredRelease、JSON・tag_name の不正は update.ErrBadRelease、
// リリースが無ければ ErrNoRelease。
func (c *Client) Latest(ctx context.Context) (update.Release, error) {
	ctx, cancel := context.WithTimeout(ctx, latestTimeout)
	defer cancel()
	resp, err := c.get(ctx, c.latestURL, "application/vnd.github+json")
	if err != nil {
		return update.Release{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return update.Release{}, ErrNoRelease
	case resp.StatusCode != http.StatusOK:
		return update.Release{}, fmt.Errorf("%w: %d", ErrStatus, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLatestBytes+1))
	if err != nil {
		return update.Release{}, err
	}
	if len(body) > maxLatestBytes {
		return update.Release{}, ErrTooLarge
	}
	return update.ParseRelease(body)
}

// Download はアセットを取得して dst へ書く。取得の前に URL が許可するホストの https か検査する。
// 本文が Content-Length・報告されたサイズ（asset.Size > 0 のとき）より短ければ ErrTruncated、上限（MaxAssetBytes）を超えれば ErrTooLarge。
// 失敗したとき dst には途中までの内容が書かれている（呼び出し側が捨てる）。
func (c *Client) Download(ctx context.Context, asset update.Asset, dst io.Writer) error {
	resp, err := c.get(ctx, asset.URL, "application/octet-stream")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrStatus, resp.StatusCode)
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, MaxAssetBytes+1))
	switch {
	case err != nil:
		return fmt.Errorf("%w: %w", ErrTruncated, err)
	case n > MaxAssetBytes:
		return ErrTooLarge
	case resp.ContentLength >= 0 && n != resp.ContentLength, asset.Size > 0 && n != asset.Size:
		return ErrTruncated
	}
	return nil
}

// get は rawURL への GET。許可リストの検査は guard が全リクエストに対して行うが、ここでも先に落として、不許可の URL に通信しないことを明確にする。
func (c *Client) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, update.ErrDisallowedURL
	}
	if err := c.allowed(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.userAgent)
	return c.hc.Do(req)
}
