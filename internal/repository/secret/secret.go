// Package secret は OS の秘密領域（macOS Keychain・Windows Credential Manager）に token を保存する。
//
// go-keyring を薄く包むだけで、フォールバック（ファイル・メモリ）は持たない。
// 秘密領域が使えないときはエラーを返す（NFR-SEC-2）。
//
// macOS での token の受け渡し: 採用版 go-keyring v0.2.8 は Set で `security -i` の標準入力に
// `add-generic-password ... -w <token>` を流す（argv に token を載せない。keyring_darwin.go）。
// 引数渡し（`-w <password>`）だったのは v0.1.x まで。そのため独自の darwin 実装は持たない。
package secret

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	// serviceName は秘密領域のサービス名。
	serviceName = "swarm-viewer"
	// keyPrefix は接続 ID から秘密領域のキー（アカウント名）を作る接頭辞。
	keyPrefix = "connection:"
	// maxTokenBytes は token の上限。Windows の資格情報は 2,560 バイト、macOS は `security -i` の
	// 1 行 4,096 バイト（base64 化で 4/3 倍）が上限のため、余裕を持って下回る値にする。
	// go-keyring の macOS 実装は上限超過を `security` の起動後に検出して子プロセスを残すため、
	// 起動前にここで弾く。
	maxTokenBytes = 2048
)

var (
	// ErrNotFound は接続に対応する token が無いとき。service は「token なし」（再ペアリング）として扱う。
	ErrNotFound = errors.New("secret: token not found")
	// ErrInvalidArgument は接続 ID・token が空、または token が大きすぎるとき。値は含めない。
	ErrInvalidArgument = errors.New("secret: invalid connection id or token")
)

// Store は OS 秘密領域への token の保存先。
type Store struct{ service string }

// NewStore は サービス名 "swarm-viewer" の Store を返す。
func NewStore() *Store { return &Store{service: serviceName} }

func (s *Store) key(connID string) string { return keyPrefix + connID }

// Set は接続の token を保存する（既存があれば上書き）。キーは "connection:{connID}"。
func (s *Store) Set(connID, token string) error {
	if connID == "" || token == "" || len(token) > maxTokenBytes {
		return ErrInvalidArgument
	}
	if err := keyring.Set(s.service, s.key(connID), token); err != nil {
		return fmt.Errorf("secret: set: %w", err)
	}
	return nil
}

// Get は接続の token を返す。無ければ ErrNotFound。
func (s *Store) Get(connID string) (string, error) {
	if connID == "" {
		return "", ErrNotFound
	}
	token, err := keyring.Get(s.service, s.key(connID))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("secret: get: %w", err)
	}
	return token, nil
}

// Delete は接続の token を削除する。無くてもエラーにしない。
func (s *Store) Delete(connID string) error {
	if connID == "" {
		return nil
	}
	err := keyring.Delete(s.service, s.key(connID))
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("secret: delete: %w", err)
	}
	return nil
}
