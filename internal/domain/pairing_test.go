package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const (
	fakeToken = "test-token-not-secret"
	goodURL   = "http%3A%2F%2F100.64.0.5%3A48120%2Freport"
)

func pairing(query string) string { return "swarm-viewer://pair?" + query }

func TestParsePairingOK(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want PairingPayload
	}{
		{
			"最小", pairing("v=1&url=" + goodURL + "&token=" + fakeToken),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5"},
		},
		{
			"全パラメータ", pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&name=devbox&sessions=0%2C1"),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "devbox", SessionsHint: []string{"0", "1"}},
		},
		{
			"順序不問", pairing("token=" + fakeToken + "&name=devbox&url=" + goodURL + "&v=1"),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "devbox"},
		},
		{
			"前後の空白除去", " \n" + pairing("v=1&url="+goodURL+"&token="+fakeToken) + "\t ",
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5"},
		},
		{
			"scheme・host 大文字", "SWARM-VIEWER://PAIR?v=1&url=" + goodURL + "&token=" + fakeToken,
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5"},
		},
		{
			"path スラッシュ", "swarm-viewer://pair/?v=1&url=" + goodURL + "&token=" + fakeToken,
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5"},
		},
		{
			"未知パラメータは無視", pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&future=x"),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5"},
		},
		{
			"日本語の表示名", pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&name=" + url.QueryEscape("開発機 1")),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "開発機 1"},
		},
		{
			"v6 の host を name に", pairing("v=1&url=" + url.QueryEscape("http://[fd7a:115c:a1e0::1]:48120/report") + "&token=" + fakeToken),
			PairingPayload{Version: 1, URL: "http://[fd7a:115c:a1e0::1]:48120/report", Token: fakeToken, Name: "fd7a:115c:a1e0::1"},
		},
		{
			"sessions は空要素を捨てる", pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&sessions=" + url.QueryEscape(" 0, ,1,,")),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: fakeToken, Name: "100.64.0.5", SessionsHint: []string{"0", "1"}},
		},
		{
			"token の %2B は + のまま", pairing("v=1&url=" + goodURL + "&token=a%2Bb"),
			PairingPayload{Version: 1, URL: "http://100.64.0.5:48120/report", Token: "a+b", Name: "100.64.0.5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePairing(tt.in)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestParsePairingErrors(t *testing.T) {
	t.Parallel()
	base := "v=1&url=" + goodURL + "&token=" + fakeToken
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"空", "", ErrPairingFormat},
		{"空白のみ", "  ", ErrPairingFormat},
		{"scheme 違い", "https://pair?" + base, ErrPairingFormat},
		{"host 違い", "swarm-viewer://other?" + base, ErrPairingFormat},
		{"host にポート", "swarm-viewer://pair:80?" + base, ErrPairingFormat},
		{"path 付き", "swarm-viewer://pair/x?" + base, ErrPairingFormat},
		{"userinfo 付き", "swarm-viewer://u:p@pair?" + base, ErrPairingFormat},
		{"フラグメント", pairing(base) + "#x", ErrPairingFormat},
		{"クエリなし", "swarm-viewer://pair", ErrPairingFormat},
		{"v 欠落", pairing("url=" + goodURL + "&token=" + fakeToken), ErrPairingFormat},
		{"v=2", pairing("v=2&url=" + goodURL + "&token=" + fakeToken), ErrPairingVersion},
		{"v=0", pairing("v=0&url=" + goodURL + "&token=" + fakeToken), ErrPairingVersion},
		{"v 空", pairing("v=&url=" + goodURL + "&token=" + fakeToken), ErrPairingVersion},
		{"v=01", pairing("v=01&url=" + goodURL + "&token=" + fakeToken), ErrPairingVersion},
		{"v 重複", pairing("v=1&v=1&url=" + goodURL + "&token=" + fakeToken), ErrPairingFormat},
		{"url 重複", pairing(base + "&url=" + goodURL), ErrPairingFormat},
		{"token 重複", pairing(base + "&token=other"), ErrPairingFormat},
		{"name 重複", pairing(base + "&name=a&name=b"), ErrPairingFormat},
		{"sessions 重複", pairing(base + "&sessions=0&sessions=1"), ErrPairingFormat},
		{"未知パラメータ重複", pairing(base + "&x=1&x=2"), ErrPairingFormat},
		{"url 欠落", pairing("v=1&token=" + fakeToken), ErrInvalidURL},
		{"url 範囲外", pairing("v=1&token=" + fakeToken + "&url=" + url.QueryEscape("http://192.168.0.1:48120/report")), ErrInvalidURL},
		{"url ホスト名", pairing("v=1&token=" + fakeToken + "&url=" + url.QueryEscape("http://devbox.ts.net:48120/report")), ErrInvalidURL},
		{"url https", pairing("v=1&token=" + fakeToken + "&url=" + url.QueryEscape("https://100.64.0.5:48120/report")), ErrInvalidURL},
		{"token 欠落", pairing("v=1&url=" + goodURL), ErrPairingToken},
		{"token 空", pairing("v=1&url=" + goodURL + "&token="), ErrPairingToken},
		{"token に空白", pairing("v=1&url=" + goodURL + "&token=a%20b"), ErrPairingToken},
		{"token に + (空白に復号)", pairing("v=1&url=" + goodURL + "&token=a+b"), ErrPairingToken},
		{"token に制御文字", pairing("v=1&url=" + goodURL + "&token=a%00b"), ErrPairingToken},
		{"token に改行", pairing("v=1&url=" + goodURL + "&token=a%0Ab"), ErrPairingToken},
		{"name 空", pairing(base + "&name="), ErrPairingFormat},
		{"name 空白のみ", pairing(base + "&name=%20%20"), ErrPairingFormat},
		{"name 101 文字", pairing(base + "&name=" + strings.Repeat("a", 101)), ErrPairingFormat},
		{"name 制御文字", pairing(base + "&name=a%07b"), ErrPairingFormat},
		{"不正なパーセントエンコード", pairing(base + "&name=%zz"), ErrPairingFormat},
		{"セミコロン区切り", pairing("v=1;url=" + goodURL), ErrPairingFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePairing(tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v; want %v", err, tt.want)
			}
			if got.Token != "" {
				t.Fatal("失敗時に payload を返さない")
			}
			if strings.Contains(err.Error(), fakeToken) {
				t.Fatalf("エラーに token が含まれる: %v", err)
			}
		})
	}
}

func TestParsePairingBoundaries(t *testing.T) {
	t.Parallel()
	prefix := pairing("v=1&url=" + goodURL + "&token=")
	// 4,096 バイトちょうどは通り、4,097 は拒否する。
	ok := prefix + strings.Repeat("a", MaxPairingBytes-len(prefix))
	if len(ok) != MaxPairingBytes {
		t.Fatalf("テストデータの長さ = %d", len(ok))
	}
	if _, err := ParsePairing(ok); err != nil {
		t.Fatalf("4096 バイト: %v", err)
	}
	if _, err := ParsePairing(ok + "a"); !errors.Is(err, ErrPairingFormat) {
		t.Fatalf("4097 バイト: %v", err)
	}
	// 空白除去後のサイズで判定する。
	if _, err := ParsePairing("  " + ok + "  "); err != nil {
		t.Fatalf("空白付き 4096 バイト: %v", err)
	}
	// name は 100 文字ちょうどまで（文字数で数える）。
	name100 := strings.Repeat("あ", MaxNameRunes)
	p, err := ParsePairing(pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&name=" + url.QueryEscape(name100)))
	if err != nil || p.Name != name100 {
		t.Fatalf("name 100 文字: %v", err)
	}
}

func TestPairingPayloadRedaction(t *testing.T) {
	t.Parallel()
	p, err := ParsePairing(pairing("v=1&url=" + goodURL + "&token=" + fakeToken + "&name=devbox"))
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"String":  p.String(),
		"%v":      fmt.Sprintf("%v", p),
		"%+v":     fmt.Sprintf("%+v", p),
		"%s":      fmt.Sprint(p),
		"%#v":     fmt.Sprintf("%#v", p),
		"%v ptr":  fmt.Sprintf("%v", &p),
		"%+v 入れ子": fmt.Sprintf("%+v", struct{ P PairingPayload }{p}),
	}
	j, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	outputs["json"] = string(j)
	jp, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}
	outputs["json ptr"] = string(jp)

	var sb strings.Builder
	h := slog.NewJSONHandler(&sb, &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}})
	slog.New(h).Info("pairing", "payload", p)
	outputs["slog"] = sb.String()
	var tb strings.Builder
	slog.New(slog.NewTextHandler(&tb, nil)).Info("pairing", "payload", p)
	outputs["slog text"] = tb.String()

	for name, out := range outputs {
		if strings.Contains(out, fakeToken) {
			t.Errorf("%s に token が含まれる: %s", name, out)
		}
		if strings.Contains(out, "/report") {
			t.Errorf("%s に完全な URL が含まれる: %s", name, out)
		}
		if !strings.Contains(out, "100.64.0.5") {
			t.Errorf("%s に host が無い: %s", name, out)
		}
	}
	if p.Token != fakeToken {
		t.Fatal("伏せ字化で元の値を壊さない")
	}
}
