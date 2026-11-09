# 非機能要件

[機能要件](functional.md)を満たすうえでの性能・セキュリティ・可用性などの制約を定める。数値目標は初版の目標値で、実機での測定後に見直す。

> §1〜§9 はデスクトップ版の要件。Android 版の要件は [§10](#10-android-版) に定める。共通の要件（NFR-SEC-1・4〜8・10〜14、NFR-PERF-4〜6、NFR-AVL-1〜3、NFR-CMP-1〜3、NFR-LOG-3・4、NFR-QLT-1〜4）はそのまま Android にも適用する。操作モード（Issue #97）の要件は NFR-SEC-1・5・10〜14、NFR-PERF-8、NFR-QLT-4、NFR-DSG-16（Android は NFR-AND-SEC-5・9、NFR-AND-PERF-8、NFR-AND-QLT-5）に置く。

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
| NFR-PERF-4 | HTTP リクエストのタイムアウト（ツールごと。基準 10 秒に倍率を掛ける） | 読み取り（`sessions_list`・`report`・`pane_tail`・`ask_card`・`dir_list`）は 10 秒、`ask_pick`・`send_text`・`send_escape` は 30 秒（10 秒 × 3）、`pane_start`・`pane_close` は 50 秒（10 秒 × 5）。いずれも接続・応答の合計。操作ツールは swarm-console 側の `accept_timeout`（既定 10 秒）・`start_timeout`（既定 30 秒）に待つ余地を残すため長い（[API 仕様 §1.2](../api/endpoints.md)） |
| NFR-PERF-5 | 画面更新ポーリングは選択中セッションの `report` 1 本のみとし、前のリクエストが終わる前に次を発行しない | — |
| NFR-PERF-6 | 通知用ポーリングは接続・セッションを**順次**取得し、並列に発行しない（ホストへの負荷を抑える） | — |
| NFR-PERF-7 | バックグラウンド（サスペンド中・最小化中）は画面更新ポーリングを止める。通知用ポーリングは通知がオンの間だけ動かす | — |
| NFR-PERF-8 | ペイン詳細（[機能要件 §4.5](functional.md)）の自動更新は、画面が見えている間（ペイン詳細を開いている間）だけ 5 秒ごとに行い、前のリクエストが終わる前に次を発行しない。状態の取り直し（`report`）は既存の single-flight を使う。手動の更新は即時に行う。他の画面の更新ポーリング（NFR-PERF-5）は変えない | — |

## 3. セキュリティ

| ID | 要件 |
|----|------|
| NFR-SEC-1 | 呼べるツールは、**接続の種類ごとの許可リストにあるものだけ**。種類は接続の URL の path で決まる（見るだけ = `/report`、操作 = `/ops`）。見るだけの接続は `sessions_list`・`report`・`pane_tail`・`ask_card` の 4 つ、操作の接続はそれに `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list` を足した 10 個だけ。許可リストは種類別の定数で、送信前（呼び出し口）と送信内容（Transport）の二重で検査する。見るだけの接続が `/ops` を呼ぶコードパスは持たない（操作系ツールを呼ぶと、通信せずに `tool_not_allowed` で断る）。`dispatch`・`note_set`・`note_log`・`file_get`・`panes_status`・`host_status`・`report_read`・`tools/list` は、どの接続でも呼べない |
| NFR-SEC-2 | token は OS 秘密領域（macOS Keychain、Windows Credential Manager）にのみ保存する。SQLite・設定ファイル・ログ・クラッシュ出力・画面・URL 履歴に平文で出さない |
| NFR-SEC-3 | HTTP 通信は Go（バックエンド）側だけで行い、token をフロント（WebView の JS）に渡さない。画面と Go の間の HTTP は Wails のアセットハンドラ経由（in-process）で、TCP ポートを開かない。フロントは接続 ID・セッション ID・ペアリング下書き ID だけを扱う |
| NFR-SEC-4 | 接続先 URL の host は tailnet 範囲（IPv4 `100.64.0.0/10`、IPv6 `fd7a:115c:a1e0::/48`）またはループバック（`127.0.0.0/8`、`::1`）の IP リテラルに限る。範囲外の URL はペアリングを拒否する。MagicDNS 名などのホスト名は受け付けない |
| NFR-SEC-5 | URL の scheme は `http`、path は `/report` または `/ops`（この 2 つ以外・末尾スラッシュ・別表記・クエリは拒否）、port は `48120`（ループバックのみ任意のポート）に限る。それ以外はペアリングを拒否する |
| NFR-SEC-6 | HTTP は平文だが、tailnet（WireGuard）の暗号化を前提とする。tailnet 外への送信を許すと token が平文で流れるため NFR-SEC-4 で防ぐ |
| NFR-SEC-7 | ペアリング用の接続文字列・ディープリンクは token を含む。解析後はメモリ上の値をすぐ破棄し、貼り付け欄の内容を保存・ログ出力しない。ディープリンクは OS のプロセス引数・履歴に残りうるため、README で「発行した接続文字列は再利用しない」ことを案内する |
| NFR-SEC-8 | トークン削除（切断）時は、秘密領域のエントリと、その接続に紐づく SQLite のレコードを削除する |
| NFR-SEC-9 | フロントのコンテンツは同梱のローカルアセットのみ。外部 URL の読み込み・リモートスクリプト・インラインスクリプトを許さず、CSP（`default-src 'self'`、`script-src 'self'`、`style-src 'self'`、`connect-src 'self'`。`unsafe-eval`・`unsafe-inline` なし）で絞る。このため HTMX は `<meta name="htmx-config" content='{"allowEval":false,"includeIndicatorStyles":false}'>` で eval とインラインスタイル注入を止め、eval を要する機能（トリガーフィルタ式、`hx-on:`・`js:` 構文）を使わない。状態変更ルートには CSRF トークンを要求する（`hx-headers` で配布）。フォントはすべて同梱し、`/static/fonts/` から配る（SIL OFL）。UI の本文は Sora + Noto Sans JP（ウェイトは Regular 400 と SemiBold 600 のみ。Noto Sans JP は JIS X 0208 の全文字・ASCII・半角カナのサブセットで、範囲外の字は OS のフォントで描く）、コードと桁を揃える数値は JetBrains Mono（Regular 400）、ペインタイトルの Nerd Font アイコンは Symbols Nerd Font（Nerd Fonts v3.5.1 の Symbols Only・Mono、`unicode-range` は私用領域 PUA だけ）。いずれも woff2。Android 版も同じ方針（Symbols Only だけを同梱し、日本語は Noto Sans JP と OS のフォントに任せる）で、`res/font/` の TTF（`symbols_nerd_font_mono_regular.ttf`）を UI の書体（Sora → Noto Sans JP → Symbols Nerd Font Mono → システム）の代替にする。CDN からは読まず、CSP の `font-src` は足さない（`default-src 'self'` が効く）。ライセンスは `THIRD_PARTY_NOTICES.md` |
| NFR-SEC-10 | ツール応答の表示は常にテキストとして描画し、HTML としては解釈しない（`title`・`activity`・`note`・`pane_tail`・`ask_card` の質問文・選択肢は信頼できない入力として扱う） |
| NFR-SEC-11 | ログに token・送信本文（`send_text`・`ask_pick` の `text`）・`pane_tail` と `ask_card` の本文を出さない。URL は host のみを記録し、完全な URL・クエリは出さない。操作のログに残すのは、接続 ID・ツール名・種別・コード・ペイン位置まで |
| NFR-SEC-12 | 中断（`send_escape`）と終了（`pane_close`）は、確認ダイアログを経ないと送れない。アプリ側も、確認済みのフォーム値（デスクトップは `confirm=true`）が無ければ `send_escape`・`pane_close` を呼ばない（`confirm_required`。`confirm: true` を無条件に付けない）。デスクトップは `POST P/close`・`POST P/start` で、送る前にサーバー側でも最新の `report` のペイン状態を再検査する（終了は `idle`・`shell`、起動は `shell`。違えば通信せず `invalid_state`）。質問カードの選択肢（`ask_pick`）は Issue #97 の決定（2026-11-08）で確認を挟まず、押したらすぐ送る。その代わり、送信中は同じペインの全ボタンを無効にし、アプリ側（service）も同じペインへの操作を排他にして二重送信を防ぐ |
| NFR-SEC-13 | 操作の接続の token（ops スコープ）は、ペインへの送信・中断・起動・終了ができる。保存・表示・ログの扱いは report の token と同じ（NFR-SEC-2・3・7・11）。加えて、(a) ペアリング確認画面で、操作の接続のときだけ警告を出す（「この接続はペインに送信・中断・起動・終了できます」。token の漏えい・失効の案内を添える）、(b) token が漏れた・端末を紛失したときは、ホストで `swarm-console token revoke <名前>` を実行して失効する（名前は `swarm-console pair --scope ops` が `viewer-ops-<ホスト名>-<yyyymmdd>` で付ける。[API 仕様 §3](../api/endpoints.md)）。失効した token は 401 になり、viewer はその接続を `needs_repair` にして再ペアリングへ誘導する。viewer 自身は token を失効させない。README にも書く |
| NFR-SEC-14 | 見るだけの接続は、`/ops` を呼ぶコードパスを通らず、操作の UI（選択肢のボタン・送信欄・中断・終了・「＋ 始める」）を描画しない。デスクトップの操作系ルートを見るだけの接続に対して直接呼んでも、コンソールに通信せずに 403 と固定文言を返す。ペイン詳細・質問の閲覧（`pane_tail`・`ask_card`）は見るだけの接続でも出す |

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
| NFR-LOG-3 | ログに token・完全な URL・ペインの本文（`activity`・`note`・`pane_tail`・`ask_card`）・送信本文を出さない。接続は表示名と host のみで識別する |
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
| NFR-QLT-2 | URL 検証（NFR-SEC-4・5）は、tailnet 範囲の境界値、ホスト名、`0.0.0.0`、`http` 以外の scheme、path（`/report`・`/ops` ちょうどだけ通し、末尾スラッシュ・大文字小文字違い・エンコード別表記・クエリは拒否）を含むテストを持つ |
| NFR-QLT-3 | ポーリングのテストは固定時間の待ちに頼らず、フェイククロックまたは条件待ちで行う |
| NFR-QLT-4 | 操作モードは次の 6 項目を自動テストで検証する（Android は NFR-AND-QLT-5 で同じ内容）。(1) 見るだけの接続: ペイン詳細に操作部品（`pick`・`send`・`escape`・`close`・`start` のフォーム・ボタン・確認ダイアログ）が出ない。`pane_tail`・`ask_card` は呼ばれる。偽の swarm-console への `/ops` のリクエストは 0 件で、操作ルートの直接呼び出しは 403 かつコンソールへの通信 0 件。(2) 操作の接続: `ask_pick`（`n`・`text`）・`send_text`・`send_escape`（`confirm: true`。確認なしは呼ばれない）・`dir_list` → `pane_start`・`pane_close` が、期待どおりのツール名・引数（`session` 明示、`/ops` path、Bearer）で届く。(3) 許可リスト: 種類別の定数、見るだけでの操作系ツールの通信なしの拒否、送信内容の検査（`/report` に操作系ツールを出せない）、`ValidateURL` の `/ops`。(4) 排他: 同じペインの操作が同時に 2 つ走らない（2 つ目は通信なしで拒否）。(5) ログに token・送信本文・`pane_tail` の本文が出ない。(6) 結果の解析と固定文言（`ok: false`・`accepted: false`・`card_closed`・エラーコード） |

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
| NFR-AND-PERF-4 | HTTP リクエストのタイムアウト（OkHttp の呼び出し全体。ツールごと） | NFR-PERF-4 と同じ（読み取り 10 秒、`ask_pick`・`send_text`・`send_escape` 30 秒、`pane_start`・`pane_close` 50 秒。`ReportTools.timeoutScale`） |
| NFR-AND-PERF-5 | 画面更新は選択中セッションの `report` 1 本のみとし、前のリクエストが終わる前に次を発行しない（NFR-PERF-5）。バックグラウンドの確認は接続・セッションを**順次**取得し、並列に発行しない（NFR-PERF-6） | — |
| NFR-AND-PERF-6 | バックグラウンドの確認 1 回の実行（接続数 3・許可セッション計 10 程度まで。通信が通るとき） | 1 分以内 |
| NFR-AND-PERF-7 | 画面が見えていない間（ライフサイクルが `STARTED` を外れている間）は画面更新ポーリングを止める。通知用の周期ジョブは通知がオンの間だけ登録する | — |
| NFR-AND-PERF-8 | ペイン詳細の自動更新は、画面が見えている間（`STARTED` 以上。`LifecycleStartEffect`）だけ 5 秒ごとに行い、バックグラウンドでは止める。前のリクエストが終わる前に次を発行しない（NFR-PERF-8） | — |

### 10.3 セキュリティ

| ID | 要件 |
|----|------|
| NFR-AND-SEC-1 | cleartext（`http`）は Network Security Config でアプリ全体に許可する（Android 9 以降は既定で禁止のため。tailnet の IP は範囲指定できない見込み。[アーキテクチャ設計 §11](../architecture/overview.md) #8）。接続先の制限は URL の検証が担い、NFR-SEC-4・5 と同じ検証（tailnet 範囲またはループバックの IP リテラル、`http`、port `48120`、path `/report` または `/ops`）を通らない URL へは接続しない。tailnet（WireGuard）の暗号化を前提とする（NFR-SEC-6）。検証を迂回する経路（接続文字列以外から URL を受け取る）を作らない |
| NFR-AND-SEC-2 | token は Android Keystore で保護した領域にのみ保存する。Room・平文のファイル・SharedPreferences・ログ・画面・通知・Intent・バックアップに出さない（NFR-SEC-2 の Android 版）。保存方式は Keystore の AES-256-GCM 鍵による暗号化で、暗号文を `noBackupFilesDir` に置く（[アーキテクチャ設計 §11](../architecture/overview.md) #9） |
| NFR-AND-SEC-3 | アプリのデータはバックアップ・端末間転送の対象外にする（`allowBackup="false"` と `dataExtractionRules`・`fullBackupContent` による全データの除外） |
| NFR-AND-SEC-4 | 接続文字列は貼り付け・ディープリンク（Intent）・QR のどれで受けても、同じ検証（NFR-SEC-4・5・7、NFR-CMP-3）を通し、確認画面で利用者が確定するまで保存しない。確認画面に token を出さない。解析後はメモリ上の値をすぐ破棄し、受け取った文字列を保存・ログ出力しない。ディープリンクは他のアプリからも送れる信頼できない入力として扱い、処理後は Intent のデータを消す |
| NFR-AND-SEC-5 | 呼ぶツールは、接続の種類ごとの許可リスト（NFR-SEC-1）にあるものだけ。見るだけ（`/report`）は `sessions_list`・`report`・`pane_tail`・`ask_card`、操作（`/ops`）はそれに `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list` を足した 10 個。許可リストは `data/report` の種類別の定数で、送信前（呼び出し口）と送信内容（`GuardInterceptor`。URL の path が `/report` なら 4 つ、`/ops` なら 10 個）の二重で検査する。見るだけの接続に操作系ツールを呼ぶと、通信せずに `tool_not_allowed`（`FetchException`）で断る。`tools/list` のコードパスを持たない。許可リスト外のツール名は呼べない |
| NFR-AND-SEC-6 | アプリが自分で宣言する権限は `INTERNET`・`POST_NOTIFICATIONS`・`CAMERA` の 3 つまで。`POST_NOTIFICATIONS` は初回に、`CAMERA` は QR の読み取りを開始したときだけ要求する（QR の読み取り方式がカメラ権限を要さないものに決まれば `CAMERA` は宣言しない。[アーキテクチャ設計 §11](../architecture/overview.md) #10）。ほかの権限を要求しない。依存ライブラリがマニフェストのマージで加える権限（WorkManager の `WAKE_LOCK`・`ACCESS_NETWORK_STATE` など）は、マージ後のマニフェストを確認して許容する |
| NFR-AND-SEC-7 | 外部に公開するコンポーネントは `MainActivity`（起動と `swarm-viewer` スキームの intent-filter）だけ。通知の `PendingIntent` は `FLAG_IMMUTABLE` とし、Intent に載せるのは接続 ID・セッション ID だけ |
| NFR-AND-SEC-8 | ツール応答の表示は常にテキストとして描画し、解釈しない（`title`・`activity`・`note`・`pane_tail`・`ask_card` の質問文・選択肢は信頼できない入力。NFR-SEC-10 と同じ） |
| NFR-AND-SEC-9 | 操作の UI は NFR-SEC-12〜14 と同じ規則に従う。見るだけの接続には Compose にも操作部品（選択肢のボタン・送信欄・中断・終了・「＋ 始める」）を出さない。中断・終了は確認ダイアログ（NFR-DSG-8 の `AppDialog`）を経ないと送らない。送信中は `PaneDetailViewModel` の `busy` で同じペインの全ボタンを無効にし、`PaneService` も同じペインの操作を排他にする。操作の接続の token は ops スコープで、失効はホストの `swarm-console token revoke <名前>`（NFR-SEC-13）。送信本文・`pane_tail` の本文は、ディスク・ログ・通知・Intent に出さない |

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
| NFR-AND-LOG-2 | 既定では警告・エラーだけ記録する。設定画面の「デバッグログ」で詳細のオン/オフを切り替える（既定オフ）。どちらも token・完全な URL・ペインの本文（`activity`・`note`・`pane_tail`・`ask_card`）・送信本文を出さない。操作のログに残すのは、接続 ID・ツール名・種別・コード・ペイン位置まで。接続は表示名と host だけで識別する（NFR-LOG-3・NFR-SEC-11） |

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
| NFR-AND-QLT-1 | domain（`parsePairing`・`validateUrl`（`/report`・`/ops`）・`parseReport`・`parseSessions`・ツール結果のパーサ・`detect`・`classify`（`operation` を含む）・`usageTone`）は、デスクトップ（Go）と**同じテストベクタ**（`testdata/vectors/*.json`）を読む JUnit テストで検証する。ベクタのケース数の下限（`/ops` のケースを足した数）を検査し、ファイルが空・読めないときに静かに空振りしない |
| NFR-AND-QLT-2 | token が `toString`・ログ・例外メッセージに出ないことをテストで検証する |
| NFR-AND-QLT-3 | `./gradlew lint ktlintCheck test` がクリーンに通る（Android Lint は警告もエラー扱い）。ポーリング・ワーカーのテストは固定時間の待ちに頼らない（NFR-QLT-3） |
| NFR-AND-QLT-4 | 実機でしか確認できない項目（cleartext・Keystore・QR・Doze・Tailscale・折りたたみ・再起動後の再開）は、[アーキテクチャ設計 §11](../architecture/overview.md) #8〜#15 に未確認として残し、確認後に確定する |
| NFR-AND-QLT-5 | 操作モードは NFR-QLT-4 と同じ 6 項目を JUnit（MockWebServer・ViewModel のテスト）で検証する。見るだけの接続では、偽のコンソールが `/ops` へのリクエストを 0 件と記録すること、操作の部品が出ないこと（ViewModel の状態）も確かめる |

## 11. Ousiass デザインガイド準拠

Web UI と Android UI は Ousiass 共通デザインガイド（[brand-guide](https://ousiassllc.github.io/brand-guide/)、確認日 2026-11-05）に従う。ガイドと食い違ったらガイドが正で、下の逸脱だけを承認済みとする。

| ID | 要件 |
|----|------|
| NFR-DSG-1 | 色はガイドのトークン（`--ink` `--bone` `--mute` `--surface` `--border` `--accent` `--accent-soft` `--focus` と状態色 `--success` `--warning` `--danger` `--info`）だけを使う。Web は `static/src/css/app.css` に定義し、Tailwind の既定パレットは無効にする。Android は `ui/theme/` の ColorScheme と状態色で持つ。独自の色・ハードコードの色を足さない |
| NFR-DSG-2 | ライト / ダークはシステムの設定に追従する（Web は `prefers-color-scheme`、Android はシステムテーマ）。デスクトップのウィンドウ背景（WebView の描画前・リサイズ中に見える面）も OS のテーマの `--bone`（ライト `#F5F5F4`・ダーク `#050505`）にし、テーマ変更に追従する（値は `internal/app/theme` に 1 か所。Android は `windowBackground`）。手動の切替 UI は持たない |
| NFR-DSG-3 | フォントは NFR-SEC-9 のとおり同梱する。SemiBold は見出しとボタンだけに使い、Medium・Bold を使わない。本文は 14px 以上（説明・警告・エラーの文章を 12px にしない）。入力欄・select・textarea の文字は 16px（`atoms.FieldClass` の `text-base`）。Android のボタン（`labelLarge`）も標準の高さ 40dp に合わせて 16sp。12px はバッジ・日時・ペイン番号などのキャプションだけ。コード・桁を揃える数値・接続先の host（IP リテラル）・ホスト由来の `usage.reason` は JetBrains Mono（Web は `font-mono`、Android は `MonoFontFamily`。ペアリング確認の host は「接続先: 」のラベルと値を分けて値だけ Mono にする） |
| NFR-DSG-4 | 主操作・リンク・選択中タブはインク色（`--accent`）で、青などのアクセント色を使わない。`working` と `report_waiting` は無彩色で、状態はラベルで区別する。フォーカスは `--focus` の 2px（offset 2）、入力欄とチェックボックスは `--focus` の 40% のリング（`--ring`。入力欄 2px・チェックボックス 3px） |
| NFR-DSG-5 | トーストは画面下中央・端から 24px・高さ 40・ピル・1 行の短文・3 秒で消え・同時に 1 つ。エラー（削除失敗など）はトーストにせず、インラインアラートで示す。操作の結果の通知は NFR-DSG-16 |
| NFR-DSG-6 | モーションはガイドのトークン（トースト・アラートの出現 320ms ease-out、モーダルの出現（本体とスクラム）は slow 帯の 280ms ease-out、退出 240ms ease-in）。`prefers-reduced-motion: reduce` ではフェードを付けない（スピナーの回転は処理中表示のため維持する。周期はガイドの 0.9s）。トースト・インラインアラートは出現でフェードイン、トースト・インラインアラート・モーダル（スクラムを含む）は退出でフェードアウトする。応答の差し替えで消えるアラートとモーダルは、フェードアウトを終えてから差し替える。ボタンの押下は小さなリップル（280ms ease-out）。ボタン・リンク・タブ・接続切替のホバーは色・背景・枠・下線の色だけを `--motion-base`（180ms）・ease-out で遷移させる（`.hover-fade`。reduced-motion では遷移を止め、色の変化は残す）。Android は Compose の既定のアニメーション（ボタンのリップルを含む）に任せ、独自のモーションは持たない |
| NFR-DSG-7 | 状態は色だけでなくアイコン（Iconoir）と文言で伝える。インラインアラート（エラー・警告・お知らせ・成功）は Web の `atoms.Alert`、Android の `AppAlert` に統一する。形はガイドの見本どおり、surface 地・1px の border・左 3px の状態色・角丸 `--r-md`・文字は `--ink`（状態色はアイコンと左の帯だけ）。状態を示すバッジ（ペイン状態・要対応・上限到達・表示中）は、ガイドのステータスピルの見本（`● 完了`）どおり、文言の前に文字と同じ色の 6px の点を置く（Web は `BadgeProps.Marker`・Android は `Badge(marker = true)`）。エージェント名のように状態でないバッジには点を付けない。接続管理で「いま表示している接続」の印は、Web・Android とも「表示中」のバッジ（Accent）にそろえる。`--mute` はライト `#6b6b6e`・ダーク `#8b8b8e`（ガイド C6。bone・surface 上で AA 4.5:1）で、補助テキスト・キャプションと、中立のバッジ（エージェント名・shell / unknown。ガイドの `.pill.neu`）の文字・枠・地に使う。それ以外の情報の文字（値・文章）には使わない。カメラ映像の上に置く文言（Android の QR 読み取り）は、映像の明るさに左右されないよう ink の帯（`inverseSurface` 地・`inverseOnSurface` の文字・角丸 `--r-md`・内側 16dp）に載せる |
| NFR-DSG-8 | 確認・破壊的操作（接続の削除・保存データの初期化・接続の追加の確認）は中央モーダルで取る。Web はネイティブ `<dialog>`（`--r-xl`・`--shadow-modal`・スクラムは外側クリックで閉じてフェードする。スクラムの濃さはガイドの見本（ink 18〜22%）に合わせ、ライトは `--ink` の 20%。ダークは `--ink` が白になるので暗い `--bone` の 60%（`--bone` はウィンドウ背景とほぼ同じ色なので、見本の 20% では暗幕にならない）。値は `app.css` の `--scrim` が唯一の出どころ）、Android は `AppDialog`（`BasicAlertDialog` に面・1dp の border・`extraLarge`・1dp の影を組んだもの。Material の `AlertDialog` は枠も影もないので使わない）。構造はガイドの見本（`.modal`）のとおり、ヘッダー（見出しと閉じる × のアイコンボタン。Web は `atoms.ModalHeader` の `data-modal-close`、Android は `AppDialog` のタイトル行で、どちらも外側クリック・Esc・戻ると同じ「閉じる」要求。ヘッダーの上下 padding は 16・左右は 20 で、Android は閉じるボタンのアイコンの余白 8dp を足して右 12dp）・本文・フッター（右寄せ・ボタン間 8）を 1px の border で区切り、幅は最大 420。本文の部品間は 16（Web は `ModalBodyClass` の `gap-4`）で、文の行どうしは 8（Web は `<dl>` の `gap-y-2`・Android の確認ダイアログは文の `Column` を `spacedBy(8.dp)` にして、その外側のアラートとの間を 16dp にする）。`window.confirm`・全画面ページでの確認を使わない。確定の危険な操作のボタンは `--danger`（Web は `ButtonDanger`、Android は `AppDangerButton`）で、どちらも透明地・文字は danger・枠は danger の 45%（無効は 0.4 倍）の枠付き。削除確認のキャンセルは `AppOutlinedButton`（サブ。1px の border） |
| NFR-DSG-9 | 部品は M3・OS の既定ではなく、ガイドの見本の形に揃える。select は OS の矢印を消し（`appearance-none`）、`--mute` のシェブロンを `linear-gradient` 2 本で右端に描く（`atoms.SelectClass`。CSP のため `data:` URI は使わない）。Android の使用率バー（`ToneBar`）は M3 の「塗りとトラックの隙間」と終端の点を消し（`gapSize = 0.dp`・`drawStopIndicator = {}`）、連続したトラックの上に塗りが載る形にする。Web の使用率バーは塗り（`::-webkit-progress-value`・`::-moz-progress-bar`）も `rounded-full` で両端を丸める。セッションタブは上下 10px・左右 0・タブ間 24px で、選択中は 2px の `--accent` の下線、未選択は `--mute`（ホバーで `--ink`）の文字だけにし、角丸とホバーの面を付けない。Android の引っ張って更新のインジケーターは M3 の既定（影つき）でなく、影なし・`surface` 地・1dp の `outline` 枠の円に円形進捗（ガイドのスピナーと同じ 20dp・2dp、先端 `primary`・トラック `surfaceVariant`（accent-soft））を置く自前の部品（`AppPullToRefreshIndicator`）にする。Android のスイッチ（`AppSwitch`）は 36×20dp・つまみ 14dp、ラジオ（`AppRadio`）は 18dp の円・1dp の `--mute` 枠で選択時は `primary` の塗りに 6dp の `background` の点にし、M3 の `Switch`・`RadioButton` は使わない。テキストリンク（Web の `navLink`・`repairLink`）は共通の `.text-link`（下線 `--mute`・位置 3px・ホバーは下線を `--ink` にするだけで面は付けない）にする |
| NFR-DSG-10 | 画面の四辺（上下左右）のガターは幅の区分で決める: Compact（〜599）16・Medium（600〜839）24・Expanded（840〜）32。Web は `layout/base.templ` の `p-4 medium:p-6 expanded:p-8`（`medium`・`expanded` は `app.css` の `--breakpoint-medium: 600px`・`--breakpoint-expanded: 840px`。Android の `WindowWidthSizeClass` と同じ境目）。Android は `MainActivity` が幅の区分を `LocalGutter`（`gutterFor`）に入れ、メイン（四辺とも。セッションタブの `edgePadding` を含む）・接続管理・ペアリング・QR 読み取り（権限の案内とカメラ上の文言・ボタン）・設定が使う。OS ごとの差は、システムが内容に重ねる領域の扱いで吸収する: Android はステータスバー・ナビゲーションバーを各画面の `Scaffold` が、横向きの切り欠き（ノッチ・パンチホール）を `MainActivity` の `displayCutout` の横 inset が避け、ガターはその内側に足す。デスクトップ（Windows・macOS）はウィンドウの枠・タイトルバーが WebView の外にあるので、ガターだけを付ける。枠を持つ独立した部品（アラート）を縦に並べる間隔は 8（Web は `gap-2`・Android は `spacedBy(8.dp)`）。部品内（利用上限のアカウント名→枠の行・接続カードの名前と host 以外の並び）も 8、2 行リストのタイトル→キャプションは 2（`--list-title-caption-gap`。Web は `gap-(--list-title-caption-gap)`・Android は `spacedBy(2.dp)`）。設定行（Android のラジオ・スイッチの行）は最小の高さ 48（`--list-row-h`。`ListRowMinHeight`）。テキストリスト（ペインのメモ）は項目間 8（`--list-gap`）・記号→文字 10（`--list-marker-gap`）・記号は 5px の `--mute` の点（`--list-marker-size`）で、Web は `flex` と `before:` の円、Android は `NoteItem` の円。角丸は `--r-sm`〜`--r-xl` と `--r-full`（9999px）を `app.css` の `:root` に持ち、`@theme` の `--radius-*` 経由で使う（`.ripple` も `var(--r-full)`） |
| NFR-DSG-11 | モーションの時間とトーストの寸法は CSS のトークン（`app.css` の `:root`: `--motion-fade-out`・`--toast-duration` 3s・`--toast-h` 40px・`--toast-offset`）が唯一の出どころで、`app.js` は数値を持たず `getComputedStyle` で読む（`cssDurationMs`。トーストの 3 秒と、応答の差し替えを退出フェードの終わりまで待つ時間）。`toast.templ` の高さも `h-(--toast-h)`。トークンだけを変えても CSS のフェードと JS の待ちがずれない。アクション付きトーストの 5 秒（`--toast-duration-action`）は、アクション付きのトーストを入れるときに足す |
| NFR-DSG-12 | ボタンの左右 padding は 18（小は 12。Web は `px-[18px]`、Android は `AppButton`・`AppOutlinedButton`・`AppDangerButton` の `ButtonContentPadding`。上下は高さ 40 が決める）。チェックボックス・ラジオ→文言は 10（`--list-marker-gap`。Web は `atoms.CheckboxRowClass`。Android のラジオは `AppRadio` が円の周囲に 3dp の余白を持つので、文言の前に足す余白は 10 − 3 = 7dp の `RadioLabelStart`）。空状態の padding は縦 48・横 24 で、画面幅で変えない（Web は `px-6 py-12`、Android は `EmptyStateHorizontalPadding`・`EmptyStateVerticalPadding`） |
| NFR-DSG-13 | トーストの出現・退出の移動量は 8px（ガイド M5「下から 8px」）。`motion.css` の `fade-in`・`fade-out` は移動量を `--fade-rise`（既定 4px）で読み、`toast.templ` の本体が `[--fade-rise:8px]` を付ける（アラート・モーダルは既定の 4px） |
| NFR-DSG-14 | 検証に失敗した入力欄は `--danger` の枠で示す（ガイド O7 の `.fld.bad`。文言は入力欄の直下（間隔 4 = `--gap-help`）のインラインアラートで、ボタンより上に置く）。Web はペアリングの textarea に `atoms.FieldInvalidClass`（枠 `--danger`・フォーカスのリング `--ring-danger` = `--danger` の 40%）を重ね、Android は `AppTextField(isError = …)` で枠・caret・ラベルを `ExtraColors.danger` にし、ペアリングの `PasteSection` は入力欄と `AppAlert` を 4dp 間隔の `Column` にまとめてから「接続」「QR を読み取る」を出す。トーストの幅は `min(92vw, 420px)` で頭打ち（`toast.templ` の `max-w-[min(92vw,26.25rem)]`。接続名など利用者が決める長い文言でも端まで伸びない） |
| NFR-DSG-15 | 入力欄はガイドの見本（`.lbl`＋`.fld`）の形にする。ラベルは入力の上（間隔 6）、入力は高さ 40・内側 12・角丸 `--r-md`・1px の枠・surface の地。Android の `AppTextField` は M3 の枠内に浮くラベル・高さ 56dp の `OutlinedTextField` を使わず、`BasicTextField` と `OutlinedTextFieldDefaults.DecorationBox`（内側 左右 12dp・上下 8dp。文字は `bodyLarge` の行高 28.8sp ではなく `FieldLineHeight` の 24sp に抑えて 8 + 24 + 8 = 40dp）に、ラベルを別の `Text`（`bodyMedium`）で上に置く。読み上げは入力欄の `contentDescription` にラベルを渡す。アコーディオン（`<details>`）は、単独のものにガイドの `.acc`（`acc` クラス: 枠 1px `--border`・角丸 `--r-lg`・surface の地・summary の padding 12px 16px、本文は `acc-body`（padding 0 16px 14px）で包み文字色は `--mute`）を付ける。アラートの中の `<details>`（取得失敗の詳細）は二重枠を避けるため `acc` を付けない。値の `<code>`（`usage.reason`・エラーコード）は ink のまま |
| NFR-DSG-16 | 操作モードの部品。(1) 質問カードの選択肢・「その他」・送信・フォルダの行は大きなボタンにし、Android は最小 48dp（親指で押せる大きさ）。(2)「中断」は詳細の最下部の控えめなボタン（サブ）にし、中断・終了の確認は NFR-DSG-8 のモーダル（Web は `ConfirmDialog`、Android は `AppDialog`）で取る。確定の危険なボタンは `--danger`。(3) 送信中は同じペインのボタンをすべて無効にする（Web は `hx-disabled-elt`、Android は `busy`）。(4) 操作の結果（成功・`ok: false`）は NFR-DSG-5 のトーストで知らせ、ツールのエラー（[機能要件 §10.1](functional.md)）はインラインアラートで示す。(5) 接続の種類のバッジ（「操作」「見るだけ」）は色だけに頼らず文言を併記し、既存のバッジ・アイコン部品を使う。(6) 見るだけの接続の質問カードは、選択肢をボタンにせず一覧で出す |
| NFR-DSG-17 | ペインカードの要対応強調（ガイドに部品がないため案件ガイドとして定める）。(1) 強調するのは attention の `asking`・`stalled` に該当するペインだけ（`report_waiting` は強調せず、バッジを併記するのみ。機能要件 §7）。(2) 表現は状態色 `--warning` だけで組み、独自の色・影・アイコンを足さない。Web は `molecules.paneCardClass` の `border-warning ring-2 ring-warning/40`（通常の 1px の `--border` を `--warning` に替え、外側に `--warning` の 40% の 2px のリングを重ねる。ガイドの `--ring`（`--focus` の 40%）と同じ濃さ）。Android は `PaneCard` の枠を 2dp の `DisplayTone.Warning` に替える（リングは持たない。枠の太さで同じ強さを出す）。角丸・余白・背景は通常のカードと同じで、カードの大きさを変えない。(3) 強調は色だけに頼らない。同じカードの状態バッジの文言（「答え待ち」「止まってるかも」など。NFR-DSG-7）と、要対応バー（機能要件 §7）の項目が文言で同じ内容を伝える。(4) 強調は 1 枚のカードに 1 回だけ（二重の枠やリングを重ねない）。`--warning` の枠・リングは要対応のペインカード以外に使い回さない |
| NFR-DSG-18 | 古いデータのグレーアウト（ガイドに規定がないため案件ガイドとして定める）。(1) かける条件は 2 つだけ: 取得に失敗して保存済みの直前値を表示しているとき（機能要件 FR-POLL-4・§10）と、`usage.status` が `stale` のとき（機能要件 §5.1）。新しい値を表示しているものには使わない。(2) 表現は不透明度 50% と彩度 0。Web は `opacity-50 grayscale`（`organisms/snapshot_view.templ` の `snapshotClass`、`organisms/usage_panel.templ` の `usageSectionClass`）。Android は `Modifier.grayedOut(stale)` の `alpha(0.5f)` だけで、彩度は落とさない（Web との差として承認する）。独自の色や取り消し線などは足さない。(3) かけるのは 1 回だけ。`opacity` は入れ子で掛け算になり、二重にかけると読めなくなる（ガイド C5）。Web は取得失敗のとき領域 `#snapshot` に 1 回かけ、その中のカード・パネルには単体でかけない（`PaneCard`・`HostPanel` は持たない）。`UsagePanel` は `RegionGrayed` でないとき（領域が新しいのに `usage.status` だけ `stale`）に限り単体でかける。Android は `grayedOut` を要対応バー・ペインカード・ホスト負荷・利用上限の各ブロックに 1 回ずつ付け、入れ子にしない（`UsagePanel` は取得失敗と `stale` のどちらでも 1 回）。(4) 失敗のバナー（`#snapshot-status`）は領域の外に置き、グレーアウトしない。(5) 色（薄さ）だけに頼らず文言を併記する。取得失敗は失敗バナーの「前回取得 HH:MM」（機能要件 §10）、`stale` は利用上限パネルの「古いデータ」と `fetched_at`。グレーアウト中もカードのリンク（ペイン詳細）は押せる。薄くした文字は AA（4.5:1）を下回りうるため、読み取りの保証は併記した文言が担う |
| NFR-DSG-19 | 省略した文字の全文表示（ガイドに規定がないため案件ガイドとして定める）。(1) 対象は、1 行で切るペインカードのタイトル・`activity`・メモ（`note.repo`・`note.items`）。Web は `truncate`、Android は `maxLines = 1` と `TextOverflow.Ellipsis` で切る。切るなら、下の全文を見る手段を必ず付ける。(2) ホバー（Web・補助）はブラウザ標準の `title` 属性（`molecules/pane_card.templ`）。タイトルの `title` は「タイトル」と `cwd` を改行で並べ、`activity` はその全文、メモは `repo` と `items` の全件（`noteTooltip`）、ペイン位置は `cwd`。値は文字列だけで HTML を入れない。ガイドのツールチップは非対話・12px の部品で別物のため使わない。`title` はタッチ・キーボードのフォーカスでは出ないので、これだけに頼らない。(3) タッチ・キーボードでは、カードを開いてペイン詳細（S4。機能要件 §4.5）で全文を見る。Web はカード全体が `<a>`（`data-pane-link`。Tab で届き Enter で開く。`aria-label` は「ペイン T1 の詳細を開く」）、Android はカード全体が `clickable`（タップで開く）。S4 のヘッダは、タイトルと `cwd` を折り返して全文を出す（Web は `break-words`・`break-all`、Android は折り返す `Text`）。`activity` の周辺は S4 の本文 `pane_tail`（画面の末尾）で見る。メモ（`note.repo`・`note.items`）は Web・Android とも S4 のヘッダが全件を折り返して全文で出す（Web は `molecules/pane_header.templ` が `paneNote` を全文モードで描画し、`truncate`・先頭 3 件の要約・「+N」・`title` を使わない。Android は `items` を全件出す）。`pos` が空でリンクを持たないカードは、`title` だけになる。(4) クリックで展開・長押しのポップアップなど、全文を見るための部品をカードに足さない（カードは読み取り専用。機能要件 §4.4）。(5) ペインカード以外の省略（ホスト負荷の行・要対応バーの項目・接続名）は、現状 `title` だけで、この規定の対象外 |
| NFR-DSG-20 | 使用率バー（利用上限の枠・ホスト負荷の CPU・スワップ・ディスク）の塗りは、使用率が 80% 未満なら `--accent`（ガイドの進捗の見本どおり）、80% 以上 95% 未満なら `--warning`、95% 以上なら `--danger` にする。利用上限の枠が `exhausted` のときは使用率に関わらず `--danger`。`unit` が `percent` 以外（`usd` など）の枠は数値だけでバーを出さず、取得できない枠はしきい値なし（`--accent`）。しきい値は `domain.WarnPercent`（80）・`domain.CriticalPercent`（95）が唯一の出どころで、判定は `domain.PercentTone`・`domain.UsageTone`、表示色への対応は `format.PercentTone`・`format.SlotTone`・`format.BarClass`（Web）と `ToneBar`（Android）が受け持つ。ガイドの進捗は `--accent` 1 色で、状態色のしきい値はガイドに規定がないので承認済みの逸脱（下の表）とする。色だけに頼らないよう、バーの隣に使用率の数値（`font-mono`・バーと同じ色の文字）を併記し、`<progress>` には `aria-label` を付ける |
| NFR-DSG-21 | Web の見出しは密なアプリ画面向けの階層にし、h1 は画面の見出しで 16〜18px SemiBold（`pageTitle`・`PaneHeader` は `text-lg` = 18px。メイン画面の見出しだけは接続名・種類バッジ・更新・設定を 1 行に並べるヘッダーの一部なので `text-base` = 16px）、h2 はパネル・セクションの見出しで 14px SemiBold（`HostPanel`・`UsagePanel`・新しいセッションの手順の見出しは `text-sm font-semibold text-ink`）、h3 はパネル内の小見出し・アカウント名で 14px Regular（`text-sm text-ink`。SemiBold は h1・h2 だけ）。12px（`text-xs`）の見出しは作らない（NFR-DSG-3）。例外はモーダルの見出し（`ModalHeader` の h2）で、別の文脈なので 18px SemiBold にする。ガイドに規定がないため、追加提案を §11 の承認済みの逸脱に記録する。Android は対象外 |
| NFR-DSG-22 | Web のメイン画面の接続切替（接続が複数のときだけ）は `<nav aria-label="接続の切替">` の中に接続ごとのリンク（チップ）を並べる。チップは枠 1px `--border`・角丸 `--r-md`・padding 縦 4px 横 8px・文字 14px（`text-sm`）・`text-ink` で、ホバーと現在の接続（`aria-current="true"`）は `--accent-soft` の地にする。色・背景の遷移は `hover-fade`（180ms）。ガイドに当たる部品がない（タブでもセグメントでもない）ため、追加提案を §11 の承認済みの逸脱に記録する。Android は対象外（接続の切替はメニュー） |

承認済みの逸脱（ガイドの値と異なるが、プロダクト固有の理由で維持するもの）:

| 箇所 | 逸脱 | 理由 |
|------|------|------|
| 要対応ペインの強調（`ring-warning`、Android は 2dp の warning 枠） | ガイドに部品がない | 状態色（`--warning`）だけで組み、形・用いる条件を NFR-DSG-17 で案件ガイドとして規定する。ガイド本体への部品の追加は、ガイドの運用側へ提案する課題として残す |
| 主ボタンのホバー（`atoms.Button` の `ButtonPrimary` は `hover:bg-accent/85`） | ガイドの見本の `.pri` には `:hover` がなく、共通の `.btn:hover{opacity:.85}` が主ボタンにも掛かる（文字ごと薄くなり、`opacity` は `hover-fade` の遷移（色・背景・枠・下線）の対象外） | 地を `--accent` の 85% にし、文字（`--bone`）は薄くしない。ガイドの「ホバーは色/背景のみ・180ms」（M1）に沿い、`hover-fade` で 180ms ずつ遷移する。ガイドへの追加提案: 主ボタンに `.pri:hover{background:color-mix(in srgb,var(--accent) 85%,transparent)}` を規定する（危険ボタンの `.dan:hover` と同じ `color-mix` の書き方）。Android はホバーを持たない |
| 古いデータのグレーアウト（`opacity-50 grayscale`、Android は `alpha`） | ガイドに規定がない | 形・かける範囲・併記する文言を NFR-DSG-18 で案件ガイドとして規定する。ガイド本体への追加は、ガイドの運用側へ提案する課題として残す |
| 使用率バーのしきい値色（`warning` / `danger`。NFR-DSG-20） | ガイドの進捗は `--accent` 1 色で、状態色のしきい値の規定がない | 使用率の高さ（80% 以上・95% 以上）を色で示す必要がある。数値と文言を併記し色だけに頼らない。ガイドへの追加提案として別途出す |
| 密なアプリ画面の見出しのサイズ階層（Web の h1 16〜18px・h2 14px・h3 14px。NFR-DSG-21） | ガイドに、1 画面にホスト・利用上限・ペインを並べる密なアプリ画面の見出しのサイズ階層の規定がない。本文 14px 以上・12px はキャプションだけ（NFR-DSG-3）なので、h3 を 12px にもできない | 見出しの階層はサイズと太さで示す: h1（画面）16〜18px SemiBold、h2（パネル・セクション）14px SemiBold、h3（パネル内の小見出し・アカウント名）14px Regular。ガイドへの追加提案: 密なアプリ画面向けの見出しスケール（h1 16〜18px・h2 14px・h3 14px で、SemiBold は h1・h2 だけ。12px はキャプションだけ）を規定する。Issue #79 は h3 を 12px と提案していたが、12px の見出しは NFR-DSG-3 に反するため 14px とした |
| 接続切替チップ（複数接続のときの `nav`。枠 1px `--border`・角丸 `--r-md`・14px・選択中と hover は `--accent-soft` の地。NFR-DSG-22） | ガイドの部品に当たるものがない。セッションタブ（下線だけ・面なし。NFR-DSG-9）とは形が違い、ガイドにセグメント（排他の切替）の部品もない。実体は `role="tab"` のタブではなく、接続ごとに別のページ（フラグメント）へ遷移するリンク | 色・文字・角丸・余白はトークンに揃え、`<nav aria-label="接続の切替">` の中のリンク（`aria-current="true"` で現在の接続を示す）にする。選択中は地の色だけに頼らず、見出しの「接続: {名前}」でも現在の接続を文言で示す。ガイドへの追加提案: 少数の項目から 1 つを選ぶ切替チップ（セグメント）を規定する（枠 1px `--border`・`--r-md`・padding 4px 8px・14px、選択中は `--accent-soft` の地、ホバーは色/背景のみ・180ms）。Android は接続の切替をメニューで行うため対象外 |
| Nerd Font の PUA 用フォント（Web・Android 共通） | ガイドに「ユーザーデータ内の記号」の規定がない | ペインタイトルのアイコン（UI アイコンではない）。UI アイコンは Iconoir |
| 状態色バッジのコントラスト（ライトの warning 文字は背景上 4:1 前後） | AA（4.5:1）未満 | ガイドのトークン値のまま使う。文言と併記しており色だけに頼らない。値の見直しはガイド側の課題 |
| Expanded（840px・dp 以上）のガター 32（Web `expanded:p-8`、Android `gutterFor`） | ガイド S5 のガターはモバイル 16・デスクトップ 24 の 2 段階 | 広い画面（デスクトップの既定ウィンドウ 1100px・タブレット）で 24 のままだと内容が端に張り付いて見えるという報告（Issue #99）への対応。Compact・Medium の値と、Medium の境目 600 は変えない。ガイド本体への追加は別途提案する |
| 画面のセクション間（`layout/base.templ` の `gap-4` = 16px、Android も 12〜16dp） | ガイドのセクション間は 48〜72px | 1 画面にホスト負荷・利用上限・ペインを並べるダッシュボードで、48px にすると 1 画面に収まらず状態を見渡せない。ガターは NFR-DSG-10 のとおり幅の区分で決める（Compact 16・Medium 24・Expanded 32） |
| 1 文で足りるインラインアラート（フォーム・操作のエラー、警告帯。見出しなしで本文 1 文） | ガイドのインラインアラートは「アイコン＋見出し＋本文」 | 固定文言の 1 文に、同じ内容の見出しを重ねても情報が増えない。取得失敗のバナーのように「種別＋詳しい内容（前回取得・次回再試行）」を持つものだけ、見出し（`Title`）と本文に分ける |
| Android の `DropdownMenu` / `ScrollableTabRow` / `TopAppBar`（Material 部品） | ガイドのメニュー・タブ・ナビの仕様と形が違う | 色・文字・角丸・余白はトークンに揃え、形の差は案件ガイドの課題とする |
| Android の引っ張って更新のインジケーター（`AppPullToRefreshIndicator`。NFR-DSG-9） | ガイドにモバイルの更新操作（pull-to-refresh）の規定がない | M3 の既定（影つき）はガイドの「影はモーダルだけ」と食い違うので、ガイドの部品（surface の地・1px の border・影なし・20dp・2dp のスピナー）で組む。ガイドへの追加提案は [brand-guide#9](https://github.com/ousiassllc/brand-guide/issues/9)。規定が入ったら本行を外し、実装をガイドに合わせる |

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
| 4.7 | 2026-11-06 | NFR-DSG-8 にモーダルのヘッダー padding 16・本文の行間 8、NFR-DSG-14 に入力欄の下のエラー文言の位置、NFR-DSG-15 に入力欄の文字の行高 24sp を追記 | Issue #89・#90（デザインガイド S2・O8） |
| 4.8 | 2026-11-08 | 操作モードを追加。NFR-SEC-1 を「接続の種類ごとの許可リスト以外は呼べない」に書き換え、NFR-SEC-5 に path `/ops` を追加、NFR-SEC-10・11・NFR-LOG-3・NFR-AND-LOG-2 に `pane_tail`・`ask_card`・送信本文を追加。NFR-SEC-12〜14（確認・ops token の失効・見るだけの隔離）、NFR-PERF-8、NFR-QLT-4、NFR-DSG-16 を追加。Android は NFR-AND-SEC-5 の書き換えと NFR-AND-SEC-9・NFR-AND-PERF-8・NFR-AND-QLT-5 | Issue #97（操作モード） |
| 4.9 | 2026-11-09 | 独立レビューの指摘とコード修正の反映。NFR-PERF-4・NFR-AND-PERF-4 をツール別のタイムアウト（読み取り 10 秒、`ask_pick`・`send_text`・`send_escape` 30 秒、`pane_start`・`pane_close` 50 秒）に、NFR-SEC-12 に `pane_close` の `confirm=true` とサーバー側の状態の再検査を追記 | Issue #97（独立レビュー ReviewDocs） |
| 5.0 | 2026-11-09 | NFR-DSG-8 にスクラムの濃さ（ライト ink 20%・ダーク bone 60%）を明記。ガイドは濃さを規定しておらず、見本（ink 18〜22%）に実装を合わせる判断と、ダークだけ bone 60% にする理由を記録 | Issue #92（デザインガイド O1） |
| 5.1 | 2026-11-09 | §11 の承認済みの逸脱に「主ボタンのホバー（`--accent` の 85% の地）」を追加し、ガイドへの追加提案の文言を記録した | Issue #93（デザインガイドの主ボタンに `:hover` の規定がない） |
| 5.2 | 2026-11-09 | §11 の承認済みの逸脱に「Android の引っ張って更新のインジケーター」を追加し、ガイドへの追加提案（brand-guide#9）を記録した | Issue #94（デザインガイドにモバイルの更新操作の規定がない） |
| 5.3 | 2026-11-09 | NFR-DSG-17（ペインカードの要対応強調）を追加し、承認済みの逸脱の「要対応ペインの強調」を規定への対応に書き換えた | Issue #76 |
| 5.4 | 2026-11-09 | NFR-DSG-18（古いデータのグレーアウト）を追加し、承認済みの逸脱の「古いデータのグレーアウト」を規定への対応に書き換えた | Issue #77 |
| 5.5 | 2026-11-09 | NFR-DSG-19（省略した文字の全文表示）を追加し、承認済みの逸脱に Web のペイン詳細のメモ要約の行を足した | Issue #95・#106 |
| 5.6 | 2026-11-09 | NFR-DSG-20（使用率バーのしきい値色: 80% で `--warning`・95% で `--danger`）を追加し、承認済みの逸脱の理由を実装に合わせて具体化 | Issue #78（デザインガイド C4・O8） |
| 5.7 | 2026-11-09 | NFR-DSG-21（Web の見出しのサイズ階層: h1 16〜18px・h2 14px・h3 14px）を追加し、§11 の承認済みの逸脱に「密なアプリ画面の見出しサイズ」を追加した。ガイドへの追加提案の文言を記録し、`pane_start` の h3 から SemiBold を外して規定にそろえた | Issue #79（デザインガイドに密なアプリ画面の見出しの規定がない） |
| 5.8 | 2026-11-09 | NFR-DSG-22（Web の接続切替チップ）を追加し、§11 の承認済みの逸脱に「接続切替チップ」を追加した。ガイドへの追加提案の文言を記録した | Issue #80（デザインガイドに接続の切替チップに当たる部品がない） |
| 5.9 | 2026-11-09 | NFR-SEC-9 に、Android も同梱の Symbols Nerd Font Mono（TTF）を UI の書体の代替にすると明記。§11 の承認済みの逸脱「Nerd Font の PUA 用フォント」を Android にも適用 | Issue #81（Android のペインタイトルの Nerd Font アイコン） |
| 6.0 | 2026-11-09 | NFR-DSG-10 のガターを、四辺とも幅の区分（Compact 16・Medium 24・Expanded 32。Web と Android で同じ境目）で決めるよう改め、Android の切り欠き（横 inset）を避けることを追記。ガイド S5（16・24）に Expanded の 32 を足すため、承認済みの逸脱に記録した | Issue #99 |
| 6.1 | 2026-11-09 | NFR-DSG-19 を改訂。Web の S4 のヘッダもメモの `repo` と `items` を全件・全文（折り返し）で出すと明記し、承認済みの逸脱から「Web のペイン詳細（S4）のメモが要約のまま」の行を外した | Issue #106（Web のペイン詳細のメモが要約のままで、タッチ・キーボードで全文を見られない） |
