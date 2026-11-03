package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string // 空なら ErrInvalidURL
	}{
		// tailnet IPv4 の境界（100.64.0.0/10 = 100.64.0.0〜100.127.255.255）
		{"v4 下端", "http://100.64.0.0:48120/report", "http://100.64.0.0:48120/report"},
		{"v4 下端の 1 つ下", "http://100.63.255.255:48120/report", ""},
		{"v4 上端", "http://100.127.255.255:48120/report", "http://100.127.255.255:48120/report"},
		{"v4 上端の 1 つ上", "http://100.128.0.0:48120/report", ""},
		{"v4 通常", "http://100.64.0.5:48120/report", "http://100.64.0.5:48120/report"},
		// tailnet IPv6 の境界（fd7a:115c:a1e0::/48）
		{"v6 下端", "http://[fd7a:115c:a1e0::]:48120/report", "http://[fd7a:115c:a1e0::]:48120/report"},
		{"v6 上端", "http://[fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff]:48120/report", "http://[fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff]:48120/report"},
		{"v6 上位 48bit の下", "http://[fd7a:115c:a1df:ffff::1]:48120/report", ""},
		{"v6 上位 48bit の上", "http://[fd7a:115c:a1e1::1]:48120/report", ""},
		{"v6 大文字は小文字へ正規化", "http://[FD7A:115C:A1E0::1]:48120/report", "http://[fd7a:115c:a1e0::1]:48120/report"},
		// ループバック（任意のポート）
		{"127.0.0.1 任意ポート", "http://127.0.0.1:8080/report", "http://127.0.0.1:8080/report"},
		{"127.0.0.1 48120", "http://127.0.0.1:48120/report", "http://127.0.0.1:48120/report"},
		{"127/8 の別アドレス", "http://127.1.2.3:9/report", "http://127.1.2.3:9/report"},
		{"::1", "http://[::1]:8080/report", "http://[::1]:8080/report"},
		{"ループバックの port 0", "http://127.0.0.1:0/report", ""},
		{"ループバックの port 65535", "http://127.0.0.1:65535/report", "http://127.0.0.1:65535/report"},
		{"ループバックの port 65536", "http://127.0.0.1:65536/report", ""},
		{"ループバックの port 先頭ゼロ", "http://127.0.0.1:08080/report", ""},
		{"ループバックの port 省略", "http://127.0.0.1/report", ""},
		// 範囲外・ホスト名
		{"localhost", "http://localhost:48120/report", ""},
		{"MagicDNS 名", "http://devbox.tail1234.ts.net:48120/report", ""},
		{"0.0.0.0", "http://0.0.0.0:48120/report", ""},
		{"::", "http://[::]:48120/report", ""},
		{"プライベート", "http://192.168.1.10:48120/report", ""},
		{"グローバル", "http://8.8.8.8:48120/report", ""},
		{"IPv4 射影 v6", "http://[::ffff:100.64.0.5]:48120/report", ""},
		{"IPv6 ゾーン", "http://[fe80::1%25eth0]:48120/report", ""},
		{"10 進 IP", "http://1684275205:48120/report", ""},
		{"空 host", "http://:48120/report", ""},
		// scheme
		{"https", "https://100.64.0.5:48120/report", ""},
		{"ftp", "ftp://100.64.0.5:48120/report", ""},
		{"scheme なし", "100.64.0.5:48120/report", ""},
		{"scheme 大文字は正規化", "HTTP://100.64.0.5:48120/report", "http://100.64.0.5:48120/report"},
		// port
		{"tailnet 別ポート", "http://100.64.0.5:8080/report", ""},
		{"tailnet ポート省略", "http://100.64.0.5/report", ""},
		{"tailnet 先頭ゼロ", "http://100.64.0.5:048120/report", ""},
		{"tailnet 空ポート", "http://100.64.0.5:/report", ""},
		// path
		{"path なし", "http://100.64.0.5:48120", ""},
		{"/ops", "http://100.64.0.5:48120/ops", ""},
		{"末尾スラッシュ", "http://100.64.0.5:48120/report/", ""},
		{"大文字", "http://100.64.0.5:48120/Report", ""},
		{"エンコード別表記", "http://100.64.0.5:48120/%72eport", ""},
		{"パス追加", "http://100.64.0.5:48120/report/x", ""},
		// userinfo・クエリ・フラグメント
		{"userinfo", "http://user:pw@100.64.0.5:48120/report", ""},
		{"userinfo 名のみ", "http://user@100.64.0.5:48120/report", ""},
		{"userinfo で host 偽装", "http://100.64.0.5:48120@evil.example/report", ""},
		{"クエリ", "http://100.64.0.5:48120/report?a=1", ""},
		{"空クエリ", "http://100.64.0.5:48120/report?", ""},
		{"フラグメント", "http://100.64.0.5:48120/report#x", ""},
		{"空フラグメント", "http://100.64.0.5:48120/report#", ""},
		// その他
		{"空", "", ""},
		{"空白のみ", "   ", ""},
		{"前後の空白は除去", "  http://100.64.0.5:48120/report \n", "http://100.64.0.5:48120/report"},
		{"途中の空白", "http://100.64.0.5: 48120/report", ""},
		{"制御文字", "http://100.64.0.5:48120/report\x00", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateURL(tt.in)
			if tt.want == "" {
				if !errors.Is(err, ErrInvalidURL) || got != "" {
					t.Fatalf("ValidateURL(%q) = %q, %v; want ErrInvalidURL", tt.in, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ValidateURL(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
			if again, err := ValidateURL(got); err != nil || again != got {
				t.Fatalf("正規化結果が冪等でない: %q -> %q, %v", got, again, err)
			}
		})
	}
}

func TestParseSessions(t *testing.T) {
	t.Parallel()
	const body = `{"sessions":[
		{"id":"0","allowed":true,"operable":false,"layout":{"rows":2,"cols":[3,3]},"panes":["T1"]},
		{"id":"secret","allowed":false},
		{"id":"1","allowed":true},
		{"id":"0","allowed":true},
		{"id":"","allowed":true},
		{"id":"2"}
	],"extra":1}`
	got, err := ParseSessions([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	want := []Session{{ID: "0", Allowed: true}, {ID: "1", Allowed: true}}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestParseSessionsEdges(t *testing.T) {
	t.Parallel()
	got, err := ParseSessions([]byte(`{"sessions":[]}`))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("空配列: got %#v, %v", got, err)
	}
	got, err = ParseSessions([]byte(`{"sessions":[{"id":"x","allowed":false}]}`))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("allowed なし: got %#v, %v", got, err)
	}
	for _, in := range []string{``, `null`, `[]`, `{}`, `{"sessions":null}`, `{"sessions":{}}`, `{"sessions":[1]}`, `not json`, `{"sessions":[`} {
		if _, err := ParseSessions([]byte(in)); !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("ParseSessions(%q) err = %v; want ErrInvalidResponse", in, err)
		}
	}
}

func TestConnectionHelpers(t *testing.T) {
	t.Parallel()
	c := Connection{URL: "http://100.64.0.5:48120/report"}
	if c.Host() != "100.64.0.5" {
		t.Errorf("Host = %q", c.Host())
	}
	if got := (Connection{URL: "http://[fd7a:115c:a1e0::1]:48120/report"}).Host(); got != "fd7a:115c:a1e0::1" {
		t.Errorf("v6 Host = %q", got)
	}
	if (Connection{URL: "://"}).Host() != "" {
		t.Error("不正 URL の Host は空")
	}
	if c.Baselined() {
		t.Error("零値は未 baseline")
	}
	c.BaselinedAt = time.Date(2026, 11, 3, 9, 10, 5, 0, time.UTC)
	if !c.Baselined() {
		t.Error("BaselinedAt があれば baseline 済み")
	}
}
