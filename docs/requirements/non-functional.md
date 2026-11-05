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
| NFR-SEC-9 | フロントのコンテンツは同梱のローカルアセットのみ。外部 URL の読み込み・リモートスクリプト・インラインスクリプトを許さず、CSP（`default-src 'self'`、`script-src 'self'`、`style-src 'self'`、`connect-src 'self'`。`unsafe-eval`・`unsafe-inline` なし）で絞る。このため HTMX は `<meta name="htmx-config" content='{"allowEval":false,"includeIndicatorStyles":false}'>` で eval とインラインスタイル注入を止め、eval を要する機能（トリガーフィルタ式、`hx-on:`・`js:` 構文）を使わない。状態変更ルートには CSRF トークンを要求する（`hx-headers` で配布）。フォントは同梱の Symbols Nerd Font（Nerd Fonts v3.5.1 の Symbols Only・Mono、woff2、`unicode-range` は私用領域 PUA だけ）を `/static/fonts/` から配り、それ以外の文字は OS のフォントで描く。CDN からは読まず、CSP の `font-src` は足さない（`default-src 'self'` が効く）。ライセンスは `THIRD_PARTY_NOTICES.md` |
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
| NFR-AND-DST-1 | 署名済みの APK を GitHub Releases で配布する（サイドロード）。Google Play は対象外。配布物は `swarm-viewer_vX.Y.Z_android.apk` で、`SHA256SUMS`（NFR-DST-1）に含める。端末側で「提供元不明のアプリ」のインストールを許可する手順を README に書く |
| NFR-AND-DST-2 | release 署名鍵（keystore）を 1 つ作り、毎回同じ鍵で署名する。同じ鍵で署名することで、インストール済みのアプリを上書きアップデートできる。鍵を失うと上書きできないため、複数の場所にバックアップする。鍵・パスワードはリポジトリにコミットしない。CI のシークレット（keystore は base64）は release ジョブだけが使う（[環境構築 §11](../environment/setup.md)） |
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
