package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ReportPort は swarm-console の `/report` の待ち受けポート。tailnet の IP ではこのポートに限る。
const ReportPort = "48120"

// ReportPath は接続先 URL のパス。
const ReportPath = "/report"

// ErrInvalidURL は接続先 URL が許可された範囲外・形式不正のとき（NFR-SEC-4・5）。
var ErrInvalidURL = errors.New("domain: connection url is not allowed")

var (
	tailnetV4 = netip.MustParsePrefix("100.64.0.0/10")
	tailnetV6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

// Connection は保存済みの接続。token を持たない（token は秘密領域のみ）。
// LastSession・BaselinedAt は未設定（DB の NULL）のとき零値（空文字・IsZero）。
type Connection struct {
	ID          string
	Name        string
	URL         string
	LastSession string
	BaselinedAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Baselined は初回の取得パスが完了済み（通知を出してよい段階）かを返す。
func (c Connection) Baselined() bool { return !c.BaselinedAt.IsZero() }

// Host は URL の host（ログ・表示用。NFR-SEC-11）。URL が解釈できなければ空文字。
func (c Connection) Host() string { return HostOf(c.URL) }

// HostOf は URL から host のみを取り出す。解釈できなければ空文字。
func HostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Session は `sessions_list` の 1 件（viewer が使う項目のみ）。
type Session struct {
	ID      string `json:"id"`
	Allowed bool   `json:"allowed"`
}

// ParseSessions は `sessions_list` 応答を許可されたセッションだけに絞る。順序は応答のまま。
// ID が空・重複のものは捨てる。0 件のときは空（nil でない）スライス。
// JSON が壊れている・`sessions` が配列でないときは ErrInvalidResponse。
func ParseSessions(data []byte) ([]Session, error) {
	var w struct {
		Sessions json.RawMessage `json:"sessions"`
	}
	if !isJSONObject(data) || json.Unmarshal(data, &w) != nil ||
		len(w.Sessions) == 0 || bytes.Equal(w.Sessions, []byte("null")) {
		return nil, ErrInvalidResponse
	}
	var all []Session
	if json.Unmarshal(w.Sessions, &all) != nil {
		return nil, ErrInvalidResponse
	}
	out := make([]Session, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	for _, s := range all {
		if !s.Allowed || s.ID == "" {
			continue
		}
		if _, dup := seen[s.ID]; dup {
			continue
		}
		seen[s.ID] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}

// ValidateURL は接続先 URL を検証し、正規化した URL を返す（NFR-SEC-4・5）。
//
// 許可するのは次の形だけ。違えば ErrInvalidURL（理由は区別しない）。
//   - scheme は http（大文字小文字は問わず小文字に正規化）
//   - host は tailnet（100.64.0.0/10、fd7a:115c:a1e0::/48）またはループバック（127.0.0.0/8、::1）の IP リテラル。
//     ホスト名・0.0.0.0・IPv6 ゾーン・IPv4 射影アドレス・省略形の IPv4 は不可
//   - port は明示が必須。tailnet は 48120 のみ、ループバックは 1〜65535（先頭ゼロ不可）
//   - path は /report ちょうど（末尾スラッシュ・パーセントエンコード別表記は不可）
//   - userinfo・クエリ（空の `?` を含む）・フラグメントは不可
//
// 前後の空白は除去してから解析する。
func ValidateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "#") {
		return "", ErrInvalidURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidURL
	}
	if !strings.EqualFold(u.Scheme, "http") || u.Opaque != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", ErrInvalidURL
	}
	if u.Path != ReportPath || u.RawPath != "" {
		return "", ErrInvalidURL
	}
	addr, err := netip.ParseAddr(u.Hostname())
	if err != nil || addr.Zone() != "" || addr.Is4In6() {
		return "", ErrInvalidURL
	}
	port, err := normalizePort(u.Port(), addr)
	if err != nil {
		return "", err
	}
	return "http://" + net.JoinHostPort(addr.String(), port) + ReportPath, nil
}

// normalizePort は host の種別に応じて port を検証する。addr は tailnet かループバックでなければ拒否する。
func normalizePort(raw string, addr netip.Addr) (string, error) {
	switch {
	case tailnetV4.Contains(addr) || tailnetV6.Contains(addr):
		if raw != ReportPort {
			return "", ErrInvalidURL
		}
		return raw, nil
	case addr.IsLoopback():
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != raw {
			return "", ErrInvalidURL
		}
		return raw, nil
	default:
		return "", ErrInvalidURL
	}
}
