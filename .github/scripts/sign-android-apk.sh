#!/usr/bin/env bash
# release.yml の build-android ジョブ：署名なしの release APK に release 署名鍵で署名し、検証して、配布するファイル名で置く。
#   使い方：bash .github/scripts/sign-android-apk.sh   （入力は環境変数）
#
# 入力（環境変数）：
#   TAG                       リリースのタグ vX.Y.Z（配布するファイル名に使う）
#   VERSION_NAME              期待する versionName（android-version.sh の出力）
#   VERSION_CODE              期待する versionCode（同）
#   UNSIGNED_APK              `./gradlew assembleRelease` が作った署名なしの APK
#   OUT_DIR                   署名済みの APK を置くディレクトリ（無ければ作る）
#   ANDROID_HOME              Android SDK（build-tools の apksigner・zipalign・aapt2 を使う）
#   ANDROID_KEYSTORE_BASE64   keystore（PKCS12）を base64 にした文字列（シークレット）
#   ANDROID_KEYSTORE_PASSWORD keystore のパスワード（シークレット）
#   ANDROID_KEY_ALIAS         鍵のエイリアス（シークレット）
#   ANDROID_KEY_PASSWORD      鍵のパスワード（シークレット）
#   RUNNER_TEMP               keystore を一時的に復元する場所（無ければ mktemp の既定）
#
# 守ること：
#   - パスワードは apksigner に env: で渡す（argv に出さない）。このスクリプトは値を echo しない（set -x もしない）。
#   - keystore は 0600 で一時ディレクトリに復元し、成否にかかわらず（trap）終了時に消す。
#   - 検証：zipalign（16 KB ページ境界を含む）・apksigner verify・debug 署名でないこと・パッケージと版が期待どおりであること。
#     どれか 1 つでも外れたら、署名済みの APK を OUT_DIR に残さず失敗する。
set -euo pipefail

fail() {
  echo "::error::$1"
  exit 1
}

for name in TAG VERSION_NAME VERSION_CODE UNSIGNED_APK OUT_DIR ANDROID_HOME \
  ANDROID_KEYSTORE_BASE64 ANDROID_KEYSTORE_PASSWORD ANDROID_KEY_ALIAS ANDROID_KEY_PASSWORD; do
  # 値は出さず、名前だけを出す
  [ -n "${!name:-}" ] || fail "$name is empty: set it (the signing secrets are ANDROID_KEYSTORE_BASE64, ANDROID_KEYSTORE_PASSWORD, ANDROID_KEY_ALIAS, ANDROID_KEY_PASSWORD)"
done
[ -f "$UNSIGNED_APK" ] || fail "$UNSIGNED_APK does not exist: run ./gradlew assembleRelease first"

# 最も新しい build-tools（apksigner・zipalign・aapt2 がそろっているもの）
build_tools=""
while IFS= read -r dir; do
  if [ -x "$dir/apksigner" ] && [ -x "$dir/zipalign" ] && [ -x "$dir/aapt2" ]; then
    build_tools="$dir"
    break
  fi
done < <(find "$ANDROID_HOME/build-tools" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -rV)
[ -n "$build_tools" ] || fail "no build-tools with apksigner, zipalign and aapt2 under \$ANDROID_HOME/build-tools: install build-tools;35.0.0 or later on the runner"

umask 077
workdir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/android-sign.XXXXXX")"
trap 'rm -rf "$workdir"' EXIT
keystore="$workdir/release.keystore"
signed="$workdir/signed.apk"

# AGP の出力が 4 バイト境界と 16 KB ページ境界（.so）に合っていること。apksigner は署名後に境界を直せないので、署名の前に確かめる
"$build_tools/zipalign" -c -P 16 4 "$UNSIGNED_APK" || fail "$UNSIGNED_APK is not zip-aligned (4 bytes, 16 KB pages for .so)"

printf '%s' "$ANDROID_KEYSTORE_BASE64" | base64 -d >"$keystore" 2>/dev/null || fail "ANDROID_KEYSTORE_BASE64 is not valid base64"
[ -s "$keystore" ] || fail "ANDROID_KEYSTORE_BASE64 decoded to an empty keystore"

# 署名してから、別の入力として読み直して検証する
"$build_tools/apksigner" sign \
  --ks "$keystore" \
  --ks-key-alias "$ANDROID_KEY_ALIAS" \
  --ks-pass env:ANDROID_KEYSTORE_PASSWORD \
  --key-pass env:ANDROID_KEY_PASSWORD \
  --out "$signed" \
  "$UNSIGNED_APK" || fail "apksigner sign failed: check the four signing secrets (the keystore, its password, the key alias and the key password)"
# 署名が終われば keystore は要らない。検証の前に消す
rm -f "$keystore"

verify_out="$("$build_tools/apksigner" verify --verbose --print-certs "$signed")" || fail "apksigner verify failed"
# minSdk 29 では apksigner は v3 だけで署名する（v3 は Android 9 以降）。v2 でも可
grep -qE 'Verified using v[23] scheme \(APK Signature Scheme v[23]\): true' <<<"$verify_out" || fail "the APK is not signed with APK Signature Scheme v2 or v3"
grep -q '^Number of signers: 1$' <<<"$verify_out" || fail "the APK must have exactly one signer"
# debug 署名（Android の既定の debug keystore。DN は CN=Android Debug）の APK は絶対に配布しない
if grep -qiE 'certificate DN:.*Android Debug' <<<"$verify_out"; then
  fail "the APK is signed with a debug certificate: never publish it"
fi
# 証明書の SHA-256 は公開情報。docs/environment/setup.md §11.3 に記録した値と、リリースのたびに照合する
grep -E '^Signer #1 certificate SHA-256 digest:' <<<"$verify_out"

badging="$("$build_tools/aapt2" dump badging "$signed")"
head -n 1 <<<"$badging" | grep -qF "package: name='com.ousiassllc.swarmviewer' versionCode='${VERSION_CODE}' versionName='${VERSION_NAME}'" ||
  fail "package, versionCode or versionName of the APK is not com.ousiassllc.swarmviewer / ${VERSION_CODE} / ${VERSION_NAME}"

mkdir -p "$OUT_DIR"
mv "$signed" "$OUT_DIR/swarm-viewer_${TAG}_android_universal.apk"
echo "signed and verified: $OUT_DIR/swarm-viewer_${TAG}_android_universal.apk"
