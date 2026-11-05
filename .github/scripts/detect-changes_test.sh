#!/usr/bin/env bash
# detect-changes.sh の表駆動テスト。ネットワーク・git は使わない（変更されたファイルの一覧を標準入力に渡す）。
#   使い方：bash .github/scripts/detect-changes_test.sh
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/detect-changes.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
failed=0

# run_case 名前 期待（「go=… android=…」） 変更されたファイル（改行区切りの 1 つの文字列。空文字列は「出力なし」）
run_case() {
  local name="$1" want="$2" files="$3" out="$WORK/out" got code=0
  : >"$out"
  printf '%s' "$files" | GITHUB_OUTPUT="$out" bash "$SCRIPT" >/dev/null 2>&1 || code=$?
  got="$(tr '\n' ' ' <"$out" | sed 's/ $//')"
  if [ "$code" -eq 0 ] && [ "$got" = "$want" ]; then
    pass=$((pass + 1))
    return
  fi
  failed=$((failed + 1))
  echo "FAIL: $name: want '$want', got '$got' (exit $code)"
}

nl=$'\n'

run_case "Go だけの変更" "go=true android=false" "internal/domain/foo.go"
run_case "android/ だけの変更" "go=false android=true" "android/app/src/main/kotlin/A.kt"
run_case "android/ の複数ファイルだけ" "go=false android=true" "android/app/build.gradle.kts${nl}android/README.md${nl}android/gradle/libs.versions.toml"
run_case "Go と Android の両方" "go=true android=true" "internal/web/a.go${nl}android/app/build.gradle.kts"
run_case "テストベクタは両方" "go=true android=true" "testdata/vectors/pairing.json"
run_case "ci.yml は両方" "go=true android=true" ".github/workflows/ci.yml"
run_case "release.yml だけなら Go 側" "go=true android=false" ".github/workflows/release.yml"
run_case "リリース用スクリプトは Go 側（テストは test ジョブが走らせる）" "go=true android=false" ".github/scripts/android-version.sh"
run_case "docs だけなら Go 側" "go=true android=false" "docs/environment/setup.md"
run_case ".editorconfig は ktlint が読むので両方" "go=true android=true" ".editorconfig"
run_case "linterly の設定は両方" "go=true android=true" ".linterly.yml"
run_case "android という名前の別のパスは Go 側" "go=true android=false" "androidx/foo.go"
run_case "android/ の中に見えて外のファイル（先頭が違う）は Go 側" "go=true android=false" "docs/android/overview.md"
run_case "差分を取れなかった印（*）は両方" "go=true android=true" "*"
run_case "空の変更は両方（安全側）" "go=true android=true" ""
run_case "空行だけも両方（安全側）" "go=true android=true" "$nl$nl"
run_case "最後の行に改行が無くても読む" "go=false android=true" "android/README.md"

echo "detect-changes_test: $pass passed, $failed failed"
[ "$failed" -eq 0 ]
