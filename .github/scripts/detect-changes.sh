#!/usr/bin/env bash
# ci.yml の changes ジョブ：変更されたファイルから、Go（デスクトップ）側の CI と Android 側の CI を動かすかを決める。
#   使い方：変更されたファイルのパス（リポジトリのルートからの相対。1 行に 1 つ）を標準入力から読む。
#   出力：go=true|false と android=true|false を $GITHUB_OUTPUT へ書く。
#
# パスの分けは workflow の `paths` ではなく、ジョブの `if` で行う。`paths` で workflow ごと止めると、必須チェックが
# 「実行されない」まま pending で残るため。`if` で skip したジョブは、必須チェックでは成功として扱われる。
# 迷ったときは動かす側に倒す（検査を飛ばして壊れた変更を通すより、余計に動かす方がよい）：
#   - 標準入力が空、または `*` だけの行（差分を取れなかった印）のときは、どちらも true
#   - Go は、android/ の外のファイルが 1 つでもあれば true（android/ だけの変更のときだけ false）
#   - Android は、android/・テストベクタ・Android の検査が読む設定・この workflow が変わったときだけ true
set -euo pipefail

: "${GITHUB_OUTPUT:?}"

go=false
android=false
seen=false

while IFS= read -r path || [ -n "$path" ]; do
  [ -n "$path" ] || continue
  seen=true
  if [ "$path" = '*' ]; then
    go=true
    android=true
    continue
  fi
  case "$path" in
  android/*) android=true ;;
  *) go=true ;;
  esac
  case "$path" in
  testdata/vectors/* | .github/workflows/ci.yml | .editorconfig | .linterly.yml | .linterlyignore | package.json | bun.lock)
    android=true
    ;;
  esac
done

if [ "$seen" = false ]; then
  go=true
  android=true
fi

{
  echo "go=$go"
  echo "android=$android"
} >>"$GITHUB_OUTPUT"
