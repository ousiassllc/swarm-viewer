# 非機能要件

[機能要件](functional.md)を満たすうえでの性能・セキュリティ・可用性などの制約を定める。数値目標は初版の目標値で、実機での測定後に見直す。

> §1〜§9 はデスクトップ版の要件。Android 版の要件は [§10](#10-android-版) に定める。共通の要件（NFR-SEC-1・4・5・6・7・8・10・11、NFR-PERF-4〜6、NFR-AVL-1〜3、NFR-CMP-1〜3、NFR-LOG-3・4、NFR-QLT-1〜3）はそのまま Android にも適用する。

## 1. 対応環境

| ID | 要件 |
|----|------|
| NFR-ENV-1 | Windows 11、および最新 2 世代の macOS（Intel / Apple Silicon）で動作する。それより古い OS はサポートしない |
| NFR-ENV-2 | 実装は Wails v3（Go）。フロントは HALT（Templ + HTMX + Tailwind、SSR。SPA は作らない。初版は Lit なし）。Windows は WebView2、macOS は WKWebView を使う。詳細は[アーキテクチャ設計](../architecture/overview.md) |
| NFR-ENV-3 | 接続先は swarm-console v0.1.11 以降。`sessions_list` とセッション引数（v0.1.11 で追加）に依存するため、それ未満は対象外 |
| NFR-ENV-4 | 端末が swarm-console と同一の tailnet に参加していることを前提とする。viewer 自体は Tailscale を同梱・制御しない |

## 2. 性能・リソース

| ID | 要件 | 目標値 |
|----|------|--------|
| NFR-PERF-1 | アプリ起動からウィンドウ表示まで | 3 秒以内 |
| NFR-PERF-2 | 起動から最初のペイン状態の表示まで（接続が到達可能なとき） | 2 秒以内（起動時間を含まず、取得開始からの時間） |
| NFR-PERF-3 | 常駐メモリ（通常使用時） | 200 MB 以下 |
| NFR-PERF-4 | HTTP リクエストのタイムアウト | 10 秒（接続・応答の合計） |
| NFR-PERF-5 | 画面更新ポーリングは選択中セッションの `report` 1 本のみとし、前のリクエストが終わる前に次を発行しない | — |
| NFR-PERF-6 | 通知用ポーリングは接続・セッションを**順次**取得し、並列に発行しない（ホストへの負荷を抑える） | — |
| NFR-PERF-7 | バックグラウンド（サスペンド中・最小化中）は画面更新ポーリングを止める。通知用ポーリングは通知がオンの間だけ動かす | — |

## 3. セキュリティ

| ID | 要件 |
|----|------|
| NFR-SEC-1 | 接続先は **`/report` のみ**。`/ops` および `/ops` 専用ツールを呼ぶコードパスを持たない。許可リストにあるツール以外の呼び出しはコード上できない |
| NFR-SEC-2 | token は OS 秘密領域（macOS Keychain、Windows Credential Manager）にのみ保存する。SQLite・設定ファイル・ログ・クラッシュ出力・画面・URL 履歴に平文で出さない |
| NFR-SEC-3 | HTTP 通信は Go（バックエンド）側だけで行い、token をフロント（WebView の JS）に渡さない。画面と Go の間の HTTP は Wails のアセットハンドラ経由（in-process）で、TCP ポートを開かない。フロントは接続 ID・セッション ID・ペアリング下書き ID だけを扱う |
| NFR-SEC-4 | 接続先 URL の host は tailnet 範囲（IPv4 `100.64.0.0/10`、IPv6 `fd7a:115c:a1e0::/48`）またはループバック（`127.0.0.0/8`、`::1`）の IP リテラルに限る。範囲外の URL はペアリングを拒否する。MagicDNS 名などのホスト名は受け付けない |
| NFR-SEC-5 | URL の scheme は `http`、path は `/report`、port は `48120`（ループバックのみ任意のポート）に限る。それ以外はペアリングを拒否する |
| NFR-SEC-6 | HTTP は平文だが、tailnet（WireGuard）の暗号化を前提とする。tailnet 外への送信を許すと token が平文で流れるため NFR-SEC-4 で防ぐ |
| NFR-SEC-7 | ペアリング用の接続文字列・ディープリンクは token を含む。解析後はメモリ上の値をすぐ破棄し、貼り付け欄の内容を保存・ログ出力しない。ディープリンクは OS のプロセス引数・履歴に残りうるため、README で「発行した接続文字列は再利用しない」ことを案内する |
| NFR-SEC-8 | トークン削除（切断）時は、秘密領域のエントリと、その接続に紐づく SQLite のレコードを削除する |
| NFR-SEC-9 | フロントのコンテンツは同梱のローカルアセットのみ。外部 URL の読み込み・リモートスクリプト・インラインスクリプトを許さず、CSP（`default-src 'self'`、`script-src 'self'`、`style-src 'self'`、`connect-src 'self'`。`unsafe-eval`・`unsafe-inline` なし）で絞る。このため HTMX は `<meta name="htmx-config" content='{"allowEval":false,"includeIndicatorStyles":false}'>` で eval とインラインスタイル注入を止め、eval を要する機能（トリガーフィルタ式、`hx-on:`・`js:` 構文）を使わない。状態変更ルートには CSRF トークンを要求する（`hx-headers` で配布）。フォントはすべて同梱し、`/static/fonts/` から配る（SIL OFL）。UI の本文は Sora + Noto Sans JP（ウェイトは Regular 400 と SemiBold 600 のみ。Noto Sans JP は JIS X 0208 の全文字・ASCII・半角カナのサブセットで、範囲外の字は OS のフォントで描く）、コードと桁を揃える数値は JetBrains Mono（Regular 400）、ペインタイトルの Nerd Font アイコンは Symbols Nerd Font（Nerd Fonts v3.5.1 の Symbols Only・Mono、`unicode-range` は私用領域 PUA だけ）。いずれも woff2。CDN からは読まず、CSP の `font-src` は足さない（`default-src 'self'` が効く）。ライセンスは `THIRD_PARTY_NOTICES.md` |
| NFR-SEC-10 | ツール応答の表示は常にテキストとして描画し、HTML としては解釈しない（`title`・`activity`・`note` は信頼できない入力として扱う） |
| NFR-SEC-11 | ログに token を出さない。URL は host のみを記録し、完全な URL・クエリは出さない |

## 4. 可用性・耐障害性

| ID | 要件 |
|----|------|
| NFR-AVL-1 | ホストへ届かない・認証失敗などの取得失敗でアプリを終了・クラッシュさせない。直前の表示内容を保持し、失敗の状態を併記する |
| NFR-AVL-2 | 取得失敗時は次の周期で自動再試行する（バックオフなし）。ただし 401・403 の接続は再ペアリングまで自動ポーリングを止める |
| NFR-AVL-3 | 一部のセッション・接続の失敗が、他のセッション・接続の表示や通知用ポーリングを止めない |
| NFR-AVL-4 | OS のサスペンドから復帰したら、即時に 1 回取得する |
| NFR-AVL-5 | SQLite が壊れた・開けない場合は起動を止めず、エラーを表示して設定を初期化できる導線を出す（token は秘密領域に残る） |
| NFR-AVL-6 | アプリは単一インスタンスとする。2 つ目の起動・ディープリンク受信は既存インスタンスへ転送する（通知の重複記録を避ける） |

## 5. 互換性・前方互換

| ID | 要件 |
|----|------|
| NFR-CMP-1 | 応答の未知のキーは無視する。未知の `state`・`attention[].kind` は表示を壊さずに扱う（[機能要件 §3・§7](functional.md)） |
| NFR-CMP-2 | 必須でないフィールドの欠落（`null`・キーなし）で例外にしない。`available: false` の枠は常に許容する |
| NFR-CMP-3 | ペアリングペイロードは `v` を持つ。未知の `v` は「アプリを更新してください」と表示して拒否する |
| NFR-CMP-4 | SQLite スキーマはマイグレーションで版管理する。新しい版のアプリで上げたスキーマを古い版が開いたときは、データを壊さず起動を拒否して案内する |

## 6. ストレージ

| ID | 要件 |
|----|------|
| NFR-DAT-1 | アプリ内部の永続データは SQLite の 1 ファイル（OS 標準のアプリデータ領域）に保存する。token は含めない |
| NFR-DAT-2 | SQLite のドライバは CGO 不要の pure Go 実装（`modernc.org/sqlite`）を使う。理由: Windows / macOS のクロスビルドと配布を単純にするため |
| NFR-DAT-3 | 通知済み記録（[機能要件 §8.2](functional.md)）は、対応するペインが消えたことを観測した時点、または接続の削除時に削除する。通知済み記録の件数に上限（接続あたり 1,000 件）を設け、超えた場合は古いものから削除する |
| NFR-DAT-4 | 直近のスナップショットは接続×セッションごとに最新 1 件だけ保存し、履歴を持たない。接続の削除で削除する。`title`・`cwd`・`activity`・`note` などホスト側の作業内容がディスク上に平文で残るため、DB ファイルは所有者のみ読める権限（`0600`）で作成し、README に記載する |

## 7. ログ・運用

| ID | 要件 |
|----|------|
| NFR-LOG-1 | ローカルのログファイルに記録し、サイズ上限でローテーションする（上限 5 MB × 3 世代） |
| NFR-LOG-2 | 既定ではエラー・警告のみ記録する。設定画面で**デバッグログ**のオン/オフを切り替えられる（既定オフ） |
| NFR-LOG-3 | ログに token・完全な URL・ペインの本文（`activity`・`note`）を出さない。接続は表示名と host のみで識別する |
| NFR-LOG-4 | ログ・クラッシュ情報を外部へ自動送信しない |

## 8. 配布・保守

| ID | 要件 |
|----|------|
| NFR-DST-1 | Windows / macOS 向けの配布物を CI（release.yml。[環境構築 §5.5](../environment/setup.md)）が作り、GitHub Releases で配布する。形式は、Windows が NSIS インストーラ（`.exe`、amd64）、macOS が `.app` を zip にしたもの（amd64 + arm64 のユニバーサル）。Release には配布物と一緒にチェックサム `SHA256SUMS`（`sha256sum` 形式）を載せる |
| NFR-DST-2 | コード署名・公証は初版では行わない（unsigned。macOS の Gatekeeper 警告は許容し、README に回避手順を書く） |
| NFR-DST-3 | 自動更新は初版では持たない（手動入れ替え） |
| NFR-DST-4 | カスタム URI スキーム `swarm-viewer://` は、`build/config.yml` の `protocols:` で宣言し、インストーラ（NSIS）／`.app`（`Info.plist`）によって OS へ登録する。インストールしていない実行ファイル単体ではディープリンクは使えず、貼り付けでペアリングする |
| NFR-DST-5 | UI の言語は日本語のみ。`report` の `lang` は指定せず、言語に依存しない `format: "json"` を使う |

## 9. 品質（テスト）

| ID | 要件 |
|----|------|
| NFR-QLT-1 | swarm-console の API 応答（`report`・`sessions_list`・エラー各種）のサンプルを固定し、パーサ・状態遷移検知・重複通知抑止をユニットテストで検証する |
| NFR-QLT-2 | URL 検証（NFR-SEC-4・5）は、tailnet 範囲の境界値、ホスト名、`0.0.0.0`、`http` 以外の scheme を含むテストを持つ |
| NFR-QLT-3 | ポーリングのテストは固定時間の待ちに頼らず、フェイククロックまたは条件待ちで行う |

## 10. Android 版

機能は[機能要件 §12](functional.md)、設計は[Android 版の設計](../android/overview.md)・[アーキテクチャ設計 §12](../architecture/overview.md)、保存は [ER §7](../architecture/er.md) を参照する。数値目標は初版の目標値で、実機（Galaxy Fold7）での測定後に見直す。

Android に**適用しない**デスクトップ版の要件: NFR-ENV-1・NFR-ENV-2（OS・Wails）、NFR-SEC-2・NFR-SEC-3・NFR-SEC-9（OS 秘密領域・WebView・CSP）、NFR-AVL-4・NFR-AVL-6（サスペンド復帰・単一インスタンス）、NFR-DAT-1・NFR-DAT-2（SQLite の配置・ドライバ）、NFR-LOG-1・NFR-LOG-2（ログファイル）、NFR-DST-1〜4（Windows / macOS の配布）。対応する Android の要件を下に置く。

### 10.1 対応環境

| ID | 要件 |
|----|------|
| NFR-AND-ENV-1 | 最小 Android は 10（API 29）。targetSdk・compileSdk は 35。それより古い Android はサポートしない |
| NFR-AND-ENV-2 | 対象端末はまず Galaxy Fold7（折りたたみ時・展開時の両方）。ほかの Android 10 以降の端末も WindowSizeClass の切替（FR-AND-2）で動くが、動作確認は Galaxy Fold7 のみ |
| NFR-AND-ENV-3 | 実装は Kotlin + Jetpack Compose（Material 3）。技術選定は[アーキテクチャ設計 §12](../architecture/overview.md)。Go のコードは共有しない |
| NFR-AND-ENV-4 | 接続先は swarm-console v0.1.11 以降（NFR-ENV-3 と同じ）。端末が同一の tailnet に参加している（端末の Tailscale アプリが有効）ことを前提とし、viewer は Tailscale を同梱・制御しない（NFR-ENV-4 と同じ）。Tailscale が切れているときは `unreachable` として扱う |
| NFR-AND-ENV-5 | UI の言語は日本語のみ。`report` の `lang` は指定しない（NFR-DST-5 と同じ） |

### 10.2 性能・リソース

| ID | 要件 | 目標値 |
|----|------|--------|
| NFR-AND-PERF-1 | アプリの起動（コールドスタート）からメイン画面（前回取得分を含む）の表示まで | 3 秒以内 |
| NFR-AND-PERF-2 | 起動から最初のペイン状態の表示まで（接続が到達可能なとき） | 2 秒以内（起動時間を含まず、取得開始からの時間） |
| NFR-AND-PERF-3 | 通常使用時のメモリ使用量（PSS） | 200 MB 以下 |
| NFR-AND-PERF-4 | HTTP リクエストのタイムアウト（OkHttp の呼び出し全体） | 10 秒（NFR-PERF-4 と同じ） |
| NFR-AND-PERF-5 | 画面更新は選択中セッションの `report` 1 本のみとし、前のリクエストが終わる前に次を発行しない（NFR-PERF-5）。バックグラウンドの確認は接続・セッションを**順次**取得し、並列に発行しない（NFR-PERF-6） | — |
| NFR-AND-PERF-6 | バックグラウンドの確認 1 回の実行（接続数 3・許可セッション計 10 程度まで。通信が通るとき） | 1 分以内 |
| NFR-AND-PERF-7 | 画面が見えていない間（ライフサイクルが `STARTED` を外れている間）は画面更新ポーリングを止める。通知用の周期ジョブは通知がオンの間だけ登録する | — |

### 10.3 セキュリティ

| ID | 要件 |
|----|------|
| NFR-AND-SEC-1 | cleartext（`http`）は Network Security Config でアプリ全体に許可する（Android 9 以降は既定で禁止のため。tailnet の IP は範囲指定できない見込み。[アーキテクチャ設計 §11](../architecture/overview.md) #8）。接続先の制限は URL の検証が担い、NFR-SEC-4・5 と同じ検証（tailnet 範囲またはループバックの IP リテラル、`http`、port `48120`、path `/report`）を通らない URL へは接続しない。tailnet（WireGuard）の暗号化を前提とする（NFR-SEC-6）。検証を迂回する経路（接続文字列以外から URL を受け取る）を作らない |
| NFR-AND-SEC-2 | token は Android Keystore で保護した領域にのみ保存する。Room・平文のファイル・SharedPreferences・ログ・画面・通知・Intent・バックアップに出さない（NFR-SEC-2 の Android 版）。保存方式は Keystore の AES-256-GCM 鍵による暗号化で、暗号文を `noBackupFilesDir` に置く（[アーキテクチャ設計 §11](../architecture/overview.md) #9） |
| NFR-AND-SEC-3 | アプリのデータはバックアップ・端末間転送の対象外にする（`allowBackup="false"` と `dataExtractionRules`・`fullBackupContent` による全データの除外） |
| NFR-AND-SEC-4 | 接続文字列は貼り付け・ディープリンク（Intent）・QR のどれで受けても、同じ検証（NFR-SEC-4・5・7、NFR-CMP-3）を通し、確認画面で利用者が確定するまで保存しない。確認画面に token を出さない。解析後はメモリ上の値をすぐ破棄し、受け取った文字列を保存・ログ出力しない。ディープリンクは他のアプリからも送れる信頼できない入力として扱い、処理後は Intent のデータを消す |
| NFR-AND-SEC-5 | 呼ぶツールは `sessions_list` と `report` だけ（NFR-SEC-1）。`/ops` と `tools/list` のコードパスを持たない。許可リストは定数で、それ以外のツール名は呼べない |
| NFR-AND-SEC-6 | アプリが自分で宣言する権限は `INTERNET`・`POST_NOTIFICATIONS`・`CAMERA` の 3 つまで。`POST_NOTIFICATIONS` は初回に、`CAMERA` は QR の読み取りを開始したときだけ要求する（QR の読み取り方式がカメラ権限を要さないものに決まれば `CAMERA` は宣言しない。[アーキテクチャ設計 §11](../architecture/overview.md) #10）。ほかの権限を要求しない。依存ライブラリがマニフェストのマージで加える権限（WorkManager の `WAKE_LOCK`・`ACCESS_NETWORK_STATE` など）は、マージ後のマニフェストを確認して許容する |
| NFR-AND-SEC-7 | 外部に公開するコンポーネントは `MainActivity`（起動と `swarm-viewer` スキームの intent-filter）だけ。通知の `PendingIntent` は `FLAG_IMMUTABLE` とし、Intent に載せるのは接続 ID・セッション ID だけ |
| NFR-AND-SEC-8 | ツール応答の表示は常にテキストとして描画し、解釈しない（`title`・`activity`・`note` は信頼できない入力。NFR-SEC-10 と同じ） |

### 10.4 可用性・耐障害性

| ID | 要件 |
|----|------|
| NFR-AND-AVL-1 | 取得失敗でアプリをクラッシュさせない。直前の表示を保持して失敗の状態を併記し、一部の接続・セッションの失敗が他の表示・確認を止めない（NFR-AVL-1〜3 と同じ）。401・403 の接続は再ペアリングまで自動の取得（画面・通知）を止める（NFR-AVL-2） |
| NFR-AND-AVL-2 | OS がプロセスを終了しても、再起動時に Room の前回スナップショットを表示し、通知済み状態を保つ（同じ状態を再通知しない） |
| NFR-AND-AVL-3 | 端末を再起動しても、通知がオンなら周期ジョブが再開する（FR-AND-6） |
| NFR-AND-AVL-4 | バックグラウンドの通知は、Doze・アプリスタンバイ・バッテリー最適化で、設定した間隔より遅れうる（best effort）。時刻の保証はしない。設定画面に明記する（FR-AND-7） |
| NFR-AND-AVL-5 | Room の DB が開けない・新しい版で作られたときは、起動を止めずエラーを表示し、設定を初期化できる導線を出す（NFR-AVL-5・NFR-CMP-4 と同じ。token は保護領域に残る） |

### 10.5 ストレージ・ログ

| ID | 要件 |
|----|------|
| NFR-AND-DAT-1 | 内部の永続データは Room の 1 ファイル（アプリ専用の内部ストレージ）に保存する。token は含めない（[ER §7](../architecture/er.md)） |
| NFR-AND-DAT-2 | 通知済み記録は接続あたり 1,000 件を上限に古いものから削除する（NFR-DAT-3）。スナップショットは接続×セッションごとに最新 1 件だけで、接続の削除で消える（NFR-DAT-4） |
| NFR-AND-DAT-3 | スキーマは Room の `Migration` で版管理し、破壊的な再作成（`fallbackToDestructiveMigration`）を使わない |
| NFR-AND-LOG-1 | ログは Logcat のみ。ファイルへは書かず、外部へ自動送信しない（NFR-LOG-4） |
| NFR-AND-LOG-2 | 既定では警告・エラーだけ記録する。設定画面の「デバッグログ」で詳細のオン/オフを切り替える（既定オフ）。どちらも token・完全な URL・ペインの本文（`activity`・`note`）を出さない。接続は表示名と host だけで識別する（NFR-LOG-3・NFR-SEC-11） |

### 10.6 配布・署名

| ID | 要件 |
|----|------|
| NFR-AND-DST-1 | 署名済みの APK を GitHub Releases で配布する（サイドロード）。Google Play は対象外。配布物は `swarm-viewer_vX.Y.Z_android_universal.apk` で、`SHA256SUMS`（NFR-DST-1）に含める。端末側で「提供元不明のアプリ」のインストールを許可する手順を README に書く |
| NFR-AND-DST-2 | release 署名鍵（keystore）を 1 つ作り、毎回同じ鍵で署名する。同じ鍵で署名することで、インストール済みのアプリを上書きアップデートできる。鍵を失うと上書きできないため、複数の場所にバックアップする。鍵・パスワードはリポジトリにコミットしない。CI のシークレット（keystore は base64）は release.yml の署名ジョブ（`build-android`）の署名の step だけが使う（[環境構築 §11](../environment/setup.md)） |
| NFR-AND-DST-3 | **debug 署名の APK は配布しない**。配布物は `apksigner` で署名し、`apksigner verify` で検証してから Release に載せる |
| NFR-AND-DST-4 | 版の唯一の元はデスクトップ版と同じくタグ（`vX.Y.Z`）。`versionName` はタグの `X.Y.Z`、`versionCode` は `X × 1,000,000 + Y × 1,000 + Z`（単調増加。上書きアップデートに必要） |
| NFR-AND-DST-5 | 自動更新（アプリ内アップデート）は持たない（手動で APK を入れ替える。NFR-DST-3 と同じ） |
| NFR-AND-DST-6 | カスタム URI スキーム `swarm-viewer://` は AndroidManifest の intent-filter で宣言する（[API 仕様 §2.3](../api/endpoints.md)）。OS への登録はインストールで済み、デスクトップ版のようなインストーラの前提はない |

### 10.7 品質（テスト）

| ID | 要件 |
|----|------|
| NFR-AND-QLT-1 | domain（`parsePairing`・`validateUrl`・`parseReport`・`parseSessions`・`detect`・`classify`・`usageTone`）は、デスクトップ（Go）と**同じテストベクタ**（`testdata/vectors/*.json`）を読む JUnit テストで検証する。ベクタのケース数の下限を検査し、ファイルが空・読めないときに静かに空振りしない |
| NFR-AND-QLT-2 | token が `toString`・ログ・例外メッセージに出ないことをテストで検証する |
| NFR-AND-QLT-3 | `./gradlew lint ktlintCheck test` がクリーンに通る（Android Lint は警告もエラー扱い）。ポーリング・ワーカーのテストは固定時間の待ちに頼らない（NFR-QLT-3） |
| NFR-AND-QLT-4 | 実機でしか確認できない項目（cleartext・Keystore・QR・Doze・Tailscale・折りたたみ・再起動後の再開）は、[アーキテクチャ設計 §11](../architecture/overview.md) #8〜#15 に未確認として残し、確認後に確定する |

## 11. Ousiass デザインガイド準拠

Web UI と Android UI は Ousiass 共通デザインガイド（[brand-guide](https://ousiassllc.github.io/brand-guide/)、確認日 2026-11-05）に従う。ガイドと食い違ったらガイドが正で、下の逸脱だけを承認済みとする。

| ID | 要件 |
|----|------|
| NFR-DSG-1 | 色はガイドのトークン（`--ink` `--bone` `--mute` `--surface` `--border` `--accent` `--accent-soft` `--focus` と状態色 `--success` `--warning` `--danger` `--info`）だけを使う。Web は `static/src/css/app.css` に定義し、Tailwind の既定パレットは無効にする。Android は `ui/theme/` の ColorScheme と状態色で持つ。独自の色・ハードコードの色を足さない |
| NFR-DSG-2 | ライト / ダークはシステムの設定に追従する（Web は `prefers-color-scheme`、Android はシステムテーマ）。デスクトップのウィンドウ背景（WebView の描画前・リサイズ中に見える面）も OS のテーマの `--bone`（ライト `#F5F5F4`・ダーク `#050505`）にし、テーマ変更に追従する（値は `internal/app/theme` に 1 か所。Android は `windowBackground`）。手動の切替 UI は持たない |
| NFR-DSG-3 | フォントは NFR-SEC-9 のとおり同梱する。SemiBold は見出しとボタンだけに使い、Medium・Bold を使わない。本文は 14px 以上（説明・警告・エラーの文章を 12px にしない）。入力欄・select・textarea の文字は 16px（`atoms.FieldClass` の `text-base`）。Android のボタン（`labelLarge`）も標準の高さ 40dp に合わせて 16sp。12px はバッジ・日時・ペイン番号などのキャプションだけ。コード・桁を揃える数値・接続先の host（IP リテラル）・ホスト由来の `usage.reason` は JetBrains Mono（Web は `font-mono`、Android は `MonoFontFamily`。ペアリング確認の host は「接続先: 」のラベルと値を分けて値だけ Mono にする） |
| NFR-DSG-4 | 主操作・リンク・選択中タブはインク色（`--accent`）で、青などのアクセント色を使わない。`working` と `report_waiting` は無彩色で、状態はラベルで区別する。フォーカスは `--focus` の 2px（offset 2）、入力欄とチェックボックスは `--focus` の 40% のリング（`--ring`。入力欄 2px・チェックボックス 3px） |
| NFR-DSG-5 | トーストは画面下中央・端から 24px・高さ 40・ピル・1 行の短文・3 秒で消え・同時に 1 つ。エラー（削除失敗など）はトーストにせず、インラインアラートで示す |
| NFR-DSG-6 | モーションはガイドのトークン（トースト・アラートの出現 320ms ease-out、モーダルの出現（本体とスクラム）は slow 帯の 280ms ease-out、退出 240ms ease-in）。`prefers-reduced-motion: reduce` ではフェードを付けない（スピナーの回転は処理中表示のため維持する。周期はガイドの 0.9s）。トースト・インラインアラートは出現でフェードイン、トースト・インラインアラート・モーダル（スクラムを含む）は退出でフェードアウトする。応答の差し替えで消えるアラートとモーダルは、フェードアウトを終えてから差し替える。ボタンの押下は小さなリップル（280ms ease-out）。ボタン・リンク・タブ・接続切替のホバーは色・背景・枠・下線の色だけを `--motion-base`（180ms）・ease-out で遷移させる（`.hover-fade`。reduced-motion では遷移を止め、色の変化は残す）。Android は Compose の既定のアニメーション（ボタンのリップルを含む）に任せ、独自のモーションは持たない |
| NFR-DSG-7 | 状態は色だけでなくアイコン（Iconoir）と文言で伝える。インラインアラート（エラー・警告・お知らせ・成功）は Web の `atoms.Alert`、Android の `AppAlert` に統一する。形はガイドの見本どおり、surface 地・1px の border・左 3px の状態色・角丸 `--r-md`・文字は `--ink`（状態色はアイコンと左の帯だけ）。状態を示すバッジ（ペイン状態・要対応・上限到達・表示中）は、ガイドのステータスピルの見本（`● 完了`）どおり、文言の前に文字と同じ色の 6px の点を置く（Web は `BadgeProps.Marker`・Android は `Badge(marker = true)`）。エージェント名のように状態でないバッジには点を付けない。接続管理で「いま表示している接続」の印は、Web・Android とも「表示中」のバッジ（Accent）にそろえる。`--mute` はライト `#6b6b6e`・ダーク `#8b8b8e`（ガイド C6。bone・surface 上で AA 4.5:1）で、補助テキスト・キャプションと、中立のバッジ（エージェント名・shell / unknown。ガイドの `.pill.neu`）の文字・枠・地に使う。それ以外の情報の文字（値・文章）には使わない。カメラ映像の上に置く文言（Android の QR 読み取り）は、映像の明るさに左右されないよう ink の帯（`inverseSurface` 地・`inverseOnSurface` の文字・角丸 `--r-md`・内側 16dp）に載せる |
| NFR-DSG-8 | 確認・破壊的操作（接続の削除・保存データの初期化・接続の追加の確認）は中央モーダルで取る。Web はネイティブ `<dialog>`（`--r-xl`・`--shadow-modal`・スクラムは外側クリックで閉じてフェードする）、Android は `AppDialog`（`BasicAlertDialog` に面・1dp の border・`extraLarge`・1dp の影を組んだもの。Material の `AlertDialog` は枠も影もないので使わない）。構造はガイドの見本（`.modal`）のとおり、ヘッダー（見出しと閉じる × のアイコンボタン。Web は `atoms.ModalHeader` の `data-modal-close`、Android は `AppDialog` のタイトル行で、どちらも外側クリック・Esc・戻ると同じ「閉じる」要求）・本文・フッター（右寄せ・ボタン間 8）を 1px の border で区切り、幅は最大 420。`window.confirm`・全画面ページでの確認を使わない。確定の危険な操作のボタンは `--danger`（Web は `ButtonDanger`、Android は `AppDangerButton`）で、どちらも透明地・文字は danger・枠は danger の 45%（無効は 0.4 倍）の枠付き。削除確認のキャンセルは `AppOutlinedButton`（サブ。1px の border） |
| NFR-DSG-9 | 部品は M3・OS の既定ではなく、ガイドの見本の形に揃える。select は OS の矢印を消し（`appearance-none`）、`--mute` のシェブロンを `linear-gradient` 2 本で右端に描く（`atoms.SelectClass`。CSP のため `data:` URI は使わない）。Android の使用率バー（`ToneBar`）は M3 の「塗りとトラックの隙間」と終端の点を消し（`gapSize = 0.dp`・`drawStopIndicator = {}`）、連続したトラックの上に塗りが載る形にする。Web の使用率バーは塗り（`::-webkit-progress-value`・`::-moz-progress-bar`）も `rounded-full` で両端を丸める。セッションタブは上下 10px・左右 0・タブ間 24px で、選択中は 2px の `--accent` の下線、未選択は `--mute`（ホバーで `--ink`）の文字だけにし、角丸とホバーの面を付けない。Android の引っ張って更新のインジケーターは M3 の既定（影つき）でなく、影なし・`surface` 地・1dp の `outline` 枠の円に円形進捗（ガイドのスピナーと同じ 20dp・2dp、先端 `primary`・トラック `surfaceVariant`（accent-soft））を置く自前の部品（`AppPullToRefreshIndicator`）にする。Android のスイッチ（`AppSwitch`）は 36×20dp・つまみ 14dp、ラジオ（`AppRadio`）は 18dp の円・1dp の `--mute` 枠で選択時は `primary` の塗りに 6dp の `background` の点にし、M3 の `Switch`・`RadioButton` は使わない。テキストリンク（Web の `navLink`・`repairLink`）は共通の `.text-link`（下線 `--mute`・位置 3px・ホバーは下線を `--ink` にするだけで面は付けない）にする |
| NFR-DSG-10 | ガターはモバイル 16・`md` 以上 24（Web は `layout/base.templ` の `p-4 md:p-6`）。Android は幅の区分が Compact なら 16dp、Medium 以上（600dp 以上。メインの 2 列表示の切替と同じ境目）なら 24dp で、`MainActivity` が `LocalGutter`（`gutterFor`）に入れ、メイン（セッションタブの `edgePadding` を含む）・接続管理・ペアリング・QR 読み取り（権限の案内とカメラ上の文言・ボタン）・設定が使う（Web の `md` 768px との差は、メインの 2 列表示の境目と揃えるため許容する）。枠を持つ独立した部品（アラート）を縦に並べる間隔は 8（Web は `gap-2`・Android は `spacedBy(8.dp)`）。部品内（利用上限のアカウント名→枠の行・接続カードの名前と host 以外の並び）も 8、2 行リストのタイトル→キャプションは 2（`--list-title-caption-gap`。Web は `gap-(--list-title-caption-gap)`・Android は `spacedBy(2.dp)`）。設定行（Android のラジオ・スイッチの行）は最小の高さ 48（`--list-row-h`。`ListRowMinHeight`）。テキストリスト（ペインのメモ）は項目間 8（`--list-gap`）・記号→文字 10（`--list-marker-gap`）・記号は 5px の `--mute` の点（`--list-marker-size`）で、Web は `flex` と `before:` の円、Android は `NoteItem` の円。角丸は `--r-sm`〜`--r-xl` と `--r-full`（9999px）を `app.css` の `:root` に持ち、`@theme` の `--radius-*` 経由で使う（`.ripple` も `var(--r-full)`） |
| NFR-DSG-11 | モーションの時間とトーストの寸法は CSS のトークン（`app.css` の `:root`: `--motion-fade-out`・`--toast-duration` 3s・`--toast-h` 40px・`--toast-offset`）が唯一の出どころで、`app.js` は数値を持たず `getComputedStyle` で読む（`cssDurationMs`。トーストの 3 秒と、応答の差し替えを退出フェードの終わりまで待つ時間）。`toast.templ` の高さも `h-(--toast-h)`。トークンだけを変えても CSS のフェードと JS の待ちがずれない。アクション付きトーストの 5 秒（`--toast-duration-action`）は、アクション付きのトーストを入れるときに足す |
| NFR-DSG-12 | ボタンの左右 padding は 18（小は 12。Web は `px-[18px]`、Android は `AppButton`・`AppOutlinedButton`・`AppDangerButton` の `ButtonContentPadding`。上下は高さ 40 が決める）。チェックボックス・ラジオ→文言は 10（`--list-marker-gap`。Web は `atoms.CheckboxRowClass`。Android のラジオは `AppRadio` が円の周囲に 3dp の余白を持つので、文言の前に足す余白は 10 − 3 = 7dp の `RadioLabelStart`）。空状態の padding は縦 48・横 24 で、画面幅で変えない（Web は `px-6 py-12`、Android は `EmptyStateHorizontalPadding`・`EmptyStateVerticalPadding`） |
| NFR-DSG-13 | トーストの出現・退出の移動量は 8px（ガイド M5「下から 8px」）。`motion.css` の `fade-in`・`fade-out` は移動量を `--fade-rise`（既定 4px）で読み、`toast.templ` の本体が `[--fade-rise:8px]` を付ける（アラート・モーダルは既定の 4px） |
| NFR-DSG-14 | 検証に失敗した入力欄は `--danger` の枠で示す（ガイド O7 の `.fld.bad`。文言は入力欄の下のインラインアラート）。Web はペアリングの textarea に `atoms.FieldInvalidClass`（枠 `--danger`・フォーカスのリング `--ring-danger` = `--danger` の 40%）を重ね、Android は `AppTextField(isError = …)` で枠・caret・ラベルを `ExtraColors.danger` にする。トーストの幅は `min(92vw, 420px)` で頭打ち（`toast.templ` の `max-w-[min(92vw,26.25rem)]`。接続名など利用者が決める長い文言でも端まで伸びない） |
| NFR-DSG-15 | 入力欄はガイドの見本（`.lbl`＋`.fld`）の形にする。ラベルは入力の上（間隔 6）、入力は高さ 40・内側 12・角丸 `--r-md`・1px の枠・surface の地。Android の `AppTextField` は M3 の枠内に浮くラベル・高さ 56dp の `OutlinedTextField` を使わず、`BasicTextField` と `OutlinedTextFieldDefaults.DecorationBox`（内側 左右 12dp・上下 8dp・本文 16sp の行高 24dp で 40dp）に、ラベルを別の `Text`（`bodyMedium`）で上に置く。読み上げは入力欄の `contentDescription` にラベルを渡す。アコーディオン（`<details>`）は、単独のものにガイドの `.acc`（`acc` クラス: 枠 1px `--border`・角丸 `--r-lg`・surface の地・summary の padding 12px 16px、本文は `acc-body`（padding 0 16px 14px）で包み文字色は `--mute`）を付ける。アラートの中の `<details>`（取得失敗の詳細）は二重枠を避けるため `acc` を付けない。値の `<code>`（`usage.reason`・エラーコード）は ink のまま |

承認済みの逸脱（ガイドの値と異なるが、プロダクト固有の理由で維持するもの）:

| 箇所 | 逸脱 | 理由 |
|------|------|------|
| 要対応ペインの強調（`ring-warning`、Android は 2dp の warning 枠） | ガイドに部品がない | 状態色（`--warning`）だけで組み、ガイドの追加提案を別途出す |
| 古いデータのグレーアウト（`opacity-50 grayscale`、Android は `alpha`） | ガイドに規定がない | 状態を示す表示で、色だけに頼らず文言（「古いデータ」）を併記する。`opacity` は入れ子で掛け算になるので、かけるのは 1 回だけ（Web は取得失敗の領域 `#snapshot` に 1 回。`usage.status` が stale のパネルは、領域がグレーアウトされていないときだけ単体でかける） |
| 使用率バーのしきい値色（`warning` / `danger`） | ガイドの進捗は `--accent` 1 色 | 80% / 95% の警告を色で示す。数値と文言を併記する |
| Nerd Font の PUA 用フォント | ガイドに「ユーザーデータ内の記号」の規定がない | ペインタイトルのアイコン（UI アイコンではない）。UI アイコンは Iconoir |
| 状態色バッジのコントラスト（ライトの warning 文字は背景上 4:1 前後） | AA（4.5:1）未満 | ガイドのトークン値のまま使う。文言と併記しており色だけに頼らない。値の見直しはガイド側の課題 |
| 画面のセクション間（`layout/base.templ` の `gap-4` = 16px、Android も 12〜16dp） | ガイドのセクション間は 48〜72px | 1 画面にホスト負荷・利用上限・ペインを並べるダッシュボードで、48px にすると 1 画面に収まらず状態を見渡せない。ガターはガイドどおり（モバイル 16・`md` 以上 24）にする |
| 1 文で足りるインラインアラート（フォーム・操作のエラー、警告帯。見出しなしで本文 1 文） | ガイドのインラインアラートは「アイコン＋見出し＋本文」 | 固定文言の 1 文に、同じ内容の見出しを重ねても情報が増えない。取得失敗のバナーのように「種別＋詳しい内容（前回取得・次回再試行）」を持つものだけ、見出し（`Title`）と本文に分ける |
| Android の `DropdownMenu` / `ScrollableTabRow` / `TopAppBar`（Material 部品） | ガイドのメニュー・タブ・ナビの仕様と形が違う | 色・文字・角丸・余白はトークンに揃え、形の差は案件ガイドの課題とする |

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|----|------|---------|---------|
| 1.0 | 2026-11-03 | 新規作成 | 初版 |
| 1.1 | 2026-11-03 | フロントを HALT・Wails v3 に更新（NFR-ENV-2、NFR-SEC-3・9、NFR-DST-1・4） | アーキテクチャ設計との整合。URL スキーム登録がインストーラ前提のため配布形態を明記 |
| 1.2 | 2026-11-03 | NFR-DAT-4（スナップショットの保存方針）を追加 | ER 設計（`snapshots`）との整合 |
| 1.3 | 2026-11-03 | NFR-AVL-2 に 401・403 の例外を追加 | API 仕様との整合 |
| 1.4 | 2026-11-03 | NFR-SEC-9 に `unsafe-eval` 禁止と htmx-config、eval を要する機能の不使用を明記 | CSP と htmx のトリガーフィルタ式の不整合の解消 |
| 1.5 | 2026-11-04 | NFR-DST-1 に配布物の形式（NSIS `.exe`・`.app` の zip・ユニバーサル）と `SHA256SUMS` を明記 | Issue #3（release.yml による配布の追加） |
| 1.6 | 2026-11-05 | NFR-SEC-9 に、フォントは同梱の Symbols Nerd Font（PUA のみ）で、それ以外は OS のフォントと明記 | Issue #9（ペインタイトルの Nerd Font アイコンの表示） |
| 1.7 | 2026-11-05 | §10「Android 版」を追加（NFR-AND-*）。最小 Android 10、cleartext のアプリ全体許可と URL 検証、Keystore、バックアップ除外、署名済み APK の配布 | Issue #15（Android 版の仕様書への反映） |
| 1.8 | 2026-11-05 | NFR-SEC-9 のフォントを、ガイド指定の Sora・Noto Sans JP・JetBrains Mono の同梱（OS のフォントに頼らない）に改訂。§11「Ousiass デザインガイド準拠」を追加 | Issue #30（デザインガイド準拠。フォント同梱の承認済み） |
| 1.9 | 2026-11-05 | §11 に NFR-DSG-7（状態のアイコン併記・インラインアラートの統一・`--mute` の用途）を追加 | Issue #33（デザインガイド C4・C6） |
| 2.0 | 2026-11-05 | §11 の承認済みの逸脱に「ダッシュボードのセクション間 16px」を追加 | Issue #35（デザインガイド S5） |
| 2.1 | 2026-11-05 | NFR-DSG-6 に、トースト・アラートの出現と退出・ボタンのリップル・Android は既定に任せることを追記 | Issue #36（デザインガイド M4・M5） |
| 2.2 | 2026-11-05 | §11 に NFR-DSG-8（確認はモーダル・危険ボタン）を追加 | Issue #37（デザインガイド O1・O2・O7） |
| 2.3 | 2026-11-05 | §11 の承認済みの逸脱に「1 文で足りるインラインアラートは見出しなし」を追加 | Issue #42（デザインガイド O8） |
| 2.4 | 2026-11-05 | NFR-DSG-7 にインラインアラートの形（surface 地・左 3px の状態色・文字は ink）を明記し、承認済みの逸脱から「警告帯のコントラスト」を外した | Issue #46（デザインガイド O8） |
| 2.5 | 2026-11-05 | NFR-DSG-8 の Android の確認を `AppDialog`（枠・影つき）に改訂 | Issue #50（デザインガイド S3） |
| 2.6 | 2026-11-05 | NFR-DSG-2 にデスクトップのウィンドウ背景色、NFR-DSG-4 にチェックボックスのフォーカスリングを追記 | Issue #52（デザインガイド C3・C5） |
| 2.7 | 2026-11-05 | NFR-DSG-3 に Android のボタン 16sp を追記し、NFR-DSG-9（select のシェブロン・Android の使用率バー）を追加 | Issue #53・#56（デザインガイド T3・O8） |
| 2.8 | 2026-11-05 | NFR-DSG-6 に、モーダルの出現は slow 帯（280ms）・モーダルとアラートの退出フェード・スピナーの回転 0.9s を明記 | Issue #55（デザインガイド M1・M5・O1） |
| 2.9 | 2026-11-05 | §11 の承認済みの逸脱「古いデータのグレーアウト」に、グレーアウトは 1 回だけ（入れ子で重ねない）を明記 | Issue #57（デザインガイド C5） |
| 3.0 | 2026-11-05 | NFR-DSG-9 に、Web の使用率バーの塗りの両端・セッションタブの余白と形（下線のみ）・Android の引っ張って更新のインジケーター（影なし）を追記 | Issue #58（デザインガイド S1・S2・S3） |
| 3.1 | 2026-11-05 | NFR-DSG-6 に、ホバーの色の遷移（`.hover-fade`・180ms・reduced-motion では止める）を追記 | Issue #59（デザインガイド M1） |
| 3.2 | 2026-11-05 | NFR-DSG-9 に、Android のスイッチ・ラジオの形とテキストリンク（`.text-link`）を追記 | Issue #60（デザインガイド O7・O8） |
| 3.3 | 2026-11-05 | NFR-DSG-10（ガター・アラートの間隔・テキストリスト・角丸トークン）を追加 | Issue #61（デザインガイド S1・S2・S5） |
| 3.4 | 2026-11-05 | NFR-DSG-11（時間・トーストの寸法は CSS のトークンが唯一の出どころ）を追加 | Issue #62（デザインガイド M1） |
| 3.5 | 2026-11-05 | NFR-DSG-8 に、モーダルの構造（ヘッダーの × ・本文・フッターを線で区切る・幅 420）と危険ボタンの枠付き（Android は `AppDangerButton`）を追記 | Issue #63（デザインガイド O2・O7） |
| 3.6 | 2026-11-05 | NFR-DSG-7 に、状態バッジの点（マーカー）と、カメラ映像の上の文言を ink の帯に載せることを追記 | Issue #64（デザインガイド C4・C5） |
| 3.7 | 2026-11-05 | NFR-DSG-3 に、接続先の host を JetBrains Mono で出すことを追記 | Issue #65（デザインガイド T5） |
| 3.8 | 2026-11-05 | NFR-DSG-10 に、Android のセッションタブ・QR 読み取りがガターを使うことと、設定行の最小高さ 48 を追記 | Issue #66（デザインガイド S2・S5） |
| 3.9 | 2026-11-05 | NFR-DSG-7 に、接続管理の「表示中」の印を Web・Android ともバッジにそろえることを追記。Android のセッションタブの未選択は `onSurfaceVariant`（`--mute`）の文字（NFR-DSG-9 のとおり） | Issue #67（デザインガイド O8） |
| 4.0 | 2026-11-05 | NFR-DSG-12（ボタンの左右 padding 18・チェックボックスの行の gap 10・空状態の padding 縦 48 横 24）を追加 | Issue #72（デザインガイド S2・S4） |
| 4.1 | 2026-11-05 | NFR-DSG-13（トーストの移動量 8px）を追加 | Issue #74（デザインガイド M5） |
| 4.2 | 2026-11-05 | NFR-DSG-14（入力欄のエラー枠・トーストの最大幅 420px）を追加 | Issue #73（デザインガイド O3・O7） |
| 4.3 | 2026-11-06 | NFR-DSG-7 の `--mute` をライト `#6b6b6e`・ダーク `#8b8b8e` に分け、中立バッジの文字を `--mute` にした。要対応バーのエージェント間の線を `--border` にそろえた | Issue #85（デザインガイド C2・C4・C6） |
| 4.4 | 2026-11-06 | NFR-DSG-12 に、Android のラジオの記号→文言も 10（円の余白 3dp を差し引いた 7dp を文言の前に置く）を追記 | Issue #87（デザインガイド S2） |
| 4.5 | 2026-11-06 | NFR-DSG-15（入力欄のラベル位置・高さ 40、アコーディオンの枠・本文色）を追加 | Issue #86（デザインガイド O8） |
| 4.6 | 2026-11-06 | NFR-DSG-9 に、Android の引っ張って更新の円形進捗（20dp・トラック accent-soft）を追記。接続カードの host は `--mute`（`onSurfaceVariant`）の Mono | Issue #88（デザインガイド O8） |
