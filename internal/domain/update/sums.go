package update

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
)

// SHA256SUMS の読み取りの失敗。
var (
	// ErrBadSums は SHA256SUMS の形が `<64 桁の 16 進>  <ファイル名>` の行でない。
	ErrBadSums = errors.New("update: unreadable SHA256SUMS")
	// ErrSumMissing は SHA256SUMS に対象のファイルの行が無い、または複数ある。
	ErrSumMissing = errors.New("update: SHA256SUMS has no unique entry for the file")
)

// Sums は SHA256SUMS の内容（ファイル名 → 小文字の 16 進ハッシュ）。同名の行が複数あるファイルは重複として覚える。
type Sums struct {
	hashes map[string]string
	dups   map[string]bool
}

// ParseSums は sha256sum 形式（`<64 桁の 16 進>  <名前>`。バイナリ表記の `<ハッシュ> *<名前>` も可）を読む。空行は読み飛ばす。
// 形の合わない行が 1 つでもあれば ErrBadSums（部分的に信用しない）。
func ParseSums(body []byte) (Sums, error) {
	s := Sums{hashes: map[string]string{}, dups: map[string]bool{}}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		hash, name, ok := splitSumLine(line)
		if !ok {
			return Sums{}, ErrBadSums
		}
		if _, seen := s.hashes[name]; seen {
			s.dups[name] = true
		}
		s.hashes[name] = hash
	}
	if sc.Err() != nil {
		return Sums{}, ErrBadSums
	}
	return s, nil
}

// splitSumLine は 1 行を（小文字の 16 進ハッシュ, 名前）に分ける。
func splitSumLine(line string) (hash, name string, ok bool) {
	const hexLen = 64
	if len(line) < hexLen+3 || (line[hexLen:hexLen+2] != "  " && line[hexLen:hexLen+2] != " *") {
		return "", "", false
	}
	hash, name = strings.ToLower(line[:hexLen]), line[hexLen+2:]
	if _, err := hex.DecodeString(hash); err != nil || name == "" || strings.TrimSpace(name) != name {
		return "", "", false
	}
	return hash, name, true
}

// Lookup は name の行のハッシュ（小文字の 16 進）を返す。無い・複数あるときは ErrSumMissing。
func (s Sums) Lookup(name string) (string, error) {
	h, ok := s.hashes[name]
	if !ok || s.dups[name] {
		return "", ErrSumMissing
	}
	return h, nil
}
