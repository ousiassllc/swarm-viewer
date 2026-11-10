#!/usr/bin/env bash
# release.yml の build ジョブ：タグ vX.Y.Z の X.Y.Z をビルド設定に埋め込む。タグを版の唯一の元とする。
#   使い方：bash .github/scripts/set-version.sh vX.Y.Z   （リポジトリのルートで実行。task と wails3 が PATH にあること）
#
# 1. build/config.yml の info.version を X.Y.Z に書き換える
# 2. `task common:update:build-assets`（wails3 update build-assets）で、config.yml から次のファイルを再生成する
#      build/darwin/Info.plist       CFBundleShortVersionString・CFBundleVersion
#      build/windows/info.json       fixed.file_version・info.0409.ProductVersion（Windows のファイルのプロパティ）
#      build/windows/nsis/wails_tools.nsh   INFO_PRODUCTVERSION（インストーラのバージョン情報）
#      build/windows/wails.exe.manifest     assemblyIdentity の version
#    このコマンドが書き換えるのは「updatable」なアセット（上記と Info.dev.plist・linux の desktop / nfpm）だけで、
#    project.nsi・各 Taskfile は触らない。plist は既存の内容にマージされる（キーの並びと空白が変わるだけ）。
# 3. internal/buildinfo/buildinfo.go の `var Version = "dev"` を X.Y.Z に書き換える。アプリが自分の版を知るための値で
#    （新しい版の確認・自動アップデート。Issue #109）、リポジトリには "dev" のまま置く。ワークツリーだけの書き換えでコミットしない
# 4. 上の各ファイルに X.Y.Z が入ったことを確かめる。入っていなければ失敗する（古い版のままリリースしない）。
set -euo pipefail

tag="${1:?usage: set-version.sh vX.Y.Z}"
readonly VERSION_RE='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
[[ "$tag" =~ $VERSION_RE ]] || {
  echo "::error::tag $tag is not vX.Y.Z"
  exit 1
}
version="${tag#v}"

# 版の形（数字と . だけ）は上で確かめてあるので、sed の置換にそのまま使える
sed -i.bak -E "s/^([[:space:]]+version:[[:space:]]*)\"[^\"]*\"/\1\"${version}\"/" build/config.yml
rm -f build/config.yml.bak

# アプリ自身の版（internal/buildinfo）。タグを版の唯一の元とするため、config.yml と同じここで書き換える
sed -i.bak -E "s/^(var Version = )\"[^\"]*\"/\1\"${version}\"/" internal/buildinfo/buildinfo.go
rm -f internal/buildinfo/buildinfo.go.bak

task common:update:build-assets

fail() {
  echo "::error::$1"
  exit 1
}

# plist の <key>$1</key> の次の行が <string>$version</string> であること
check_plist() {
  awk -v key="$1" '
    $0 ~ "<key>" key "</key>" { getline; print; found = 1 }
    END { if (!found) exit 1 }
  ' build/darwin/Info.plist | grep -qF "<string>${version}</string>" ||
    fail "build/darwin/Info.plist: $1 is not ${version}"
}

grep -qE "^[[:space:]]+version:[[:space:]]*\"${version}\"" build/config.yml ||
  fail "build/config.yml: info.version is not ${version}"
grep -qxF "var Version = \"${version}\"" internal/buildinfo/buildinfo.go ||
  fail "internal/buildinfo/buildinfo.go: Version is not ${version}"
check_plist CFBundleShortVersionString
check_plist CFBundleVersion
grep -qE "\"file_version\":[[:space:]]*\"${version}\"" build/windows/info.json ||
  fail "build/windows/info.json: file_version is not ${version}"
grep -qE "\"ProductVersion\":[[:space:]]*\"${version}\"" build/windows/info.json ||
  fail "build/windows/info.json: ProductVersion is not ${version}"
grep -qF "!define INFO_PRODUCTVERSION \"${version}\"" build/windows/nsis/wails_tools.nsh ||
  fail "build/windows/nsis/wails_tools.nsh: INFO_PRODUCTVERSION is not ${version}"
grep -qE "name=\"com\.ousiassllc\.swarm-viewer\" version=\"${version}\"" build/windows/wails.exe.manifest ||
  fail "build/windows/wails.exe.manifest: version is not ${version}"

echo "version ${version} is set in build/config.yml, internal/buildinfo and the build assets"
