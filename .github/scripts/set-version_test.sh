#!/usr/bin/env bash
# set-version.sh のテスト（アプリ自身の版 internal/buildinfo の書き換え。Issue #109）。task・wails3 は偽物に差し替え、
# `task common:update:build-assets` が生成するファイルは、あらかじめ新しい版で置いておく（ネットワーク・wails3 不要）。
#   使い方：bash .github/scripts/set-version_test.sh
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/set-version.sh"
REPO="$(cd "$(dirname "$SCRIPT")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
failed=0

check() { # check <name> <condition の終了コード>
  if [ "$2" -eq 0 ]; then pass=$((pass + 1)); else failed=$((failed + 1)); echo "FAIL: $1"; fi
}

# 偽の task：何も生成しない（生成物は seed が置く）
mkdir -p "$WORK/bin"
printf '#!/bin/sh\nexit 0\n' >"$WORK/bin/task"
chmod +x "$WORK/bin/task"

# seed <dir> <生成物に入れる版>：リポジトリの config.yml・buildinfo.go と、task が生成するはずのファイル
seed() {
  local dir="$1" version="$2"
  mkdir -p "$dir/build/darwin" "$dir/build/windows/nsis" "$dir/internal/buildinfo"
  cp "$REPO/build/config.yml" "$dir/build/config.yml"
  cp "$REPO/internal/buildinfo/buildinfo.go" "$dir/internal/buildinfo/buildinfo.go"
  printf '<key>CFBundleShortVersionString</key>\n<string>%s</string>\n<key>CFBundleVersion</key>\n<string>%s</string>\n' "$version" "$version" >"$dir/build/darwin/Info.plist"
  printf '{"file_version": "%s", "ProductVersion": "%s"}\n' "$version" "$version" >"$dir/build/windows/info.json"
  printf '!define INFO_PRODUCTVERSION "%s"\n' "$version" >"$dir/build/windows/nsis/wails_tools.nsh"
  printf '<assemblyIdentity name="com.ousiassllc.swarm-viewer" version="%s"/>\n' "$version" >"$dir/build/windows/wails.exe.manifest"
}

run() { # run <dir> <tag>
  (cd "$1" && PATH="$WORK/bin:$PATH" bash "$SCRIPT" "$2") >"$WORK/out" 2>&1
}

# リポジトリの値は開発ビルド（dev）
grep -qxF 'var Version = "dev"' "$REPO/internal/buildinfo/buildinfo.go"
check "リポジトリの buildinfo は dev" $?

# 正常：タグの X.Y.Z が buildinfo.go に入り、.bak が残らない
seed "$WORK/ok" 0.1.13
run "$WORK/ok" v0.1.13
check "正常終了" $?
grep -qxF 'var Version = "0.1.13"' "$WORK/ok/internal/buildinfo/buildinfo.go"
check "buildinfo.go の版が書き換わる" $?
[ -z "$(find "$WORK/ok" -name '*.bak')" ]
check ".bak が残らない" $?
grep -q 'version: "0.1.13"' "$WORK/ok/build/config.yml"
check "config.yml も書き換わる" $?

# 桁が増えても数値のまま（v0.1.9 → v0.1.13 のような版）
seed "$WORK/ten" 0.10.100
run "$WORK/ten" v0.10.100 && grep -qxF 'var Version = "0.10.100"' "$WORK/ten/internal/buildinfo/buildinfo.go"
check "0.10.100 が入る" $?

# buildinfo.go の形が変わって書き換えられないときは失敗する（dev のままリリースしない）
seed "$WORK/shape" 0.1.13
printf 'package buildinfo\n\nvar Version = "dev" // changed shape\n' >"$WORK/shape/internal/buildinfo/buildinfo.go"
if run "$WORK/shape" v0.1.13; then rc=1; else rc=0; fi
check "buildinfo の形が変わったら失敗" $rc

# タグの形が違えば失敗する
seed "$WORK/bad" 0.1.13
for tag in 0.1.13 v0.1 v01.1.13 v0.1.13-rc1; do
  if run "$WORK/bad" "$tag"; then rc=1; else rc=0; fi
  check "不正なタグ $tag は失敗" $rc
done
grep -qxF 'var Version = "dev"' "$WORK/bad/internal/buildinfo/buildinfo.go"
check "不正なタグでは書き換えない" $?

echo "set-version_test: ${pass} passed, ${failed} failed"
[ "$failed" -eq 0 ]
