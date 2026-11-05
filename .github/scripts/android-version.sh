#!/usr/bin/env bash
# release.yml の build-android ジョブ：タグ vX.Y.Z から、APK の versionName と versionCode を決める。タグを版の唯一の元とする。
#   使い方：bash .github/scripts/android-version.sh vX.Y.Z
#   出力：version_name=X.Y.Z と version_code=N を、$GITHUB_OUTPUT があればそこへ、無ければ標準出力へ書く。
#
# versionName = X.Y.Z
# versionCode = X × 1,000,000 + Y × 1,000 + Z（docs/requirements/non-functional.md NFR-AND-DST-4）
#   版が上がれば必ず大きくなる（上書きアップデートに必要）。Y・Z は 999 まで、X は 2099 まで（versionCode の上限 2,100,000,000 に収まる範囲）。
#   範囲を超える版はエラーにする（桁あふれで versionCode が小さくなり、上書きできなくなるのを防ぐ）。
set -euo pipefail

tag="${1:?usage: android-version.sh vX.Y.Z}"
readonly VERSION_RE='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
readonly MAX_MAJOR=2099
readonly MAX_MINOR_PATCH=999

fail() {
  echo "::error::$1"
  exit 1
}

[[ "$tag" =~ $VERSION_RE ]] || fail "tag $tag is not vX.Y.Z"
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

# 桁の多い数は bash の算術で溢れるため、比較は文字数を先に見る（正規表現で先頭の 0 は無いと保証済み）
too_big() { [ "${#1}" -gt "${#2}" ] || { [ "${#1}" -eq "${#2}" ] && [ "$1" -gt "$2" ]; }; }
too_big "$major" "$MAX_MAJOR" && fail "major $major of $tag is above $MAX_MAJOR: versionCode would exceed 2100000000"
too_big "$minor" "$MAX_MINOR_PATCH" && fail "minor $minor of $tag is above $MAX_MINOR_PATCH: versionCode would not increase monotonically"
too_big "$patch" "$MAX_MINOR_PATCH" && fail "patch $patch of $tag is above $MAX_MINOR_PATCH: versionCode would not increase monotonically"

code=$((major * 1000000 + minor * 1000 + patch))
[ "$code" -ge 1 ] || fail "$tag gives versionCode $code: versionCode must be at least 1 (v0.0.0 is not releasable)"

{
  echo "version_name=${major}.${minor}.${patch}"
  echo "version_code=${code}"
} >>"${GITHUB_OUTPUT:-/dev/stdout}"
