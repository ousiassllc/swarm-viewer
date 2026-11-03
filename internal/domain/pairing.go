package domain

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ペアリング接続文字列の制約（API 仕様 §2.1）。
const (
	// PairingVersion は対応するペイロード版。
	PairingVersion = 1
	// MaxPairingBytes は接続文字列全体（前後の空白除去後）のサイズ上限（バイト）。
	MaxPairingBytes = 4096
	// MaxNameRunes は表示名の最大文字数。
	MaxNameRunes = 100

	pairingScheme = "swarm-viewer"
	pairingHost   = "pair"
	redacted      = "***"
)

// ペアリング失敗の種別。メッセージは固定文言で、入力値（token を含みうる）を含めない。
var (
	// ErrPairingFormat は scheme・パラメータ重複・サイズ超過・必須欠落・name 不正など形式の誤り。
	ErrPairingFormat = errors.New("domain: invalid pairing string format")
	// ErrPairingVersion は `v` が対応版（1）でない。「アプリを更新してください」。
	ErrPairingVersion = errors.New("domain: unsupported pairing version")
	// ErrPairingToken は token が空・空白や制御文字を含む。
	ErrPairingToken = errors.New("domain: invalid pairing token")
)

// PairingPayload は解析・検証済みの接続文字列。Token を持つ唯一の domain 型。
// String・GoString・MarshalJSON・LogValue は Token を伏せ、URL も host のみにする。
type PairingPayload struct {
	Version      int
	URL          string // ValidateURL で正規化済み
	Token        string
	Name         string   // 未指定なら URL の host
	SessionsHint []string // 表示のヒントのみ。権限の正は sessions_list
}

// String は token を伏せた表示。
func (p PairingPayload) String() string {
	return "PairingPayload{version=" + strconv.Itoa(p.Version) + " host=" + HostOf(p.URL) + " name=" + p.Name + " token=" + redacted + "}"
}

// GoString は %#v 用。token を伏せる。
func (p PairingPayload) GoString() string { return p.String() }

// MarshalJSON は token を伏せた JSON を返す（ログ・デバッグ出力に token を出さない）。
func (p PairingPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Version      int      `json:"version"`
		Host         string   `json:"host"`
		Token        string   `json:"token"`
		Name         string   `json:"name"`
		SessionsHint []string `json:"sessions_hint,omitempty"`
	}{p.Version, HostOf(p.URL), redacted, p.Name, p.SessionsHint})
}

// LogValue は slog 用。token を伏せ、URL は host のみを出す（NFR-SEC-11）。
func (p PairingPayload) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("version", p.Version),
		slog.String("host", HostOf(p.URL)),
		slog.String("name", p.Name),
		slog.String("token", redacted),
	)
}

// ParsePairing は接続文字列 `swarm-viewer://pair?v=1&url=…&token=…[&name=…][&sessions=…]` を検証して解析する
// （API 仕様 §2.1）。貼り付けでもディープリンクでも同じ。
//
//   - 前後の空白を除去し、4,096 バイトを超えたら ErrPairingFormat
//   - scheme・host（pair）は大文字小文字を区別しない。path・userinfo・port・フラグメントは不可
//   - 同じパラメータが複数あれば ErrPairingFormat（未知のパラメータも対象）。未知のパラメータは無視
//   - v が 1 以外は ErrPairingVersion（欠落は ErrPairingFormat）
//   - url は ValidateURL（失敗は ErrInvalidURL）
//   - token は空でなく、空白・制御文字を含まない（失敗は ErrPairingToken）
//   - name は任意で 1〜100 文字・制御文字なし（無ければ url の host）。不正は ErrPairingFormat
//   - sessions はカンマ区切りのヒント。空要素・制御文字を含む要素・100 文字超の要素は捨てる
//
// クエリは標準のフォームエンコードとして復号する（`+` は空白）。token に `+` を含める場合は %2B にする。
// エラーに入力値は含めない。
func ParsePairing(raw string) (PairingPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > MaxPairingBytes || !utf8.ValidString(raw) || strings.Contains(raw, "#") {
		return PairingPayload{}, ErrPairingFormat
	}
	params, err := pairingParams(raw)
	if err != nil {
		return PairingPayload{}, err
	}
	if v, ok := params["v"]; !ok {
		return PairingPayload{}, ErrPairingFormat
	} else if v != "1" {
		return PairingPayload{}, ErrPairingVersion
	}
	normalized, err := ValidateURL(params["url"])
	if err != nil {
		return PairingPayload{}, err
	}
	token := params["token"]
	if !validToken(token) {
		return PairingPayload{}, ErrPairingToken
	}
	name, err := pairingName(params, normalized)
	if err != nil {
		return PairingPayload{}, err
	}
	return PairingPayload{
		Version:      PairingVersion,
		URL:          normalized,
		Token:        token,
		Name:         name,
		SessionsHint: sessionsHint(params["sessions"]),
	}, nil
}

// pairingParams は scheme・host を検証し、クエリを重複なしの map にして返す。
func pairingParams(raw string) (map[string]string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != pairingScheme || strings.ToLower(u.Host) != pairingHost ||
		(u.Path != "" && u.Path != "/") || u.Opaque != "" || u.User != nil || u.Fragment != "" {
		return nil, ErrPairingFormat
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, ErrPairingFormat
	}
	params := make(map[string]string, len(values))
	for k, vs := range values {
		if len(vs) != 1 {
			return nil, ErrPairingFormat
		}
		params[k] = vs[0]
	}
	return params, nil
}

func validToken(t string) bool {
	if t == "" {
		return false
	}
	for _, r := range t {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func pairingName(params map[string]string, normalizedURL string) (string, error) {
	raw, ok := params["name"]
	if !ok {
		return HostOf(normalizedURL), nil
	}
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > MaxNameRunes || hasControl(name) {
		return "", ErrPairingFormat
	}
	return name, nil
}

func sessionsHint(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" || utf8.RuneCountInString(s) > MaxNameRunes || hasControl(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func hasControl(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}
