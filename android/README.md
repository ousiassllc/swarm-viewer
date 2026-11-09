# swarm-viewer Android

swarm-viewer の Android 版（Kotlin + Jetpack Compose）。親 Issue は [#7](https://github.com/ousiassllc/swarm-viewer/issues/7)、操作モードは [#97](https://github.com/ousiassllc/swarm-viewer/issues/97)。接続の種類（`/report` = 見るだけ、`/ops` = 操作）は接続文字列の `url` の path で決まり、操作の接続だけがペインへの回答・送信・中断・起動・終了をできる（設計は [`docs/android/overview.md`](../docs/android/overview.md) §10）。

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
| `data/report` | swarm-console `/report`・`/ops` のクライアント（`ReportClient`）。OkHttp で MCP Streamable HTTP（JSON-RPC 2.0）の `POST` だけを送る。呼べるツールは接続の種類ごとの許可リスト（`ReportTools`。見るだけ（`/report`）: `sessions_list`・`report`・`pane_tail`・`ask_card`、操作（`/ops`）: それに `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list`）。見るだけの接続に操作系ツールを呼ぶと通信せずに `FetchException`（`tool_not_allowed`）。送信直前に `GuardInterceptor` が URL の path・メソッド・JSON-RPC・ツール名を再検査する（`/report` なら 4 つ、`/ops` なら 10 個）。失敗は domain の `FetchException`（`classify`。操作ツールの失敗は `operation`）。タイムアウトは接続・応答の合計 10 秒、リトライなし |
| `data/secret` | token の保護領域（`FileTokenStore`）。Android Keystore の AES-256-GCM 鍵（エクスポート不可。`KeystoreTokenCipher`）で暗号化し、`noBackupFilesDir/` に接続 ID ごとのファイルとして保存する（EncryptedSharedPreferences は使わない）。暗号文は接続 ID と結び付く。復号できない（鍵の消失・破損）token は `null` で、「再ペアリングが必要」として扱う |
| `data/store` | Room（`SwarmDatabase`、DB 版 1）。`connections`・`settings`・`snapshots`・`notified_states`（主キー `(接続, セッション, ペイン, 状態)` が二重通知を防ぐ）。token の列は持たない。窓口は `ViewerStore`（Room 実装は `RoomViewerStore`）。`ConnectionRepository.delete` が token の削除、接続の行の削除の順に行う（`ON DELETE CASCADE` でスナップショット・通知済み状態も消える） |

- Room のスキーマ JSON は `app/schemas/` に書き出してコミットする（最初の版で、マイグレーションはない）。スキーマを変えるときは DB の版を上げて `Migration` を書き、`fallbackToDestructiveMigration` は使わない
- JVM のユニットテスト（`./gradlew test`）は、MockWebServer による client と許可リスト、`FileTokenStore`（メモリ上の鍵）、スナップショットの JSON、接続削除の順序を検査する。Keystore と Room（SQLite の制約・連鎖削除）は実機・エミュレータが要るため `app/src/androidTest/` に置き、`./gradlew connectedDebugAndroidTest`（接続済みのデバイスまたはエミュレータが必要）で実行する

## service 層と画面（ペアリング・接続管理）

- `service/PairingService`: 接続文字列（貼り付け・ディープリンク・QR のどれも `parse`）を検証して下書き（token はこの中だけ。有効 5 分・同時に 1 件）を作り、`confirm` が疎通確認（`sessions_list`）に成功したときだけ接続を保存する（token は `data/secret`、接続は `data/store`）。同じ URL の再ペアリングは接続 ID・`baselined_at` を保ったまま token を差し替える。保存のあと `baselined_at` が未記録なら最初の一巡（全許可セッションを順に取得し、今ある `asking` / `stalled` を通知せず通知済みとして記録）を行い、全セッションが取れたときだけ `baselined_at` を記録する（取れなければ未記録のまま、通知側（Issue #21）の次の取得が続きを行う）
- `service/ConnectionService`: 一覧・切替・削除（token → 行の順。スナップショット・通知済み状態は外部キーで連鎖削除）と、401・403 で再ペアリングが必要な接続のメモリ上の記録
- `service/SnapshotService`: 画面更新の取得。許可セッション（`allowed: true`）の取得と、一覧から消えたセッションのスナップショットの削除、最後に選んだセッションの接続ごとの記憶、選択中セッションの `report` 1 本の取得と接続×セッションごとの最新 1 件の保存（起動直後・失敗時に「前回取得」として出す）。`service/SettingsService` は設定（画面更新間隔 5 / 10 / 30 / 60 秒）の読み書き
- `service/PaneService`: ペイン詳細の取得（`pane_tail`・`ask_card`。見るだけの接続でも）と操作（`ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list`。操作の接続だけ）。見るだけの接続は通信せずに断り、同じペインの操作は排他にし（実行中に次が来たら通信せず断る）、401・403 は `needs_repair` にする。`ui/pane`: `PaneDetailScreen`・`PaneDetailViewModel`（開いている間、画面が見えている間だけ 5 秒ごとに更新）・`StartSessionScreen`・`StartSessionViewModel`（フォルダピッカー）。操作の文言は `ui/OperationMessages.kt`（デスクトップ版と同じ）。token・送信本文・`pane_tail` の本文はログに出さない
- `ui/pairing`（貼り付け・確認・QR 読み取り）、`ui/connections`（S1）、`ui/main`（メイン画面 A2）、`ui/settings`（設定画面 A3）、`ui/navigation`（最小のナビゲーション）。ViewModel は固定文言（`ui/Messages.kt`。デスクトップ版と同じ文言）だけを画面に出し、token・完全な URL・貼り付けた文字列・例外の本文は出さない
- メイン画面: `MainViewModel` が選択中の接続・セッションと取得済みの表示を持ち（構成変更で失われない）、画面が見えている間（`LifecycleStartEffect`）だけ設定の間隔でポーリングする。前の取得が終わるまで次を出さず、バックグラウンドで取得ごと止め、復帰で即時取得する。取得失敗は直前の表示をグレーアウトして残し、401・403 は `ConnectionService.markNeedsRepair` に記録してポーリングを止め、再ペアリングへ誘導する。幅の区分（`material3-window-size-class`）が Compact ならペイン順のリスト 1 列、Medium / Expanded なら `layout` のグリッドと利用上限・ホスト負荷を横に置く。スクロール位置は `rememberScrollState`（`rememberSaveable`）で保つ。`title`・`activity`・`note` などホスト由来の文字列は常にテキストで描画する。ペインカードのタップでペイン詳細を開く。操作の UI（「＋ 始める」など）は操作の接続だけに出し、見るだけの接続には出さない
- 通知（`notify/`）: `NotificationService` が取得した `report` を `detect` に流し、通知済みの記録（`notified_states`）に**実際に挿入できた**遷移だけ通知する（主キー制約が、画面更新とバックグラウンドの二重通知を防ぐ）。新規ペアリング後の最初の一巡・通知オフの間は記録だけ。`AttentionCheck` がバックグラウンドの 1 回（全接続・全許可セッションを順次。失敗は飛ばして記録を変えない。401・403 は `RepairFlags` に記録して対象から外し、再ペアリングの通知を 1 回）、`NotificationWorker`（WorkManager の `CoroutineWorker`。常に成功で終え、リトライしない）が呼ぶ。`NotificationScheduler` が設定（通知のオン / オフ・15 / 30 / 60 分）どおりに周期ジョブ `attention-check` を登録・取り消し、`SettingsService` の保存とアプリ起動時に呼ばれる。通知チャンネルは `asking`・`stalled`・`repair`。通知のタップは `SessionLink` の Intent（接続 ID・セッション ID だけ）で、`MainActivity` が検証して `MainViewModel.openSession` に渡す。権限（`POST_NOTIFICATIONS`）は通知をオンにするとき・初回起動時に要求し、拒否されたら設定画面に案内を出す
- QR 読み取りは CameraX + ML Kit の同梱モデル版（[アーキテクチャ設計 §11](../docs/architecture/overview.md) #10）。`AppContainer` が data 層とサービスを組み立てる（Application サブクラスは持たない）

## 依存の版について

- `compileSdk` は 35 に固定している（親 Issue #7 の確定事項）。このため、`minCompileSdk` が 36 以上の版（Compose BOM 2025.08.00 以降・`activity-compose` 1.11 以降・`core-ktx` 1.17 以降・OkHttp 5.4 以降）は使えない。詳細は `gradle/libs.versions.toml` の先頭コメントを参照。
- 「新しい版がある」系の lint（`GradleDependency`・`NewerVersionAvailable`・`OldTargetApi`）は、新しいコミットなしに CI が失敗するのを避けるため無効にしている。

## マニフェストの方針

- アプリが宣言する権限は `INTERNET` と `CAMERA`（QR 読み取りを開始したときだけ実行時に要求する。`android.hardware.camera` は `required="false"`）。WorkManager など依存ライブラリが自身のマニフェストで追加する権限（`WAKE_LOCK`・`ACCESS_NETWORK_STATE` など）は、マージ後のマニフェストに加わる。
- `allowBackup="false"` に加え、`dataExtractionRules`（Android 12 以降）と `fullBackupContent`（Android 11 以前）で全データをバックアップ・端末間転送から除外する。
- `MainActivity` は `launchMode="singleTask"` で、`swarm-viewer://pair?...` の intent-filter（`VIEW`・`DEFAULT`・`BROWSABLE`）を 1 つだけ公開する。リンクは信頼できない入力として `parsePairing` で検証し、利用者が確認画面で確定するまで保存しない（処理後は Intent のデータを消す）。
- cleartext（http）はアプリ全体で許可する（`res/xml/network_security_config.xml`）。tailnet の IP は範囲指定できないため。接続先 URL の検証は domain 層が担う。

## リリース署名

署名設定・keystore・パスワードはこのディレクトリに置かない（`app/build.gradle.kts` の release に `signingConfig` はない）。`./gradlew assembleRelease` は署名なしの APK を作り、署名は release.yml の `build-android`（`.github/scripts/sign-android-apk.sh`）が `apksigner` で行う。版は `-PappVersionName=X.Y.Z -PappVersionCode=N` で渡す（タグから `.github/scripts/android-version.sh` が決める。渡さないときは `0.1.0` / `1`）。鍵の作り方・シークレット・検証手順は [環境構築 §11.3・§11.4](../docs/environment/setup.md)。

### 署名証明書の指紋

release 署名証明書の SHA-256 フィンガープリント（公開情報。[環境構築 §11.3](../docs/environment/setup.md) の「証明書の確認」）。Release の APK がこの値と一致することを、リリースのたびに確かめる（`apksigner verify --verbose --print-certs` の `Signer #1 certificate SHA-256 digest`。手順は §11.4）。

| 項目 | 値 |
|------|-----|
| SHA-256 | `E7:EB:05:8D:88:29:51:85:22:05:FC:95:E1:68:73:D9:E7:B6:E1:96:6B:11:08:4C:9A:AE:75:2C:92:EE:16:2A` |
| Alias | `swarm-viewer` |
| Owner | `CN=Yuhta Takezawa, O=Ousiass LLC` |
