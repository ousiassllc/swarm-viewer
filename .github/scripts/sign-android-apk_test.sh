#!/usr/bin/env bash
# sign-android-apk.sh のテスト。Android SDK・keytool・ネットワークは使わない（build-tools の apksigner・zipalign・aapt2 を
# 偽物に差し替え、スクリプトの組み立て・検査・後始末を確かめる。署名そのものは実物の apksigner で別に確かめる）。
#   使い方：bash .github/scripts/sign-android-apk_test.sh
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/sign-android-apk.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
failed=0

SDK="$WORK/sdk"
mkdir -p "$SDK/build-tools/34.0.0" "$SDK/build-tools/35.0.0"
# 古い build-tools（34.0.0）は apksigner だけで、使われないことを確かめる（最新で 3 つそろったものを選ぶ）
cat >"$SDK/build-tools/34.0.0/apksigner" <<'EOF'
#!/bin/sh
echo old >>"$STUB_LOG"
exit 9
EOF
chmod +x "$SDK/build-tools/34.0.0/apksigner"

# 偽の zipalign：STUB_ALIGN=bad のとき境界が合っていないとする
cat >"$SDK/build-tools/35.0.0/zipalign" <<'EOF'
#!/bin/sh
echo "zipalign $*" >>"$STUB_LOG"
[ "${STUB_ALIGN:-ok}" = ok ]
EOF
# 偽の apksigner：sign は --out にファイルを作り、パスワードの env 名だけを記録する。verify は STUB_DN の署名者を返す
cat >"$SDK/build-tools/35.0.0/apksigner" <<'EOF'
#!/bin/sh
echo "apksigner $*" >>"$STUB_LOG"
if [ "$1" = sign ]; then
  [ "${STUB_SIGN:-ok}" = ok ] || exit 1
  while [ $# -gt 0 ]; do
    [ "$1" = --out ] && out="$2"
    [ "$1" = --ks ] && [ -s "$2" ] && stat -c %a "$2" >>"$STUB_LOG"
    shift
  done
  echo signed >"$out"
else
  echo Verifies
  echo "Verified using v3 scheme (APK Signature Scheme v3): ${STUB_V3:-true}"
  echo "Number of signers: ${STUB_SIGNERS:-1}"
  echo "Signer #1 certificate DN: ${STUB_DN:-CN=Release}"
  echo "Signer #1 certificate SHA-256 digest: 00ff"
fi
EOF
# 偽の aapt2：dump badging の 1 行目を STUB_BADGING で変えられる
cat >"$SDK/build-tools/35.0.0/aapt2" <<'EOF'
#!/bin/sh
echo "${STUB_BADGING:-package: name='com.ousiassllc.swarmviewer' versionCode='1002' versionName='0.1.2' platformBuildVersionName='15'}"
EOF
chmod +x "$SDK/build-tools/35.0.0/"*

printf 'unsigned' >"$WORK/unsigned.apk"
PASSWORD='s3cret-store-pw'
KEY_PASSWORD='s3cret-key-pw'

# run_case 名前 期待(ok|error) 期待するエラーの断片（ok のときは空） [環境変数 KEY=VAL ...]
run_case() {
  local name="$1" want="$2" msg="$3" code=0 log="$WORK/log" out="$WORK/out" tmp="$WORK/tmp" dir="$WORK/dist"
  shift 3
  rm -rf "$out" "$tmp" "$dir"
  mkdir -p "$tmp"
  : >"$log"
  (
    export TAG=v0.1.2 VERSION_NAME=0.1.2 VERSION_CODE=1002 UNSIGNED_APK="$WORK/unsigned.apk" OUT_DIR="$dir"
    export ANDROID_HOME="$SDK" RUNNER_TEMP="$tmp" STUB_LOG="$log"
    keystore_base64="$(printf 'keystore-bytes' | base64 -w0)"
    export ANDROID_KEYSTORE_BASE64="$keystore_base64" ANDROID_KEYSTORE_PASSWORD="$PASSWORD"
    export ANDROID_KEY_ALIAS=release ANDROID_KEY_PASSWORD="$KEY_PASSWORD"
    for kv in "$@"; do export "${kv?}"; done
    bash "$SCRIPT"
  ) >"$out" 2>&1 || code=$?

  local problem=""
  if [ "$want" = ok ]; then
    [ "$code" -eq 0 ] || problem="exit $code"
    [ -f "$dir/swarm-viewer_v0.1.2_android_universal.apk" ] || problem="${problem:+$problem; }no signed APK"
    # keystore は 0600 で復元され、終了後に残らない
    grep -qx 600 "$log" || problem="${problem:+$problem; }the keystore was not 0600"
  else
    [ "$code" -ne 0 ] || problem="exit 0"
    grep -qF -- "$msg" "$out" || problem="${problem:+$problem; }no '$msg' in the output"
    [ ! -e "$dir/swarm-viewer_v0.1.2_android_universal.apk" ] || problem="${problem:+$problem; }the APK was left in OUT_DIR"
  fi
  [ -z "$(ls -A "$tmp")" ] || problem="${problem:+$problem; }the temp dir was not cleaned: $(ls -A "$tmp")"
  # パスワードの値と keystore の中身を、出力にも apksigner の引数にも出さない
  if grep -qF -e "$PASSWORD" -e "$KEY_PASSWORD" -e keystore-bytes "$out" "$log"; then
    problem="${problem:+$problem; }a secret leaked into the output or the apksigner arguments"
  fi

  if [ -z "$problem" ]; then
    pass=$((pass + 1))
    return
  fi
  failed=$((failed + 1))
  echo "FAIL: $name: $problem"
  sed 's/^/    /' "$out" "$log"
}

run_case "署名して検証し、配布名で置く" ok ""
run_case "パスワードは env: で渡す" ok "" # 下で引数の中身を見る
grep -qF -e '--ks-pass env:ANDROID_KEYSTORE_PASSWORD' -e '--key-pass env:ANDROID_KEY_PASSWORD' "$WORK/log" || {
  failed=$((failed + 1))
  echo "FAIL: apksigner did not get the passwords through env:"
}

run_case "keystore のシークレットが空" error "ANDROID_KEYSTORE_BASE64 is empty" ANDROID_KEYSTORE_BASE64=
run_case "keystore のパスワードが空" error "ANDROID_KEYSTORE_PASSWORD is empty" ANDROID_KEYSTORE_PASSWORD=
run_case "鍵のエイリアスが空" error "ANDROID_KEY_ALIAS is empty" ANDROID_KEY_ALIAS=
run_case "鍵のパスワードが空" error "ANDROID_KEY_PASSWORD is empty" ANDROID_KEY_PASSWORD=
run_case "base64 でない keystore" error "not valid base64" "ANDROID_KEYSTORE_BASE64=***"
run_case "署名なしの APK が無い" error "does not exist" UNSIGNED_APK="$WORK/none.apk"
run_case "build-tools が無い" error "no build-tools" ANDROID_HOME="$WORK/empty"
run_case "境界が合っていない" error "not zip-aligned" STUB_ALIGN=bad
run_case "署名の失敗" error "apksigner sign failed" STUB_SIGN=bad
run_case "v2・v3 のどちらでも署名されていない" error "Signature Scheme v2 or v3" STUB_V3=false
run_case "署名者が複数" error "exactly one signer" STUB_SIGNERS=2
run_case "debug 署名は配布しない" error "debug certificate" "STUB_DN=CN=Android Debug, O=Android, C=US"
run_case "versionCode が違う" error "versionCode or versionName" VERSION_CODE=1003
run_case "versionName が違う" error "versionCode or versionName" VERSION_NAME=0.1.3
run_case "パッケージが違う" error "versionCode or versionName" "STUB_BADGING=package: name='com.example.other' versionCode='1002' versionName='0.1.2'"

echo "sign-android-apk_test: $pass passed, $failed failed"
[ "$failed" -eq 0 ]
