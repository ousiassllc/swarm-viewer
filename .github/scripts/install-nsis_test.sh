#!/usr/bin/env bash
# install-nsis.sh の download_verified のテスト。ネットワークは使わない（curl を関数で差し替える）。
#   使い方：bash .github/scripts/install-nsis_test.sh
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/install-nsis.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# shellcheck source-path=SCRIPTDIR source=install-nsis.sh
# shellcheck disable=SC2154 # download_error は install-nsis.sh が入れる
source "$SCRIPT"
export NSIS_RETRY_DELAYS="0 0 0" # 待たない

pass=0
failed=0

printf 'nsis' >"$WORK/payload"
good="$(sha256sum "$WORK/payload" | cut -d' ' -f1)"

# curl の差し替え：最初の CURL_FAILS 回は 503 で失敗し、その後は payload を書く。呼び出し回数を CURL_CALLS に数える
CURL_FAILS=0
CURL_CALLS=0
curl() {
  local out="" arg
  while [ $# -gt 0 ]; do
    arg="$1"
    shift
    [ "$arg" = --output ] && out="$1"
  done
  CURL_CALLS=$((CURL_CALLS + 1))
  if [ "$CURL_CALLS" -le "$CURL_FAILS" ]; then
    echo "curl: (22) The requested URL returned error: 503" >&2
    return 22
  fi
  cp "$WORK/payload" "$out"
}

# check 名前 期待(ok|fail) 失敗する回数 sha256 期待する curl の回数 [エラーに含まれるはずの文字列]
check() {
  local name="$1" want="$2" fails="$3" sha="$4" calls="$5" msg="${6:-}" code=0
  CURL_FAILS="$fails"
  CURL_CALLS=0
  rm -f "$WORK/out"
  download_verified "http://example.invalid/nsis.zip" "$sha" "$WORK/out" || code=$?
  if { [ "$want" = ok ] && [ "$code" -eq 0 ] && cmp -s "$WORK/out" "$WORK/payload"; } ||
    { [ "$want" = fail ] && [ "$code" -ne 0 ] && [[ "$download_error" == *"$msg"* ]]; }; then
    if [ "$CURL_CALLS" -eq "$calls" ]; then
      pass=$((pass + 1))
      return
    fi
  fi
  failed=$((failed + 1))
  echo "FAIL: $name: want $want in $calls call(s); got exit $code in $CURL_CALLS call(s), error '$download_error'"
}

check "1 回で取れる" ok 0 "$good" 1
check "503 が 3 回続いても 4 回目で取れる" ok 3 "$good" 4
check "503 が 4 回続けば原因付きで失敗" fail 4 "$good" 4 "503"
check "sha256 が違えば再試行し切って失敗" fail 0 "0000" 4 "sha256 mismatch: got $good, want 0000"

echo "install-nsis_test: $pass passed, $failed failed"
[ "$failed" -eq 0 ]
