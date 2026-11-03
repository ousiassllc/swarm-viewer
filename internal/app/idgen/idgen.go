// Package idgen は接続 ID（ULID）を発行する service.IDGenerator の実装。
package idgen

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/oklog/ulid/v2"
)

// ULID は oklog/ulid による IDGenerator。ゼロ値は実時間と crypto/rand を使う。
type ULID struct {
	// Now と Entropy はテスト用の差し込み口。nil なら time.Now / crypto/rand。
	Now     func() time.Time
	Entropy io.Reader
}

// NewID は新しい ULID（26 文字の Crockford base32）を返す。
func (g ULID) NewID() string {
	now, entropy := g.Now, g.Entropy
	if now == nil {
		now = time.Now
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	// 乱数源が失敗するのは OS の乱数が使えないときだけ。ID を作れないまま続行しない。
	return ulid.MustNew(ulid.Timestamp(now()), entropy).String()
}
