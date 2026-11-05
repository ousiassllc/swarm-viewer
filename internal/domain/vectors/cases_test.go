package vectors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/ousiassllc/swarm-viewer/internal/domain"
)

// internal/domain の公開 API を testdata/vectors/*.json で検証する（domain 本体のテストと分けるのは、linterly の 1 ディレクトリ 2,000 行の上限のため）。
// testdata/vectors/*.json は Android 版（Kotlin）と共有するテストベクタ。形式は testdata/vectors/README.md。
// ケース数の下限は、ファイルが空・キーの綴り違いでケースが消えてもテストが空振りしないためのもの。
const (
	minURLCases      = 60
	minPairingCases  = 55
	minToneCases     = 15
	minPercentCases  = 7
	minClassifyCases = 30
	vectorsFakeToken = "test-token-not-secret"
)

// loadVectors は testdata/vectors/<name> を v に読む。未知のキーはエラー（綴り違いの検知）。
func loadVectors(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "vectors", name)) //nolint:gosec // G304: テスト内の固定ディレクトリ（testdata/vectors）のファイル名のみ
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func requireMinCases(t *testing.T, what string, got, want int) {
	t.Helper()
	if got < want {
		t.Fatalf("%s のケースが %d 件しかない（下限 %d）", what, got, want)
	}
}

func TestVectorsURL(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		Cases       []struct {
			Name  string  `json:"name"`
			Input string  `json:"input"`
			Want  *string `json:"want"`
		}
	}
	loadVectors(t, "url.json", &f)
	requireMinCases(t, "url.json", len(f.Cases), minURLCases)
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateURL(c.Input)
			if c.Want == nil {
				if !errors.Is(err, ErrInvalidURL) || got != "" {
					t.Fatalf("ValidateURL(%q) = %q, %v; want ErrInvalidURL", c.Input, got, err)
				}
				return
			}
			if err != nil || got != *c.Want {
				t.Fatalf("ValidateURL(%q) = %q, %v; want %q", c.Input, got, err, *c.Want)
			}
			if again, err := ValidateURL(got); err != nil || again != got {
				t.Fatalf("正規化結果が冪等でない: %q -> %q, %v", got, again, err)
			}
		})
	}
}

type pairingWant struct {
	Version      int      `json:"version"`
	URL          string   `json:"url"`
	Token        string   `json:"token"`
	Name         string   `json:"name"`
	SessionsHint []string `json:"sessions_hint"`
}

func TestVectorsPairing(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		Cases       []struct {
			Name            string       `json:"name"`
			Input           string       `json:"input"`
			Pad             *pairingPad  `json:"pad"`
			Want            *pairingWant `json:"want"`
			WantTokenLength *int         `json:"want_token_length"`
			Error           string       `json:"error"`
		}
	}
	loadVectors(t, "pairing.json", &f)
	requireMinCases(t, "pairing.json", len(f.Cases), minPairingCases)
	errs := map[string]error{
		"format": ErrPairingFormat, "version": ErrPairingVersion, "token": ErrPairingToken, "invalid_url": ErrInvalidURL,
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			in := c.Pad.apply(c.Input)
			got, err := ParsePairing(in)
			if c.Error != "" {
				want, known := errs[c.Error]
				if !known {
					t.Fatalf("未知の error: %q", c.Error)
				}
				if !errors.Is(err, want) || got.Token != "" {
					t.Fatalf("err = %v, token = %q; want %v", err, got.Token, want)
				}
				if strings.Contains(err.Error(), vectorsFakeToken) || (len(in) > 8 && strings.Contains(err.Error(), in)) {
					t.Fatalf("エラーに入力が含まれる: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			want := c.Want
			if c.WantTokenLength != nil {
				want = &pairingWant{
					Version: 1, URL: "http://100.64.0.5:48120/report", Name: "100.64.0.5",
					Token: strings.Repeat("a", *c.WantTokenLength),
				}
			}
			if want == nil {
				t.Fatal("want・want_token_length・error のどれも無い")
			}
			if got.Version != want.Version || got.URL != want.URL || got.Token != want.Token || got.Name != want.Name ||
				!slices.Equal(got.SessionsHint, want.SessionsHint) {
				t.Fatalf("got %+v\nwant %+v", got, *want)
			}
		})
	}
}

// pairingPad は入力の後ろに Fill を ToBytes（UTF-8 のバイト数）まで足し、Suffix を付ける。
type pairingPad struct {
	Fill    string `json:"fill"`
	ToBytes int    `json:"to_bytes"`
	Suffix  string `json:"suffix"`
}

func (p *pairingPad) apply(prefix string) string {
	if p == nil {
		return prefix
	}
	var sb strings.Builder
	sb.WriteString(prefix)
	for sb.Len() < p.ToBytes {
		sb.WriteString(p.Fill)
	}
	sb.WriteString(p.Suffix)
	return sb.String()
}

func TestVectorsUsageTone(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		Cases       []struct {
			Name string `json:"name"`
			Slot struct {
				Available bool     `json:"available"`
				Used      *float64 `json:"used"`
				Unit      string   `json:"unit"`
				Status    string   `json:"status"`
			} `json:"slot"`
			Want string `json:"want"`
		}
		PercentCases []struct {
			Percent float64 `json:"percent"`
			Want    string  `json:"want"`
		} `json:"percent_cases"`
	}
	loadVectors(t, "usage_tone.json", &f)
	requireMinCases(t, "usage_tone.json の cases", len(f.Cases), minToneCases)
	requireMinCases(t, "usage_tone.json の percent_cases", len(f.PercentCases), minPercentCases)
	for _, c := range f.Cases {
		slot := Slot{Available: c.Slot.Available, Used: c.Slot.Used, Unit: c.Slot.Unit, Status: c.Slot.Status}
		if got := UsageTone(slot); string(got) != c.Want {
			t.Errorf("%s: UsageTone = %s; want %s", c.Name, got, c.Want)
		}
	}
	for _, c := range f.PercentCases {
		if got := PercentTone(c.Percent); string(got) != c.Want {
			t.Errorf("PercentTone(%v) = %s; want %s", c.Percent, got, c.Want)
		}
	}
}

func TestVectorsClassify(t *testing.T) {
	t.Parallel()
	var f struct {
		Description string
		Cases       []struct {
			Name       string `json:"name"`
			Error      string `json:"error"`
			FetchKind  string `json:"fetch_kind"`
			FetchCode  string `json:"fetch_code"`
			HTTPStatus int    `json:"http_status"`
			ToolCode   string `json:"tool_code"`
			Want       *struct {
				Kind string `json:"kind"`
				Code string `json:"code"`
			} `json:"want"`
		}
	}
	loadVectors(t, "classify.json", &f)
	requireMinCases(t, "classify.json", len(f.Cases), minClassifyCases)
	kinds := map[string]ErrKind{}
	for k := ErrUnreachable; k <= ErrProtocol; k++ {
		kinds[k.String()] = k
	}
	for _, c := range f.Cases {
		var err error
		switch c.Error {
		case "none":
		case "network":
			err = errors.New("network")
		case "invalid_response":
			err = ErrInvalidResponse
		case "fetch_error":
			err = &FetchError{Kind: kinds[c.FetchKind], Code: c.FetchCode}
		default:
			t.Fatalf("%s: 未知の error %q", c.Name, c.Error)
		}
		got := Classify(err, c.HTTPStatus, c.ToolCode)
		if c.Want == nil {
			if got != nil {
				t.Errorf("%s: got %+v; want nil", c.Name, got)
			}
			continue
		}
		if got == nil || got.Kind != kinds[c.Want.Kind] || got.Code != c.Want.Code {
			t.Errorf("%s: got %+v; want %+v", c.Name, got, c.Want)
			continue
		}
		if c.ToolCode != "" && len(c.ToolCode) > 8 && c.Want.Code != c.ToolCode && strings.Contains(got.Error(), c.ToolCode) {
			t.Errorf("%s: エラー文言に信頼できないコードが含まれる: %v", c.Name, got)
		}
	}
}
