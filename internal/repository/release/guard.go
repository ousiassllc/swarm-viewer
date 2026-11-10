package release

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// credentialHeaders は GitHub へ送らないヘッダ。console の token（Authorization: Bearer）と Cookie を含む。
var credentialHeaders = []string{"Authorization", "Proxy-Authorization", "Cookie"}

func stripCredentials(h http.Header) {
	for _, k := range credentialHeaders {
		h.Del(k)
	}
}

// guard は送信の直前に、URL を許可リストで検査し、資格情報のヘッダを外す RoundTripper。
// リダイレクトの追従も新しいリクエストとしてここを通るので、許可外のホストや http への転送は通信せずに失敗する。
type guard struct {
	base    http.RoundTripper
	allowed func(*url.URL) error
}

func (g *guard) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := g.allowed(req.URL); err != nil {
		return nil, err
	}
	// RoundTripper は受け取った req を書き換えてはならない。
	out := req.Clone(req.Context())
	stripCredentials(out.Header)
	return g.base.RoundTrip(out)
}

// errPrivateAddress は接続先のアドレスがプライベート・ループバック・tailnet などのとき。
var errPrivateAddress = errors.New("release: connecting to a private or local address is not allowed")

// cgnat は tailnet（Tailscale）が使う 100.64.0.0/10。
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// denyPrivate は net.Dialer.Control として、名前解決後の実際の接続先アドレスを検査する。
// 許可ホストの名前が tailnet・プライベートのアドレスへ解決されても（DNS の差し替え）接続しない。
func denyPrivate(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("release: bad dial address: %w", err)
	}
	if isPrivate(ap.Addr()) {
		return errPrivateAddress
	}
	return nil
}

func isPrivate(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified() || cgnat.Contains(a)
}

// newTransport は本番のトランスポート。プロキシは使わない（プロキシ経由だと接続先の検査が効かない）。
// 圧縮は無効にして、Content-Length と受け取ったバイト数を比べられるようにする。
func newTransport() *http.Transport {
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second, Control: denyPrivate}
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           d.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:    true,
		MaxIdleConns:          2,
		IdleConnTimeout:       30 * time.Second,
	}
}
