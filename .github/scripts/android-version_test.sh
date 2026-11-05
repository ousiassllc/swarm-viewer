#!/usr/bin/env bash
# android-version.sh の表駆動テスト。ネットワークは使わない。
#   使い方：bash .github/scripts/android-version_test.sh
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/android-version.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
failed=0

# run_case 名前 期待 タグ
#   期待：「version_name=X.Y.Z version_code=N」または「error」（終了コードが 0 でなく、出力ファイルに何も書かない）
run_case() {
  local name="$1" want="$2" tag="$3" out="$WORK/out" code=0 got
  : >"$out"
  GITHUB_OUTPUT="$out" bash "$SCRIPT" "$tag" >"$WORK/log" 2>&1 || code=$?
  got="$(tr '\n' ' ' <"$out" | sed 's/ $//')"
  if [ "$want" = error ]; then
    if [ "$code" -ne 0 ] && [ -z "$got" ]; then
      pass=$((pass + 1))
      return
    fi
  elif [ "$code" -eq 0 ] && [ "$got" = "$want" ]; then
    pass=$((pass + 1))
    return
  fi
  failed=$((failed + 1))
  echo "FAIL: $name: want '$want', got '$got' (exit $code)"
  sed 's/^/    /' "$WORK/log"
}

# --- 正しい版 ---
run_case "初回の v0.1.0" "version_name=0.1.0 version_code=1000" v0.1.0
run_case "v0.1.1" "version_name=0.1.1 version_code=1001" v0.1.1
run_case "v0.1.10 は v0.1.9 より大きい（数として比較）" "version_name=0.1.10 version_code=1010" v0.1.10
run_case "v0.2.0" "version_name=0.2.0 version_code=2000" v0.2.0
run_case "v0.10.0" "version_name=0.10.0 version_code=10000" v0.10.0
run_case "v1.0.0" "version_name=1.0.0 version_code=1000000" v1.0.0
run_case "v1.2.3" "version_name=1.2.3 version_code=1002003" v1.2.3
run_case "v0.0.1（最小の版）" "version_name=0.0.1 version_code=1" v0.0.1
run_case "Y・Z の上限 v0.999.999" "version_name=0.999.999 version_code=999999" v0.999.999
run_case "X の上限 v2099.999.999" "version_name=2099.999.999 version_code=2099999999" v2099.999.999

# --- 単調増加（隣り合う版の組）は下で別に確かめる ---

# --- 拒否する版 ---
run_case "v0.0.0 は versionCode が 0 になる" error v0.0.0
run_case "Z が 1000 以上" error v0.1.1000
run_case "Y が 1000 以上" error v0.1000.0
run_case "X が 2100 以上" error v2100.0.0
run_case "桁あふれする巨大な数" error v99999999999999999999.0.0
run_case "プレリリース形" error v1.0.0-rc1
run_case "ビルド付き" error v1.0.0+b1
run_case "v 無し" error 1.0.0
run_case "vX.Y" error v1.0
run_case "先頭の 0" error v01.2.3
run_case "末尾に文字" error v1.0.0x
run_case "空のタグ" error ""

# --- versionCode が版の大小の順に単調増加すること ---
prev=0
for tag in v0.0.1 v0.0.9 v0.0.10 v0.0.999 v0.1.0 v0.1.9 v0.1.10 v0.9.999 v0.10.0 v0.999.999 v1.0.0 v1.0.1 v1.1.0 v2.0.0 v2099.999.999; do
  : >"$WORK/out"
  GITHUB_OUTPUT="$WORK/out" bash "$SCRIPT" "$tag" >/dev/null
  code="$(sed -n 's/^version_code=//p' "$WORK/out")"
  if [ "$code" -le "$prev" ]; then
    failed=$((failed + 1))
    echo "FAIL: versionCode of $tag ($code) is not greater than the previous ($prev)"
  else
    pass=$((pass + 1))
  fi
  prev="$code"
done

echo "android-version_test: $pass passed, $failed failed"
[ "$failed" -eq 0 ]
