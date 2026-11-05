# swarm-viewer Android

swarm-viewer の Android 版（Kotlin + Jetpack Compose）。親 Issue は [#7](https://github.com/ousiassllc/swarm-viewer/issues/7)。現状は土台（空の Compose 画面・lint・format・ユニットテスト）だけで、画面・通信・保存・通知は後続の Issue で実装する。

| 項目 | 内容 |
|------|------|
| applicationId / パッケージ | `com.ousiassllc.swarmviewer` |
| minSdk / targetSdk / compileSdk | 29 / 35 / 35 |
| ビルド | Gradle（Kotlin DSL）。版は `gradle/libs.versions.toml`、Gradle 本体は `gradle/wrapper/gradle-wrapper.properties` で固定 |
| UI | Jetpack Compose + Material 3 |
| lint / format | Android Lint（警告もエラー扱い）、ktlint（Gradle プラグイン。設定は ルートの `.editorconfig`） |

## 必要なツール

- **JDK 17**（`JAVA_HOME` を設定する）
- **Android SDK**（`ANDROID_HOME` を設定するか、`android/local.properties` に `sdk.dir=<パス>` を書く。`local.properties` はコミットしない）
  - `platforms;android-35` と `build-tools;35.0.0` 以降
- 導入手順は [`docs/environment/setup.md`](../docs/environment/setup.md) §11 の Android 版に従う。設計は [`docs/android/overview.md`](../docs/android/overview.md)

Gradle 本体はラッパー（`./gradlew`）が取得するため、別途インストールしない。

## コマンド

`android/` で実行する。

| コマンド | 内容 |
|----------|------|
| `./gradlew assembleDebug` | debug APK を `app/build/outputs/apk/debug/app-debug.apk` に作る（debug 署名。配布しない） |
| `./gradlew lint ktlintCheck` | Android Lint と ktlint。警告が 1 件でもあれば失敗する |
| `./gradlew ktlintFormat` | ktlint の指摘を自動修正する |
| `./gradlew test` | JVM 上のユニットテスト |

push 前は `./gradlew lint ktlintCheck test` がクリーンに通ることを確認する。

## domain 層とテストベクタ

`app/src/main/kotlin/com/ousiassllc/swarmviewer/domain/` は、デスクトップ版（Go の `internal/domain`）と同じ責務の純 Kotlin（Android API・OkHttp・Room を import しない）。接続文字列と URL の検証（`parsePairing`・`validateUrl`）、`report` / `sessions_list` の解析（`parseReport`・`parseSessions`）、通知判定（`detect`）、エラー分類（`classify`）、しきい値の判定（`usageTone`）を持つ。token は `PairingPayload` だけが持ち、`toString`・例外メッセージには出さない。

挙動の一致は、リポジトリ直下の [`testdata/vectors/`](../testdata/vectors/README.md)（JSON）を Go と Kotlin の両方が読んで保つ。ケースを足す・変えるときは、`./gradlew test` と `go test ./internal/domain/... ./internal/web/format/...` の両方が通ることを確かめる。

## data 層

`app/src/main/kotlin/com/ousiassllc/swarmviewer/data/` は domain の下で、通信・token の保護・保存を持つ（`ui` / `work` から使う）。

| パッケージ | 内容 |
|------------|------|
| `data/report` | swarm-console `/report` のクライアント（`ReportClient`）。OkHttp で MCP Streamable HTTP（JSON-RPC 2.0）の `POST` だけを送り、呼ぶツールは `sessions_list` と `report` だけ（許可リスト `ReportTools`。送信直前に `GuardInterceptor` が URL・メソッド・JSON-RPC・ツール名を再検査する）。失敗は domain の `FetchException`（`classify`）。タイムアウトは接続・応答の合計 10 秒、リトライなし |
| `data/secret` | token の保護領域（`FileTokenStore`）。Android Keystore の AES-256-GCM 鍵（エクスポート不可。`KeystoreTokenCipher`）で暗号化し、`noBackupFilesDir/` に接続 ID ごとのファイルとして保存する（EncryptedSharedPreferences は使わない）。暗号文は接続 ID と結び付く。復号できない（鍵の消失・破損）token は `null` で、「再ペアリングが必要」として扱う |
| `data/store` | Room（`SwarmDatabase`、DB 版 1）。`connections`・`settings`・`snapshots`・`notified_states`（主キー `(接続, セッション, ペイン, 状態)` が二重通知を防ぐ）。token の列は持たない。窓口は `ViewerStore`（Room 実装は `RoomViewerStore`）。`ConnectionRepository.delete` が token の削除、接続の行の削除の順に行う（`ON DELETE CASCADE` でスナップショット・通知済み状態も消える） |

- Room のスキーマ JSON は `app/schemas/` に書き出してコミットする（最初の版で、マイグレーションはない）。スキーマを変えるときは DB の版を上げて `Migration` を書き、`fallbackToDestructiveMigration` は使わない
- JVM のユニットテスト（`./gradlew test`）は、MockWebServer による client と許可リスト、`FileTokenStore`（メモリ上の鍵）、スナップショットの JSON、接続削除の順序を検査する。Keystore と Room（SQLite の制約・連鎖削除）は実機・エミュレータが要るため `app/src/androidTest/` に置き、`./gradlew connectedDebugAndroidTest`（接続済みのデバイスまたはエミュレータが必要）で実行する

## 依存の版について

- `compileSdk` は 35 に固定している（親 Issue #7 の確定事項）。このため、`minCompileSdk` が 36 以上の版（Compose BOM 2025.08.00 以降・`activity-compose` 1.11 以降・`core-ktx` 1.17 以降・OkHttp 5.4 以降）は使えない。詳細は `gradle/libs.versions.toml` の先頭コメントを参照。
- 「新しい版がある」系の lint（`GradleDependency`・`NewerVersionAvailable`・`OldTargetApi`）は、新しいコミットなしに CI が失敗するのを避けるため無効にしている。

## マニフェストの方針

- アプリが宣言する権限は `INTERNET` のみ。WorkManager など依存ライブラリが自身のマニフェストで追加する権限（`WAKE_LOCK`・`ACCESS_NETWORK_STATE` など）は、マージ後のマニフェストに加わる。
- `allowBackup="false"` に加え、`dataExtractionRules`（Android 12 以降）と `fullBackupContent`（Android 11 以前）で全データをバックアップ・端末間転送から除外する。
- cleartext（http）はアプリ全体で許可する（`res/xml/network_security_config.xml`）。tailnet の IP は範囲指定できないため。接続先 URL の検証は domain 層が担う。

## リリース署名

署名設定・keystore・パスワードはこのディレクトリに置かない。リリースビルドの署名は CI の release ジョブ（別 Issue）が担う。
