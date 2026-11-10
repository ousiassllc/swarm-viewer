package release

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

// 本番のクライアントが通信できるのは GitHub のホストへの https だけ。それ以外（tailnet・プライベート・ローカル・http）は、
// 実際に待ち受けているサーバーがあっても通信しない（接続先をコードに固定していること）。
func TestProductionClientOnlyTalksToGitHubOverHTTPS(t *testing.T) {
	rec := &recorder{}
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rec.record(r) }))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rec.record(r) }))
	defer tls.Close()

	c := NewClient("swarm-viewer/test")
	for _, raw := range []string{
		plain.URL + "/x",                // http のローカル
		tls.URL + "/x",                  // https のローカル（127.0.0.1）
		"http://api.github.com/x",       // https でない GitHub
		"https://100.64.52.167:48120/x", // tailnet の console
		"https://evil.example/x",
	} {
		err := c.Download(t.Context(), update.Asset{Name: "x", URL: raw}, discard{})
		if err == nil {
			t.Errorf("Download(%q) が成功した", raw)
		}
	}
	if rec.hits != 0 {
		t.Fatalf("許可外のサーバーへ %d 回通信した", rec.hits)
	}
}

// 本番の設定：最新リリースの取得先は api.github.com の https に固定されている。
func TestProductionLatestURLIsGitHubAPI(t *testing.T) {
	c := NewClient("swarm-viewer/test")
	if c.latestURL != "https://api.github.com/repos/ousiassllc/swarm-viewer/releases/latest" {
		t.Fatalf("latestURL = %q", c.latestURL)
	}
	u, err := url.Parse(c.latestURL)
	if err != nil || update.CheckURL(u) != nil {
		t.Fatalf("取得先が許可リストを通らない: %v", err)
	}
	hosts := update.AllowedHosts()
	slices.Sort(hosts)
	want := []string{"api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"}
	if !slices.Equal(hosts, want) {
		t.Fatalf("許可ホスト = %v, want %v", hosts, want)
	}
}

// 認証情報はリダイレクトのとき外す（CheckRedirect）。
func TestCheckRedirectStripsCredentials(t *testing.T) {
	c := NewClient("swarm-viewer/test")
	req, _ := http.NewRequest(http.MethodGet, "https://objects.githubusercontent.com/x", http.NoBody)
	req.Header.Set("Authorization", "Bearer console-token")
	req.Header.Set("Cookie", "a=b")
	req.Header.Set("Proxy-Authorization", "Basic x")
	if err := c.hc.CheckRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
		if req.Header.Get(k) != "" {
			t.Errorf("%s が残った", k)
		}
	}
	bad, _ := http.NewRequest(http.MethodGet, "https://100.64.52.167:48120/ops", http.NoBody)
	if err := c.hc.CheckRedirect(bad, nil); err == nil {
		t.Error("tailnet へのリダイレクトを許した")
	}
	if c.hc.Jar != nil {
		t.Error("Cookie Jar を持っている")
	}
}

// 送信の直前にも、資格情報のヘッダを外し、元のリクエストは書き換えない。
func TestGuardStripsCredentialsBeforeSending(t *testing.T) {
	var got http.Header
	base := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		got = r.Header.Clone()
		return &http.Response{StatusCode: 200, Body: http.NoBody, Request: r}, nil
	})
	g := &guard{base: base, allowed: update.CheckURL}
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/x", http.NoBody)
	req.Header.Set("Authorization", "Bearer console-token")
	req.Header.Set("Cookie", "a=b")
	req.Header.Set("X-Keep", "1")
	resp, err := g.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got.Get("Authorization") != "" || got.Get("Cookie") != "" || got.Get("X-Keep") != "1" {
		t.Fatalf("送信されたヘッダ = %v", got)
	}
	if req.Header.Get("Authorization") == "" {
		t.Fatal("RoundTripper が受け取ったリクエストを書き換えた")
	}
	blockedReq, _ := http.NewRequest(http.MethodGet, "https://100.64.52.167:48120/report", http.NoBody)
	got = nil
	if _, err := g.RoundTrip(blockedReq); err == nil || got != nil {
		t.Fatalf("tailnet へ送った: err=%v", err)
	}
}

// 名前解決の結果が tailnet・プライベート・ローカルのアドレスなら接続しない。
func TestDenyPrivateAddresses(t *testing.T) {
	for addr, private := range map[string]bool{
		"100.64.52.167:443":       true, // tailnet
		"100.127.255.255:443":     true,
		"127.0.0.1:443":           true,
		"10.0.0.1:443":            true,
		"172.16.0.1:443":          true,
		"192.168.1.1:443":         true,
		"169.254.169.254:443":     true,
		"0.0.0.0:443":             true,
		"[::1]:443":               true,
		"[fd00::1]:443":           true,
		"[fe80::1]:443":           true,
		"[::ffff:100.64.0.1]:443": true,  // IPv4 射影
		"140.82.112.5:443":        false, // github.com の範囲
		"185.199.108.133:443":     false,
		"100.63.255.255:443":      false, // tailnet の直前
		"100.128.0.0:443":         false, // tailnet の直後
		"[2606:50c0:8000::1]:443": false,
	} {
		err := denyPrivate("tcp", addr, nil)
		if (err != nil) != private {
			t.Errorf("denyPrivate(%s) = %v, want private=%v", addr, err, private)
		}
	}
	if denyPrivate("tcp", "not-an-address", nil) == nil {
		t.Error("不正なアドレスを通した")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
