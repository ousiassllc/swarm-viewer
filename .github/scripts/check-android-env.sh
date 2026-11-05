#!/usr/bin/env bash
# ci.yml の android-* ジョブと release.yml の build-android ジョブの最初に、ランナーの JDK と Android SDK を確かめる。
# 直前の setup-android-toolchain.sh（環境に無ければ JDK・SDK をダウンロードする）の結果を確かめる。ここでは何も導入しない（docs/environment/setup.md §11.2）。
# 足りないものがあるときは、何が足りないかを 1 行のエラーにして失敗する（Gradle の長いエラーを読まずに済むように）。
#   JAVA_HOME      JDK 17 を指す（`$JAVA_HOME/bin/java` を見る。無ければ PATH の java）
#   ANDROID_HOME   platforms/android-35 と build-tools（apksigner を含む 35.0.0 以降）を持つ Android SDK
# 署名（release）には apksigner・zipalign・aapt2 が要る。CHECK_SIGNING_TOOLS=true のときはそれも確かめる。
set -euo pipefail

fail() {
  echo "::error::$1"
  exit 1
}

java_bin="java"
[ -z "${JAVA_HOME:-}" ] || java_bin="$JAVA_HOME/bin/java"
command -v "$java_bin" >/dev/null 2>&1 || fail "java was not found (JAVA_HOME=${JAVA_HOME:-unset}): run .github/scripts/setup-android-toolchain.sh first, or install JDK 17 and set JAVA_HOME (docs/environment/setup.md §11.2)"
major="$("$java_bin" -version 2>&1 | sed -n '1s/^[^"]*"\([0-9]*\).*/\1/p')"
[ "$major" = 17 ] || fail "JDK ${major:-unknown} is installed, but JDK 17 is required: point JAVA_HOME at JDK 17, or unset it so that setup-android-toolchain.sh downloads one (docs/environment/setup.md §11.2)"

[ -n "${ANDROID_HOME:-}" ] || fail "ANDROID_HOME is not set: run .github/scripts/setup-android-toolchain.sh first, or install the Android SDK and set ANDROID_HOME (docs/environment/setup.md §11.2)"
[ -d "$ANDROID_HOME/platforms/android-35" ] || fail "\$ANDROID_HOME/platforms/android-35 does not exist: install platforms;android-35 (docs/environment/setup.md §11.2)"
[ -d "$ANDROID_HOME/build-tools" ] || fail "\$ANDROID_HOME/build-tools does not exist: install build-tools;35.0.0 or later (docs/environment/setup.md §11.2)"

if [ "${CHECK_SIGNING_TOOLS:-false}" = true ]; then
  for tool in apksigner zipalign aapt2; do
    found=false
    for dir in "$ANDROID_HOME"/build-tools/*/; do
      if [ -x "${dir}${tool}" ]; then
        found=true
        break
      fi
    done
    [ "$found" = true ] || fail "$tool was not found under \$ANDROID_HOME/build-tools: install build-tools;35.0.0 or later (docs/environment/setup.md §11.2)"
  done
fi

echo "JDK ${major} and the Android SDK are available (ANDROID_HOME=${ANDROID_HOME})"
