#!/usr/bin/env bash
# android-* ジョブと build-android ジョブの最初に、JDK 17 と Android SDK が無ければダウンロードして使えるようにする（docs/environment/setup.md §11.2）。
# ランナーのホストには何も導入しない。ランナーの設定・サービスにも触らない。導入先はランナーのツールキャッシュ（RUNNER_TOOL_CACHE。
# セルフホステッドではジョブをまたいで残るので、2 回目以降はダウンロードしない）。無ければ ~/.cache/swarm-viewer-toolchain。
#   JDK 17        環境（JAVA_HOME）に有れば、それを使う。無ければ Temurin の固定版を取得し、sha256 を検証して展開する
#   Android SDK   環境（ANDROID_HOME）に platforms;android-35 と build-tools が有れば、それを使う。無ければ cmdline-tools の固定版を取得し
#                 （sha256 を検証）、ライセンスに同意して platforms;android-35 と build-tools を入れる
# 結果は GITHUB_ENV に JAVA_HOME・ANDROID_HOME・ANDROID_SDK_ROOT として、GITHUB_PATH に JDK の bin として書く（後続の step で使える）。
# GITHUB_ENV が無い（手元で実行した）ときは、同じ内容を export 文として標準出力に出す。
# 複数のジョブが同じホストで同時に動いても壊れないよう、導入は flock で直列にし、展開は一時ディレクトリへ行ってから rename する。
# 取得は再試行し、失敗したら理由つきの ::error:: で止まる。
set -euo pipefail

JDK_VERSION="17.0.21+9"
JDK_URL="https://github.com/adoptium/temurin17-binaries/releases/download/jdk-17.0.21%2B9/OpenJDK17U-jdk_x64_linux_hotspot_17.0.21_9.tar.gz"
JDK_SHA256="8b9b611d917e3dfd8419d3566b5949226f69b052fed7e109c1368add1dfe8711"

CMDLINE_TOOLS_REV="16111833"
CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-${CMDLINE_TOOLS_REV}_latest.zip"
CMDLINE_TOOLS_SHA256="0877a1d048fe4a24efe2eff536ca4223f7adeb58648bb81909d33c446918cfa8"

# build-tools;36.0.0 は AGP（gradle/libs.versions.toml の agp）が既定で使う版。入れておかないと Gradle が最初の実行中に取得する
SDK_PACKAGES=("platforms;android-35" "build-tools;35.0.0" "build-tools;36.0.0")

RETRY_WAITS="${SETUP_ANDROID_RETRY_WAITS:-10 30 60}"

fail() {
  echo "::error::$1"
  exit 1
}

cache_root="${RUNNER_TOOL_CACHE:-${HOME}/.cache}/swarm-viewer-toolchain"
mkdir -p "$cache_root"

# $1: URL  $2: 保存先  $3: 期待する sha256。取得に失敗したら待ってやり直し、最後まで失敗したら止まる。sha256 が違うときはやり直さず止まる
download() {
  local url="$1" out="$2" want="$3" attempt=1 total waits wait_s got
  read -r -a waits <<<"$RETRY_WAITS"
  total=$((${#waits[@]} + 1))
  while true; do
    if curl --fail --silent --show-error --location --max-time 600 --output "$out" "$url"; then
      got="$(sha256sum "$out" | cut -d' ' -f1)"
      [ "$got" = "$want" ] || fail "sha256 mismatch for ${url}: expected ${want}, got ${got}"
      return 0
    fi
    if [ "$attempt" -ge "$total" ]; then
      fail "could not download ${url} after ${total} attempts"
    fi
    wait_s="${waits[$((attempt - 1))]}"
    echo "::warning::download failed (attempt ${attempt}/${total}): ${url}; retrying in ${wait_s}s"
    sleep "$wait_s"
    attempt=$((attempt + 1))
  done
}

java_major() {
  local bin="$1/bin/java"
  [ -x "$bin" ] || return 1
  "$bin" -version 2>&1 | sed -n '1s/^[^"]*"\([0-9]*\).*/\1/p'
}

# 環境に JDK 17 が有ればそのパスを、無ければ固定版を入れてそのパスを、グローバル変数 java_home に入れる
# （コマンド置換にしない。::warning:: / ::error:: を標準出力に出してランナーに読ませるため）
ensure_jdk() {
  if [ -n "${JAVA_HOME:-}" ] && [ "$(java_major "$JAVA_HOME" || true)" = 17 ]; then
    echo "JDK 17 found in the environment (JAVA_HOME=${JAVA_HOME}): no download"
    java_home="$JAVA_HOME"
    return
  fi
  local dest="${cache_root}/jdk-${JDK_VERSION}"
  if [ "$(java_major "$dest" || true)" = 17 ]; then
    echo "JDK ${JDK_VERSION}: cache hit (${dest}): no download"
    java_home="$dest"
    return
  fi
  echo "JDK ${JDK_VERSION}: cache miss, downloading ${JDK_URL}"
  local tmp
  tmp="$(mktemp -d "${cache_root}/jdk.XXXXXX")"
  download "$JDK_URL" "${tmp}/jdk.tar.gz" "$JDK_SHA256"
  mkdir "${tmp}/x"
  tar -xzf "${tmp}/jdk.tar.gz" -C "${tmp}/x" --strip-components=1
  [ "$(java_major "${tmp}/x" || true)" = 17 ] || fail "the downloaded JDK is not JDK 17"
  rm -rf "$dest"
  mv "${tmp}/x" "$dest"
  rm -rf "$tmp"
  echo "JDK ${JDK_VERSION}: installed to ${dest}"
  java_home="$dest"
}

sdk_ready() {
  local root="$1" pkg
  [ -d "${root}/platforms/android-35" ] || return 1
  ls -d "${root}"/build-tools/*/ >/dev/null 2>&1 || return 1
  for pkg in apksigner zipalign aapt2; do
    ls "${root}"/build-tools/*/"$pkg" >/dev/null 2>&1 || return 1
  done
}

# $1: JAVA_HOME。環境に使える SDK が有ればそのパスを、無ければ固定版を入れてそのパスを、グローバル変数 android_home に入れる
ensure_sdk() {
  local java_home="$1"
  if [ -n "${ANDROID_HOME:-}" ] && sdk_ready "$ANDROID_HOME"; then
    echo "Android SDK found in the environment (ANDROID_HOME=${ANDROID_HOME}): no download"
    android_home="$ANDROID_HOME"
    return
  fi
  local dest="${cache_root}/android-sdk-${CMDLINE_TOOLS_REV}"
  if sdk_ready "$dest"; then
    echo "Android SDK: cache hit (${dest}): no download"
    android_home="$dest"
    return
  fi
  echo "Android SDK: cache miss, downloading ${CMDLINE_TOOLS_URL}"
  local tmp sdk
  tmp="$(mktemp -d "${cache_root}/sdk.XXXXXX")"
  sdk="${tmp}/sdk"
  mkdir -p "${sdk}/cmdline-tools"
  download "$CMDLINE_TOOLS_URL" "${tmp}/cmdline-tools.zip" "$CMDLINE_TOOLS_SHA256"
  unzip -q "${tmp}/cmdline-tools.zip" -d "${tmp}/unzipped"
  mv "${tmp}/unzipped/cmdline-tools" "${sdk}/cmdline-tools/latest"
  # ライセンスへの同意（yes の SIGPIPE を pipefail に拾わせない）。SDK の導入に必要な操作で、導入先はこの一時ディレクトリだけ
  (
    set +o pipefail
    yes | JAVA_HOME="$java_home" "${sdk}/cmdline-tools/latest/bin/sdkmanager" --sdk_root="$sdk" --licenses >/dev/null
  ) || fail "sdkmanager --licenses failed"
  JAVA_HOME="$java_home" "${sdk}/cmdline-tools/latest/bin/sdkmanager" --sdk_root="$sdk" "${SDK_PACKAGES[@]}" \
    || fail "sdkmanager could not install ${SDK_PACKAGES[*]}"
  sdk_ready "$sdk" || fail "the downloaded Android SDK lacks platforms;android-35 or build-tools (apksigner, zipalign, aapt2)"
  rm -rf "$dest"
  mv "$sdk" "$dest"
  rm -rf "$tmp"
  echo "Android SDK: installed to ${dest}"
  android_home="$dest"
}

exec 9>"${cache_root}/.lock"
flock 9

ensure_jdk
ensure_sdk "$java_home"

if [ -n "${GITHUB_ENV:-}" ]; then
  {
    echo "JAVA_HOME=${java_home}"
    echo "ANDROID_HOME=${android_home}"
    echo "ANDROID_SDK_ROOT=${android_home}"
  } >>"$GITHUB_ENV"
  echo "${java_home}/bin" >>"${GITHUB_PATH:?GITHUB_PATH is required with GITHUB_ENV}"
else
  echo "export JAVA_HOME='${java_home}' ANDROID_HOME='${android_home}' ANDROID_SDK_ROOT='${android_home}'"
fi
