# API 仕様

swarm-viewer が関わる API は 4 種類ある。viewer 自身は**外部に公開する API を持たない**（TCP ポートを開かない。[アーキテクチャ設計 §1](../architecture/overview.md)）。

| 種別 | 向き | 内容 | 節 |
|------|------|------|----|
| swarm-console `/report` の利用 | viewer → swarm-console | MCP `tools/call` の `sessions_list`・`report` | §1 |
| ペアリング形式 | swarm-console → viewer | 接続文字列 / `swarm-viewer://` ディープリンク | §2 |
| swarm-console への要望（`pair`） | — | `swarm-console pair` コマンド（未実装） | §3 |
| 画面ルート（アプリ内 HTTP） | WebView → Go | Gin のルート。HTMX が HTML フラグメントを取得 | §4 |

Huma（OpenAPI）で登録する JSON オペレーションは **v1 では 0 本**（[アーキテクチャ設計 §4.3](../architecture/overview.md)）。

## 1. swarm-console `/report` の利用

接続先の仕様の正本は swarm-console の `docs/api/endpoints.md`（v0.1.11 以降）。ここでは viewer が使う部分だけを記す。

### 1.1 接続

| 項目 | 値 |
|------|----|
| プロトコル | MCP（Streamable HTTP、JSON-RPC 2.0）。クライアントは公式 Go SDK（`github.com/modelcontextprotocol/go-sdk/mcp`） |
| URL | 接続の `url`（`http://<tailnet-ip>:48120/report`）。**`/report` のみ**。`/ops` は使わない（NFR-SEC-1） |
| メソッド | `POST` のみ（swarm-console は `GET`・`DELETE` を 405 で拒否するステートレス運用） |
| ヘッダ | `Authorization: Bearer <token>`、`Content-Type: application/json`、`Accept: application/json, text/event-stream`（SDK が付与する Bearer は `http.Client` の Transport で付ける） |
| タイムアウト | 10 秒（接続・応答の合計。NFR-PERF-4） |
| 本文上限 | リクエストは 256 KiB 未満（swarm-console 側の上限。viewer の引数は小さいため問題にならない） |
| リトライ | しない（次の周期で再取得する。NFR-AVL-2） |

### 1.2 呼び出すツール（許可リスト）

viewer が呼ぶツールは次の 2 つだけ。許可リストは `internal/repository/report` の定数で、それ以外のツール名はコードから呼べない（NFR-SEC-1）。

| ツール | 呼ぶ場面 | 引数 |
|--------|---------|------|
| `sessions_list` | 接続の疎通確認（ペアリング）、メイン画面の描画（タブの取得）、通知用ポーリング | `{}`（`all` は指定しない。許可セッションのみ受け取る） |
| `report` | 画面更新ポーリング、通知用ポーリング | `{"format": "json", "session": "<id>"}`（`lang`・`pane` は指定しない） |

- `report` の `session` は**常に明示する**。許可セッションが 2 つ以上のトークンで省略すると `session_required` になるため、許可数に依存しない動作にする
- 未使用のツール（`panes_status`・`host_status`・`pane_tail`・`ask_card`・`report_read` など）は呼ばない。`report` の JSON に必要な値がすべて入っているため
- `tools/list` は呼ばない。許可リストを viewer 側で固定する

### 1.3 `sessions_list`

リクエスト引数: `{}`。

レスポンス（`content[0].text` の JSON）:

```json
{"sessions": [
  {"id": "0", "allowed": true, "operable": false, "layout": {"rows": 2, "cols": [3, 3]}, "panes": ["T1", "T2", "T3", "B1", "B2", "B3"]}
]}
```

| キー | viewer での使い方 |
|------|------------------|
| `sessions[].id` | タブの識別子・表示名（tmux セッション名）。`report` の `session` 引数に使う |
| `sessions[].allowed` | `true` のものだけをタブにする。`false` は表示しない（`all` を指定しないため通常は来ない） |
| `sessions[].layout` / `panes[]` | 使わない（メインの描画は `report` の `layout` を使う） |
| `sessions[].operable` | 使わない（`/report` では常に `false`） |

- 返却順（`config.toml` の `sessions` の順）をそのままタブの順にする
- `allowed` なセッションが 0 件のときは、「許可されたセッションがありません」と表示する

### 1.4 `report`（`format: "json"`）

viewer が読むキーと、画面での使い方。未知のキーは無視する（NFR-CMP-1）。

| キー | 型 | 画面での使い方 |
|------|----|---------------|
| `at` | string（RFC 3339） | スナップショットの時刻（`snapshots.fetched_at` には viewer の取得時刻を保存する） |
| `layout` | `{rows, cols[]}` | ペイングリッドの形（[機能要件 FR-PANE-1](../requirements/functional.md)）。欠落・不整合ならペイン順のリスト表示 |
| `panes[]` | array | ペインカード（§1.4.1） |
| `attention[]` | array of `{kind, pane, detail}` | 要対応バー（[機能要件 §7](../requirements/functional.md)）。3 キーとも文字列 |
| `usage` | object | 利用上限パネル（§1.4.2） |
| `host.cpu` / `host.mem` / `host.swap` / `host.disk[]` | object | ホスト負荷（[機能要件 §6](../requirements/functional.md)）。`host.mem` は `{available, total_gb, used_gb, used_percent}`（`swap` と同じ形。[swarm-console #86](https://github.com/ousiassllc/swarm-console/issues/86)）で、無い古い swarm-console では `available: false` と同じく「—」。`host.load1`・`host.mem_free_gb` は使わない |
| `warnings[]` | `{code, message}` | 警告バナー。`code`: `layout_changed`・`tmux_unavailable`・`session_missing`・`pane_unreadable` |
| `auth.agents[]` | `{agent, status, expires_at}` | `attention` の `auth_expired` / `auth_expiring` の補足（`expires_at` の併記）にのみ使う |
| `omp`・`update` | object | 使わない |

セッションに絞られる値: `panes`・`layout`・`attention` のペイン項目・`warnings`。絞られない値（ホスト全体）: `host`・`usage`・`omp`・`update`・`auth`、および `attention` のログイン項目（`auth_expired`・`auth_expiring`）。

#### 1.4.1 `panes[]`

| キー | 型 | 備考 |
|------|----|------|
| `pos` | string | セッション内の位置（`T1`・`B3` など） |
| `id` | string | tmux ペイン ID（`%12`）。通知の同一性判定に使う（[ER §3.4](../architecture/er.md)） |
| `agent` | string \| null | `omp`・`claude`・`codex`・`null`。未知の値はそのまま文字列で表示する |
| `state` | string | `working`・`asking`・`idle`・`stalled`・`shell`・`unknown`。それ以外は `unknown` として扱う |
| `title` | string | |
| `cwd` | string | ツールチップのみ |
| `elapsed` | string \| null | `42m`・`1h05m` 形式。そのまま表示する |
| `activity` | string \| null | 1 行。`truncated: true` が付くときは末尾に `…` |
| `changed_ago` | string \| null | `stalled` のとき併記 |
| `note` | object \| null | `{from, repo, items[], next}`。viewer は `repo` と `items` だけ使う |

#### 1.4.2 `usage`

| キー | 内容 |
|------|------|
| `status` | `ok`・`stale`・`unavailable`・`pending`・`disabled`（それ以外は `unavailable` として扱う） |
| `reason` | `stale`・`unavailable` のときの理由コード（`not_found`・`timeout`・`exec`・`parse`）。折りたたみ表示 |
| `fetched_at` | 最後に成功した取得の時刻。`null` あり |
| `claude[]` / `codex[]` | `{account, five_hour, seven_day, reason}`。`reason` は 2 枠とも取れなかったときの理由コード（`without_usage`・`no_email`。`null`・欠落・知らない値は理由なし） |
| `cursor[]` | `{account, window, models}`（`reason` が付いても使わない）。`window` は `monthly`・`weekly` など、`null` あり |
| 枠 | `{available, used, unit, status, resets_at}`。`available: false` のときほかは `null`。`unit`: `percent`・`usd`。`status`: `ok`・`exhausted`・`unknown` |

- 7 日枠のキーは `seven_day`（`weekly` ではない）
- 週枠が欠けるアカウントがありうる。必ず `available` を確認し、`false` は「—」で表示する
- `claude[]` / `codex[]` で 2 枠とも `available: false` のアカウントは「—」を 2 行出さず「取得できません」を 1 行で出し、`reason` があれば理由を添える（[機能要件 §5.2](../requirements/functional.md)）。`reason` は swarm-console の対応版（[swarm-console #87](https://github.com/ousiassllc/swarm-console/issues/87)）から返る。無い版でも「取得できません」の 1 行は出る
- 表示規則（バー・しきい値・金額表示）は[機能要件 §5](../requirements/functional.md)

### 1.5 エラーの扱い

swarm-console のエラー本文は HTTP レベルでもツールでも `{"error": {"code", "message", "details"}}`。ツールの失敗は JSON-RPC の `result` に `isError: true` として入る。viewer はエラーを次のように分類し、画面表示は[機能要件 §10](../requirements/functional.md)に従う。

| 検知 | 分類（domain のエラー種別） | 後続の動作 |
|------|---------------------------|-----------|
| 接続タイムアウト・ネットワークエラー（DNS・route なし） | `unreachable` | 次の周期で再試行。直前の表示は残す |
| HTTP 401 `unauthorized` | `unauthorized` | 再ペアリングへ誘導。その接続の自動ポーリング（画面・通知）を止める（無駄な再試行と通知抜けを避ける）。再ペアリングで再開 |
| HTTP 403 `forbidden_scope` | `forbidden_scope` | 同上（`/report` に report 以外のスコープで接続できない状態は、スコープが失効・誤りの可能性） |
| HTTP 404・405・413 | `protocol` | 「接続先がこのアプリと互換ではありません」を表示。ログにステータスコード |
| ツール `session_required` | `session_required` | viewer は常に `session` を明示するため通常は出ない。出たら `sessions_list` を再取得して選択を更新 |
| ツール `session_forbidden` | `session_forbidden` | タブ一覧を再取得（`sessions_list`）して許可外のタブを消す |
| ツール `invalid_args`（`session` の名前が `sessions` にない） | `session_missing` | 同上。`last_session` を先頭の許可セッションにフォールバック |
| ツール `tmux_error`・`internal` | `host_error` | 「ホスト側でエラー」＋ `code`。次の周期で再試行 |
| 上記以外の `isError`・JSON の形が想定外 | `protocol` | ログに `code` のみ（本文は出さない）。直前の表示は残す |

- 401 / 403 の接続は `connections` の状態を DB に持たず、メモリ上のフラグ（`needs_repair`）で保持する。アプリ再起動後は初回の取得で再度判定する
- エラーの `message` は画面には出さない（固定文言に置き換える）。`details` は `session_required` の `allowed_count` などを含むが、画面には出さずログにも本文を出さない（NFR-LOG-3）

### 1.6 MCP ハンドシェイク

公式 Go SDK を使うため、`initialize` → `notifications/initialized` → `tools/call` の手順は SDK が行う。swarm-console はステートレス運用のため、SDK が張るセッション管理（`Mcp-Session-Id`、常駐 GET の SSE、終了時の `DELETE`）が 405 等で拒否される場合の挙動は**実機で確認して確定する**（[アーキテクチャ設計 §11](../architecture/overview.md)の未確認事項 1）。viewer のコードは、`GET` / `DELETE` の 405 を失敗として扱わない。

## 2. ペアリング形式

### 2.1 接続文字列（ディープリンクと同一）

接続文字列は `swarm-viewer://pair` の URL 1 本。貼り付けでもディープリンクでも同じパーサで解析する。

```
swarm-viewer://pair?v=1&url=<URLエンコード>&token=<token>[&name=<表示名>][&sessions=0,1]
```

例:

```
swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=<token>&name=devbox&sessions=0%2C1
```

| パラメータ | 必須 | 内容 | 検証 |
|-----------|------|------|------|
| `v` | 必須 | ペイロード版。現在は `1` | `1` 以外は「アプリを更新してください」と表示して拒否（NFR-CMP-3） |
| `url` | 必須 | 接続先 URL。URL エンコードする | scheme は `http`、host は tailnet 範囲またはループバックの **IP リテラル**、port は `48120`（ループバックのみ任意）、path は `/report`、userinfo・クエリ・フラグメントなし（NFR-SEC-4・5）。末尾スラッシュなし |
| `token` | 必須 | report スコープの token | 空でない。空白・制御文字を含まない |
| `name` | 任意 | 表示名 | 1〜100 文字。無ければ `url` の host で補う |
| `sessions` | 任意 | 許可セッション（カンマ区切り）。**表示のヒントのみ** | 保存しない。確認画面に「ホストが申告した許可セッション」として出すだけ。権限の正は `sessions_list` |

- 全体のサイズ上限は 4,096 バイト。超えたら拒否する
- 同じパラメータが複数あるときは拒否する。未知のパラメータは無視する（NFR-CMP-1）
- 文字列は前後の空白を除去してから解析する。scheme・host は小文字として扱う
- `token` の値は解析後も `url` と一緒にメモリ上の下書きにだけ置く（§4.2）。ログ・画面・URL 履歴に出さない（NFR-SEC-7）

### 2.2 検証失敗時の表示

| 失敗 | 表示 |
|------|------|
| scheme が `swarm-viewer://pair` でない | 「接続文字列の形式が正しくありません」 |
| `v` が `1` でない | 「アプリを更新してください」 |
| `url` が範囲外・形式不正 | 「接続先が許可された範囲外です（Tailscale の IP のみ）」 |
| `token` が空・不正 | 「token が含まれていません」 |
| 重複・サイズ超過 | 「接続文字列の形式が正しくありません」 |
| 疎通確認に失敗 | §1.5 の分類に従った固定文言。**保存しない** |

### 2.3 Android 版（QR・intent-filter）

接続文字列の形式・検証・失敗時の表示（§2.1・§2.2）は Android 版でも変えない。入口だけが増える（[機能要件 §12.3](../requirements/functional.md)）。

| 手段 | 内容 |
|------|------|
| 貼り付け | §2.1 の接続文字列をそのまま貼り付ける |
| QR | **QR の中身は §2.1 の接続文字列そのまま**（`swarm-viewer://pair?v=1&url=…&token=…`。形式・パラメータ・サイズ上限 4,096 バイトは変えない）。読み取った文字列は貼り付けと同じパーサ・同じ検証に入れる。QR を生成する側（`swarm-console pair` の出力か、別の手段か）はこのアプリの範囲外 |
| ディープリンク | `AndroidManifest.xml` の intent-filter で `swarm-viewer` スキームを受ける（下記） |

intent-filter（`MainActivity`。外部に公開するのはこの 1 つだけ。[NFR-AND-SEC-7](../requirements/non-functional.md)）:

```xml
<intent-filter>
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="swarm-viewer" android:host="pair" />
</intent-filter>
```

- 受け取った Intent の `data`（URI 全体）を §2.1 の文字列として `parsePairing` に渡す。scheme・host（`pair`）以外の組み合わせはフィルタで受けない。ほかのアプリや QR 読み取りアプリ・ブラウザからも送れるため、信頼できない入力として扱う
- 解析に成功しても、確認画面で利用者が確定するまで保存しない。確認画面には表示名・接続先のホスト部・ホストが申告した許可セッションだけを出し、**token は出さない**（デスクトップ版 §4.2 の確認画面と同じ）。失敗したときは §2.2 の固定文言を出し、受け取った文字列（token を含みうる）は画面・ログ・履歴に残さない
- 処理が済んだら Intent の `data` を消す（再生成・履歴からの再処理で token が残らないようにする）。アプリが起動済みなら既存の画面を前面に出して処理する（`launchMode` は `singleTask` とし、`onNewIntent` で受ける）
- 通知のタップは別の（アプリ内の）Intent で、接続 ID・セッション ID だけを載せる。接続文字列・token を載せない

## 3. swarm-console への要望（`pair` コマンド）

swarm-console 本体の機能追加は本リポの範囲外。viewer が前提とするインターフェースを次に定める（swarm-console 側の Issue に起票する元になる）。

```
swarm-console pair [--sessions 0,1] [--name <表示名>]
```

| 項目 | 内容 |
|------|------|
| 動作 | `token add <自動名> --scope report [--sessions …]` 相当を内部で実行し、平文 token を **1 回だけ**標準出力に出す |
| `--sessions` | 許可セッション。省略時は `config.toml` の `sessions` 全部（`token add` と同じ） |
| `--name` | 表示名（`name` パラメータにそのまま入れる）。省略時は付けない（viewer が host で補う） |
| 出力 | §2.1 の接続文字列 1 行（`swarm-viewer://pair?v=1&url=…&token=…`）。`url` は `listen_addr` の tailnet IP と `48120`、path `/report` から組み立てる |
| token 名 | swarm-console 側が自動で決める（例: `viewer-<hostname>-<yyyymmdd>`。規則は swarm-console 側の判断で、viewer は依存しない） |
| 有効期限 | 現状のトークンには期限がなく、失効は `token revoke` のみ。viewer は期限を扱わない |
| 注意 | 標準出力に token を含むため、コマンド履歴・端末ログ・チャットに貼らない。再利用せず、紛失時は `token revoke` して `pair` を再実行する |

`pair` が実装されるまでの代替手順: `swarm-console token add <名前> --scope report [--sessions …]` で token を発行し、§2.1 の形式を手で組み立てて貼り付ける（暫定。`url` の host は tailnet IP）。

## 4. 画面ルート（アプリ内 HTTP）

WebView と Go の間の HTTP。Wails のアセットハンドラ経由（in-process）で、TCP ポートを開かない。HALT の分類（ページルート / アクションルート）に従う。JSON API ルート（Huma）は持たない。

### 4.1 共通規約

| 項目 | 内容 |
|------|------|
| ページルート | `HX-Request` ヘッダで分岐する。ヘッダなし（`SetURL` による直接遷移・リロード）= レイアウト込みフルページ、あり = フラグメント |
| アクションルート | 常にフラグメント（または `HX-Redirect` ヘッダ）を返す。ただし、移動先が画面になるルート（`POST /pairing`・`confirm`・`discard`・`/recovery/reset`）は、`HX-Request` のない送信に限り移動先の画面をその場に描画する（下記「リダイレクトを使わない」） |
| リダイレクトを使わない | `http.Redirect`（302・303）は WebKit の WebView（macOS）で動かない（Wails のアセットサーバー `internal/assetserver/asset_fileserver.go` のコメント）。通常モードのルートは 3xx を返さない。ページ遷移（ヘッダなしのフルページ）は解決した画面をその場に 200 で描画し、HX リクエストの移動は `HX-Redirect` ヘッダ（htmx がクライアントで遷移する）で指示する。ポーリング URL・リンクは、要求された URL によらず正規のパス `/connections/{cid}/sessions/{sid}/snapshot` を使う（[アーキテクチャ設計 §11](../architecture/overview.md) #7） |
| ID | `{cid}` = `connections.id`（ULID）、`{sid}` = tmux セッション名（URL エンコード）、`{did}` = ペアリング下書き ID（ランダム 128 bit、base64url） |
| CSRF | 状態変更ルート（`POST`・`PUT`・`DELETE`）は `X-CSRF-Token` を要求する。値はレイアウトが `<body hx-headers='{"X-CSRF-Token":"…"}'>` で配り、HTMX が全リクエストに付ける（JSON 属性のため eval 不要）。プロセス起動ごとに生成。不一致は 403 |
| `Cache-Control` | 全ルート `no-store`（スナップショットがキャッシュされないように） |
| ステータス | アプリ内の取得失敗（swarm-console の不達・認証エラー）は **200** のまま、状態を HTML で表現する（§4.4）。入力検証エラーは 422（フォームを再描画）。不正な `{cid}`・`{did}` は 404 |
| トースト | 保存成功・削除成功は `hx-swap-oob` でトースト atom（1 行の短文）を返す。3 秒で `app.js` が消す。エラー（接続の削除失敗など）はトーストにせず、フラグメント内のインラインアラートで示す（NFR-DSG-5） |
| 文字列 | すべて Templ のエスケープに任せる。`templ.Raw` を使わない（NFR-SEC-10） |

### 4.2 ルート一覧

| メソッド | パス | 種別 | 返却 | 説明 |
|---------|------|------|------|------|
| GET | `/` | ページ | フルページ / フラグメント（200） | 接続が 0 件なら `/pairing` と同じ画面（貼り付けフォーム）。1 件以上なら `/connections/{last}`（`settings.last_connection_id`、なければ先頭）と同じ画面。リダイレクトしない |
| GET | `/pairing` | ページ | フルページ / フラグメント | 接続文字列の貼り付けフォーム。`?link=invalid` を付けると、ディープリンクを解析できなかった旨の固定文言を出す（受け取った文字列は URL にも画面にも載せない。app が失敗時に `SetURL` する） |
| POST | `/pairing` | アクション | `HX-Redirect: /pairing/drafts/{did}`（`HX-Request` なしなら確認画面を描画）/ 422 フォーム | 貼り付けた接続文字列を 1 回だけ受け取り、解析・検証して下書きを作る（§2.1）。フォーム値は破棄し、応答に token を含めない |
| GET | `/pairing/drafts/{did}` | ページ | フルページ | 確認画面。表示: 表示名、接続先ホスト（`url` の host のみ）、ホストが申告した許可セッション。**token は出さない** |
| POST | `/pairing/drafts/{did}/confirm` | アクション | `HX-Redirect: /connections/{cid}`（`HX-Request` なしならメイン画面を描画）/ 200 エラーフラグメント | `sessions_list` で疎通確認（FR-PAIR-2）。成功したら秘密領域に token、SQLite に接続を保存し、下書きを削除する（§4.3）。失敗は保存せず、下書きは残す（再試行可。期限は §4.3） |
| POST | `/pairing/drafts/{did}/discard` | アクション | `HX-Redirect: /pairing` または `/`（`HX-Request` なしならその画面を描画） | 下書きを即時削除する |
| GET | `/connections` | ページ | フルページ / フラグメント | 保存済み接続の一覧。表示名・URL の host のみ・「切替」・「削除」 |
| DELETE | `/connections/{cid}` | アクション | フラグメント（一覧）+ 成功時のみトースト OOB（失敗は一覧の上にインラインアラート） | 接続を削除する（SQLite 行を先に削除し、秘密領域を削除。[ER §4](../architecture/er.md)） |
| GET | `/connections/{cid}` | ページ | フルページ / フラグメント（200） | その接続の `last_session`（なければ先頭の許可セッション）のメイン画面をその場に描画する（リダイレクトしない）。初回描画の規則は下記「メイン画面の初回描画」。許可セッションが 0 件なら「許可されたセッションがありません」ページ |
| GET | `/connections/{cid}/sessions/{sid}` | ページ | フルページ / フラグメント | メイン画面。接続切替・セッションタブ・スナップショット領域を含む。`last_session` を表示するセッションに更新する。ホストとは通信しない（下記「メイン画面の初回描画」）。`{sid}` が許可外と分かっている（直近のタブに無い）ときは先頭の許可セッションを描画する |
| GET | `/connections/{cid}/sessions/{sid}/snapshot` | ページ（部分） | フラグメント | スナップショット領域（要対応バー・ペイングリッド・利用上限・ホスト負荷）。`report` を呼ぶ（同一 `(cid, sid)` は single-flight）。直近の `sessions_list` が無い・古い（30 秒超）ときは並行して引き直し、`#session-tabs` の OOB を返す。画面更新ポーリングと手動更新が叩く |
| GET | `/settings` | ページ | フルページ / フラグメント | 設定画面。OS が通知を許可していないときはその旨を表示する |
| PUT | `/settings` | アクション | フラグメント（設定フォーム）+ トースト OOB / 422 | 設定を保存する。許容値は[ER §3.2](../architecture/er.md)。許容外は 422 でフォームを再描画 |
| GET | `/static/*` | — | 埋め込みアセット | `htmx.min.js`・`app.js`（§4.5 の可視性制御）・`app.css`。外部ホストは参照しない |
| GET / POST | `/`・`/recovery/reset` | ページ / アクション | フルページ / フラグメント | **recovery モードのみ**（DB が開けない・スキーマが新しすぎる。上記の画面ルートは登録しない）。`/` は初期化導線、`POST /recovery/reset` は保存データを退避して作り直し、成功したら `HX-Redirect: /`（`HX-Request` なしなら案内のみ）。それ以外の GET は初期化導線を 200 で描画し、その他のメソッドは 404 |

#### メイン画面の初回描画

`/`・`/connections/{cid}`・`/connections/{cid}/sessions/{sid}` は `sessions_list` や `report` を待たずに描画する。到達不能なホストで初回表示が数十秒遅れないようにするため（FR-POLL-4、NFR-PERF-1・2）。

1. 表示するセッション: `{sid}`（`/connections/{cid}` は `last_session`、無ければ先頭の許可セッション）。直近のタブ（`ConnectionService` のキャッシュ）があり、そこに無く、取得が 30 秒以内なら先頭の許可セッション
2. タブ: 直近のタブ。無ければ表示するセッション 1 件だけ
3. スナップショット領域: 保存済みの直前値をグレーアウトして描画し（無ければ空状態）、`hx-trigger` の `load` で直ちに取得する
4. 手掛かりが全く無い（保存値も `last_session` も直近のタブも無い初回）ときだけ、3 秒の上限付きで `sessions_list` を 1 回呼ぶ。失敗は「接続できません」画面（再試行の導線つき）
5. 最初の `…/snapshot` 応答が `#session-tabs` を OOB で差し替え、タブが最新になる。その後は 30 秒ごとに引き直す

- Go 側から起動するルート遷移（`SetURL`）: ディープリンク受信 → `/pairing/drafts/{did}`（下書き作成後）、通知クリック → `/connections/{cid}/sessions/{sid}`、トレイ「開く」→ 直前の URL（なければ `/`）
- `GET /connections/{cid}/sessions/{sid}` と snapshot は読み取りだけでなく、副作用として `last_session` の更新とスナップショットの保存（`snapshots`）を行う。冪等で、再送しても結果は同じ

### 4.3 ペアリング下書き

| 項目 | 内容 |
|------|------|
| 保管場所 | メモリのみ（SQLite・ディスクに書かない） |
| 内容 | 解析済みの `url`・`token`・`name`・`sessions` |
| 有効期限 | 作成から 5 分。期限切れ・アプリ終了で消える |
| 削除 | 確定（`confirm` 成功）、破棄（`discard`）、期限切れで即削除。`confirm` 失敗時は残す（再試行できる）が、期限は延長しない |
| 同時数 | 1 件のみ。新しい下書きを作ると古い下書きは破棄する |
| 期限切れ・未知の `{did}` | 404 ではなく「もう一度ペアリングしてください」の画面 |

### 4.4 取得失敗の返し方

`/snapshot` が swarm-console から取得できなかったとき:

- 直前のスナップショット（メモリ → なければ `snapshots` の保存分）をグレーアウトして返す。何も無ければ空状態の画面
- 失敗種別（§1.5）に応じた固定文言のバナーを `#snapshot-status` へ OOB で返す。「最終更新 HH:MM」「次回再試行 N 秒後」を併記する
- `unauthorized` / `forbidden_scope`: バナーに「再ペアリング」リンク（`/pairing`）を出し、`hx-trigger` のポーリング属性は出さない（ポーリングを止める。HATEOAS: 取得できない間は取得の導線そのものを出さない）
- `session_forbidden` / `session_missing` / `session_required`: `sessions_list` を引き直し、許可セッションに無ければ `HX-Redirect` で先頭の許可セッション（許可セッションが 0 件なら `/connections/{cid}`）へ。タブに有る・タブを取得できないときは移動せず、バナーのまま次のポーリングで再試行する（同じ失敗の繰り返しを避ける）
- 成功したら通常のフラグメント（ポーリング属性つき）を返し、バナーを消す

### 4.5 HTMX パターン

| 画面 | 属性 | 意味 |
|------|------|------|
| スナップショット領域 | `hx-get=".../snapshot"` `hx-trigger="load, every Ns, refresh from:body"` `hx-sync="this:drop"` | 画面更新ポーリング。`N` は `settings.screen_poll_seconds` をサーバーがレンダリング。前のリクエスト中は破棄（NFR-PERF-5）。**フィルタ式 `[…]` は使わない**（htmx が eval で評価するため、`unsafe-eval` なしの CSP で動かない） |
| 更新ボタン | `hx-get="/connections/{cid}/sessions/{sid}"` `hx-target="#main"` | タブも再取得して全体を再描画（FR-POLL-2） |
| セッションタブ | `hx-get="/connections/{cid}/sessions/{sid}"` `hx-target="#main"` | 切り替え（FR-SESS-2） |
| 接続切替 | `hx-get="/connections/{cid}"` `hx-target="#main"` | 切り替え（`last_session` のメイン画面フラグメントを返す） |
| 非表示中の停止・復帰時の即時取得 | 同梱の静的 `app.js`（下記） | フィルタ式の代わりにクライアントで制御する |
| 設定フォーム | `hx-put="/settings"` `hx-target="#settings-form"` `hx-target-422="#settings-form"` | `response-targets` 拡張で 422 のとき再描画先を分ける |
| 接続削除 | `hx-delete="/connections/{cid}"` `hx-confirm` | アクション |
| 共通 | `hx-disabled-elt="this"`、`hx-indicator` | 二重送信防止・ローディング表示。Optimistic UI とスケルトンは使わない |

`static/dist/js/app.js`（外部ファイル。CSP の `script-src 'self'` で許可される。インラインではない）は次の 3 つだけを行う。

- `htmx:beforeRequest` で、`document.hidden` のあいだ `…/snapshot` へのリクエストを `preventDefault()` で取り消す（FR-POLL-3）
- `visibilitychange` で `document.hidden` が `false` になったら `htmx.trigger(document.body, 'refresh')` を発火し、即時取得する（NFR-AVL-4）
- `htmx:oobAfterSwap` で `#toasts` が差し替わったら、3 秒後に `#toasts` の中身を空にする（NFR-DSG-5）

HTMX の設定は `<meta name="htmx-config" content='{"allowEval":false,"includeIndicatorStyles":false}'>` をレイアウトの `<head>` に置く。`allowEval:false` で eval を使う機能（トリガーフィルタ、`hx-on:`・`js:` 構文）を無効にし、`includeIndicatorStyles:false` で htmx が注入するインライン `<style>` を止める（`style-src 'self'` に反するため）。`.htmx-indicator` の表示切替は `app.css`（Tailwind）に書く。

- Lit コンポーネントは使わないため、HTMX ↔ Lit 境界ルールは適用しない（[アーキテクチャ設計 §8](../architecture/overview.md)）
- トーストの 3 秒消去（`htmx:oobAfterSwap` で `#toasts` を空にする）だけをクライアントで処理する。通信障害（`htmx:responseError`）の汎用表示は持たない（アプリ内通信のため通常は起きない）

## 5. 変更の影響範囲

| 変更 | 影響 |
|------|------|
| swarm-console の `report` に新しい `state`・`attention.kind` が追加された | 未知値は `unknown`・生文字列として表示（NFR-CMP-1）。表示ラベルの追加が必要になる |
| ペイロード版 `v` の更新 | §2.1・§2.2 を更新し、パーサを版で分岐する。旧版アプリは「アプリを更新してください」を表示する |
| swarm-console が `/ops` 専用だったツールを `/report` に追加 | viewer は許可リストにあるツールしか呼ばないため影響なし |

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|----|------|---------|---------|
| 1.0 | 2026-11-03 | 新規作成 | 初版 |
| 1.1 | 2026-11-03 | トリガーフィルタ `[document.visibilityState…]` を廃止し、静的 `app.js` で非表示中の停止・復帰時の即時取得を行う方式に変更。CSRF 配布を `hx-headers` に訂正。htmx-config を明記 | CSP（`unsafe-eval` なし）で htmx のフィルタ式が動かないため |
| 1.2 | 2026-11-04 | 通常モードのルートから 302・303 を撤去（フルページはその場に描画、HX は `HX-Redirect`）。メイン画面の初回描画をホストへの通信なしにし、タブを snapshot 応答の OOB で更新する | WebKit の WebView が `http.Redirect` を扱えないため。到達不能なホストで初回表示が遅れるため |
| 1.3 | 2026-11-05 | `usage.claude[]`・`codex[]` に `reason`（2 枠とも取れない理由）を追加し、2 枠とも `available: false` は 1 行表示と明記 | Issue #11（swarm-console #87 の `reason` に備える） |
| 1.4 | 2026-11-05 | `report` の `host.mem` を読むキーに追加（無い古い swarm-console では「—」） | Issue #10（swarm-console #86 の `host.mem` に備える） |
