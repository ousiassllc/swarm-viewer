package release

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain/update"
)

// recorder は受けたリクエストのヘッダを覚える偽の GitHub。
type recorder struct {
	mu   sync.Mutex
	seen []http.Header
	hits int
}

func (r *recorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hits++
	r.seen = append(r.seen, req.Header.Clone())
}

func (r *recorder) headers() []http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]http.Header(nil), r.seen...)
}

// testClient は srv（TLS）だけを許可するクライアント。本番の NewClient と同じ構造（guard・CheckRedirect）を使う。
func testClient(t *testing.T, latestURL string, srvs ...*httptest.Server) *Client {
	t.Helper()
	hosts := map[string]bool{}
	for _, s := range srvs {
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}
		hosts[u.Host] = true
	}
	allowed := func(u *url.URL) error {
		if u == nil || u.Scheme != "https" || !hosts[u.Host] {
			return update.ErrDisallowedURL
		}
		return nil
	}
	return newClient(latestURL, allowed, srvs[0].Client().Transport, "swarm-viewer/test")
}

const releaseJSON = `{"tag_name":"v0.1.13","assets":[{"name":"a.zip","browser_download_url":"URL/a.zip","size":5}]}`

func TestLatestParsesReleaseAndSendsNoCredentials(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()
	c := testClient(t, srv.URL+"/latest", srv)

	rel, err := c.Latest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v0.1.13" || len(rel.Assets) != 1 {
		t.Fatalf("release = %+v", rel)
	}
	assertNoCredentials(t, rec)
	if got := rec.headers()[0].Get("User-Agent"); got != "swarm-viewer/test" {
		t.Fatalf("User-Agent = %q", got)
	}
}

// assertNoCredentials は、受けたすべてのリクエストに資格情報のヘッダが無いことを確かめる。
func assertNoCredentials(t *testing.T, rec *recorder) {
	t.Helper()
	if len(rec.headers()) == 0 {
		t.Fatal("リクエストが 1 つも届いていない")
	}
	for _, h := range rec.headers() {
		for _, k := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
			if v := h.Get(k); v != "" {
				t.Errorf("GitHub へ %s ヘッダが送られた", k)
			}
		}
	}
}

func TestLatestFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{"404 はリリースなし", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }, ErrNoRelease},
		{"500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, ErrStatus},
		{"403（レート制限）", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }, ErrStatus},
		{"壊れた JSON", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) }, update.ErrBadRelease},
		{"draft", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","draft":true}`))
		}, update.ErrIgnoredRelease},
		{"prerelease", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","prerelease":true}`))
		}, update.ErrIgnoredRelease},
		{"v なしのタグ", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"tag_name":"9.9.9"}`)) }, update.ErrBadRelease},
	} {
		srv := httptest.NewTLSServer(tc.handler)
		_, err := testClient(t, srv.URL+"/latest", srv).Latest(t.Context())
		srv.Close()
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestLatestOffline(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	c := testClient(t, srv.URL+"/latest", srv)
	srv.Close() // 接続できない
	if _, err := c.Latest(t.Context()); err == nil {
		t.Fatal("オフラインで成功した")
	}
}

func TestDownloadWritesBody(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	c := testClient(t, "", srv)
	var buf bytes.Buffer
	if err := c.Download(t.Context(), update.Asset{Name: "a", URL: srv.URL + "/a", Size: 5}, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Fatalf("body = %q", buf.String())
	}
	assertNoCredentials(t, rec)
}

// 途中で切れたダウンロード（Content-Length より短い）は失敗にする。
func TestDownloadTruncated(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("only a part"))
		// ハンドラが約束より短く終わると、サーバーは接続を切る。
	}))
	defer srv.Close()
	var buf bytes.Buffer
	err := testClient(t, "", srv).Download(t.Context(), update.Asset{URL: srv.URL + "/a"}, &buf)
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

// GitHub の報告したサイズと違う本文（Content-Length は一致していても）は失敗にする。
func TestDownloadSizeMismatchWithReportedSize(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("short")) }))
	defer srv.Close()
	err := testClient(t, "", srv).Download(t.Context(), update.Asset{URL: srv.URL + "/a", Size: 99}, &bytes.Buffer{})
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(502) }))
	defer srv.Close()
	err := testClient(t, "", srv).Download(t.Context(), update.Asset{URL: srv.URL + "/a"}, &bytes.Buffer{})
	if !errors.Is(err, ErrStatus) {
		t.Fatalf("err = %v, want ErrStatus", err)
	}
}

// 許可するホストの間のリダイレクト（github.com → objects.githubusercontent.com の形）は追従する。資格情報は送られない。
func TestDownloadFollowsAllowedRedirect(t *testing.T) {
	rec := &recorder{}
	cdn := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		_, _ = w.Write([]byte("payload"))
	}))
	defer cdn.Close()
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		http.Redirect(w, r, cdn.URL+"/blob", http.StatusFound)
	}))
	defer front.Close()
	var buf bytes.Buffer
	if err := testClient(t, "", front, cdn).Download(t.Context(), update.Asset{URL: front.URL + "/a"}, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "payload" {
		t.Fatalf("body = %q", buf.String())
	}
	assertNoCredentials(t, rec)
}

// 許可外のホストへのリダイレクトは、そのホストへ通信せずに失敗する。
func TestDownloadRefusesRedirectToDisallowedHost(t *testing.T) {
	evil := &recorder{}
	evilSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evil.record(r)
		_, _ = w.Write([]byte("evil"))
	}))
	defer evilSrv.Close()
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evilSrv.URL+"/blob", http.StatusFound)
	}))
	defer front.Close()
	var buf bytes.Buffer
	err := testClient(t, "", front).Download(t.Context(), update.Asset{URL: front.URL + "/a"}, &buf)
	if err == nil {
		t.Fatal("許可外へのリダイレクトに追従した")
	}
	if evil.hits != 0 || buf.Len() != 0 {
		t.Fatalf("許可外のホストへ通信した: hits=%d body=%q", evil.hits, buf.String())
	}
}

// https から http への転送（ダウングレード）は追従しない。
func TestDownloadRefusesRedirectToHTTP(t *testing.T) {
	plain := &recorder{}
	plainSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plain.record(r)
		_, _ = w.Write([]byte("plain"))
	}))
	defer plainSrv.Close()
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plainSrv.URL+"/blob", http.StatusFound)
	}))
	defer front.Close()
	// 許可の判定は host だけ（スキームを見ない）にして、リダイレクト先が許可ホストでも https でなければ追従しない、をスキームの検査（domain/update.CheckURL）で守っていることを確かめる。
	// ここでは本番と同じ update.CheckURL に、http のサーバーのホストだけを足したものを使う。
	pu, _ := url.Parse(plainSrv.URL)
	fu, _ := url.Parse(front.URL)
	c := newClient("", func(x *url.URL) error {
		if x.Host == fu.Host {
			return nil
		}
		if x.Host == pu.Host {
			x2 := *x
			x2.Host = "api.github.com" // 許可ホストの名前にしても、スキームが http なら通らない
			return update.CheckURL(&x2)
		}
		return update.ErrDisallowedURL
	}, front.Client().Transport, "swarm-viewer/test")
	if err := c.Download(t.Context(), update.Asset{URL: front.URL + "/a"}, &bytes.Buffer{}); err == nil {
		t.Fatal("http への転送に追従した")
	}
	if plain.hits != 0 {
		t.Fatalf("http のサーバーへ通信した: %d", plain.hits)
	}
}

// 無限のリダイレクトは打ち切る。
func TestDownloadStopsRedirectLoop(t *testing.T) {
	var srv *httptest.Server
	hits := 0
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, srv.URL+"/again?n="+strconv.Itoa(hits), http.StatusFound)
	}))
	defer srv.Close()
	if err := testClient(t, "", srv).Download(t.Context(), update.Asset{URL: srv.URL + "/a"}, &bytes.Buffer{}); err == nil {
		t.Fatal("リダイレクトのループが止まらない")
	}
	if hits > maxRedirects+1 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestDownloadCancel(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := testClient(t, "", srv).Download(ctx, update.Asset{URL: srv.URL + "/a"}, &bytes.Buffer{}); err == nil {
		t.Fatal("キャンセル済みで成功した")
	}
}
