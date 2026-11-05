#!/usr/bin/env bash
# release.yml の build-windows ジョブ：NSIS（makensis）を 3.11 に固定して用意する。Chocolatey は使わない（一時障害の 503 でリリースが止まったため。Issue #6）。
#   使い方：bash .github/scripts/install-nsis.sh   （GitHub ホストの Windows ランナーの bash で実行）
#
# 1. makensis が PATH か既定の場所にあり、版が NSIS_VERSION と一致すればそれを使う（イメージ同梱の有無に頼らない）
# 2. そうでなければ、公式配布（SourceForge の nsis-<版>.zip。固定 URL）を取得し、sha256 を検証してから展開する。
#    取得（と検証）は失敗したらバックオフ付きで再試行する（NSIS_RETRY_DELAYS の秒数だけ待つ）。試行し切ったら原因付きで失敗する
# 3. 使う makensis のディレクトリを GITHUB_PATH に足し、`makensis -VERSION` をログに出す
#
# 版を上げるときは NSIS_VERSION と NSIS_SHA256 を一緒に直す。sha256 は公式配布物（SourceForge の nsis-3.11.zip）を実際に取得して確かめた値：
#   https://downloads.sourceforge.net/project/nsis/NSIS%203/3.11/nsis-3.11.zip
set -euo pipefail

: "${NSIS_VERSION:=3.11}"
: "${NSIS_SHA256:=c7d27f780ddb6cffb4730138cd1591e841f4b7edb155856901cdf5f214394fa1}"
: "${NSIS_URL:=https://downloads.sourceforge.net/project/nsis/NSIS%203/${NSIS_VERSION}/nsis-${NSIS_VERSION}.zip}"
# 失敗から次の試行までの待ち（秒）。要素数 + 1 回まで試す（既定は 4 回、合計 100 秒待つ）
: "${NSIS_RETRY_DELAYS:=10 30 60}"
readonly DEFAULT_NSIS_DIR='/c/Program Files (x86)/NSIS'

fail() {
  echo "::error::$1"
  exit 1
}

# Git bash（Windows）と GitHub Actions の間でパスの形を変える（RUNNER_TEMP は D:\a\_temp 形式。GITHUB_PATH も Windows 形式）。cygpath が無い環境（手元・テスト）ではそのまま
to_posix() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -u "$1"; else echo "$1"; fi
}
to_windows() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else echo "$1"; fi
}

# 使える makensis の版（"v3.11" 形式）。無い・動かないときは空
makensis_version() {
  makensis -VERSION 2>/dev/null | tr -d '\r' || true
}

# url を取得して sha256 を検証し、dest に置く。試行し切って失敗したら、最後の原因を $download_error に入れて 1 を返す
download_verified() {
  local url="$1" sha256="$2" dest="$3" attempt=1 delay got
  local -a delays
  read -r -a delays <<<"$NSIS_RETRY_DELAYS"
  download_error=""
  while :; do
    if curl --fail --location --silent --show-error --connect-timeout 30 --max-time 300 --output "$dest" "$url" 2>"$dest.err"; then
      got="$(sha256sum "$dest" | cut -d' ' -f1)"
      if [ "$got" = "$sha256" ]; then
        return 0
      fi
      download_error="sha256 mismatch: got $got, want $sha256"
    else
      download_error="curl failed: $(tr '\n' ' ' <"$dest.err")"
    fi
    if [ "$attempt" -gt "${#delays[@]}" ]; then
      return 1
    fi
    delay="${delays[$((attempt - 1))]}"
    echo "::warning::NSIS download attempt $attempt failed ($download_error); retrying in ${delay}s"
    sleep "$delay"
    attempt=$((attempt + 1))
  done
}

# zip を dir に展開する。Windows ランナーは PowerShell（Git bash に unzip が無い）、それ以外（手元・テスト）は unzip
extract_zip() {
  local zip="$1" dir="$2"
  if command -v powershell >/dev/null 2>&1; then
    powershell -NoProfile -Command "Expand-Archive -LiteralPath '$(to_windows "$zip")' -DestinationPath '$(to_windows "$dir")' -Force"
  else
    unzip -q -o "$zip" -d "$dir"
  fi
}

# makensis のあるディレクトリを、この後のステップと、このスクリプトの残りの PATH に足す
add_to_path() {
  local dir="$1"
  PATH="$dir:$PATH"
  export PATH
  if [ -n "${GITHUB_PATH:-}" ]; then
    to_windows "$dir" >>"$GITHUB_PATH"
  fi
}

main() {
  local want="v${NSIS_VERSION}" got dir tmp attempts
  PATH="$DEFAULT_NSIS_DIR:$PATH"
  export PATH
  got="$(makensis_version)"
  if [ "$got" = "$want" ]; then
    echo "using the installed makensis ($got)"
    add_to_path "$(dirname "$(command -v makensis)")"
    echo "makensis $got"
    return 0
  fi
  echo "makensis is missing or is not $want (found: '${got:-none}'): installing NSIS ${NSIS_VERSION} from ${NSIS_URL}"

  tmp="$(to_posix "${RUNNER_TEMP:-$(mktemp -d)}")"
  dir="$tmp/nsis"
  rm -rf "$dir"
  mkdir -p "$dir"
  if ! download_verified "$NSIS_URL" "$NSIS_SHA256" "$tmp/nsis.zip"; then
    attempts=$(($(wc -w <<<"$NSIS_RETRY_DELAYS") + 1))
    fail "cannot get NSIS ${NSIS_VERSION} from ${NSIS_URL} after ${attempts} attempts: ${download_error}"
  fi
  extract_zip "$tmp/nsis.zip" "$dir" || fail "cannot extract the NSIS ${NSIS_VERSION} zip"
  add_to_path "$dir/nsis-${NSIS_VERSION}"

  got="$(makensis_version)"
  [ "$got" = "$want" ] || fail "makensis -VERSION is '${got:-none}' after the install, want '$want'"
  echo "makensis $got"
}

# source されたとき（テスト）は何も実行しない
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
