# API 仕様

swarm-viewer が関わる API は 4 種類ある。viewer 自身は**外部に公開する API を持たない**（TCP ポートを開かない。[アーキテクチャ設計 §1](../architecture/overview.md)）。

| 種別 | 向き | 内容 | 節 |
|------|------|------|----|
| swarm-console `/report`・`/ops` の利用 | viewer → swarm-console | MCP `tools/call`。見るだけ（`/report`）は `sessions_list`・`report`・`pane_tail`・`ask_card`、操作（`/ops`）はそれに `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list` を足した 10 個 | §1 |
| ペアリング形式 | swarm-console → viewer | 接続文字列 / `swarm-viewer://` ディープリンク（`url` の path で見るだけ / 操作を見分ける） | §2 |
| swarm-console の `pair` コマンド | — | `swarm-console pair [--scope report\|ops]`（swarm-console 側が実装。viewer が前提とする出力） | §3 |
| 画面ルート（アプリ内 HTTP） | WebView → Go | Gin のルート。HTMX が HTML フラグメントを取得 | §4 |

Huma（OpenAPI）で登録する JSON オペレーションは **v1 では 0 本**（[アーキテクチャ設計 §4.3](../architecture/overview.md)）。

## 1. swarm-console `/report`・`/ops` の利用

接続先の仕様の正本は swarm-console の `docs/api/endpoints.md`（v0.1.11 以降）。ここでは viewer が使う部分だけを記す。

### 1.1 接続

接続の種類は、接続の `url` の path で決まる（保存データに種類の列は持たない。ER §3.1）。

| 種類 | `url` の path | 必要な token の scope | できること |
|------|--------------|----------------------|-----------|
| 見るだけ | `/report` | `report`（`ops` の token も通る） | 状態の閲覧、ペインの中身（`pane_tail`）と質問（`ask_card`）の閲覧 |
| 操作 | `/ops` | `ops`（`report` の token は 403 `forbidden_scope`） | 上に加えて、質問への回答・送信・中断・起動・終了・フォルダ一覧 |

| 項目 | 値 |
|------|----|
| プロトコル | MCP（Streamable HTTP、JSON-RPC 2.0）。クライアントは公式 Go SDK（`github.com/modelcontextprotocol/go-sdk/mcp`） |
| URL | 接続の `url`（`http://<tailnet-ip>:48120/report` または `…/ops`）。**path は `/report` か `/ops` ちょうどだけ**（NFR-SEC-5）。見るだけの接続は `/ops` を呼ばない（NFR-SEC-1・NFR-SEC-14） |
| メソッド | `POST` のみ（swarm-console は `GET`・`DELETE` を 405 で拒否するステートレス運用） |
| ヘッダ | `Authorization: Bearer <token>`、`Content-Type: application/json`、`Accept: application/json, text/event-stream`（SDK が付与する Bearer は `http.Client` の Transport で付ける） |
| タイムアウト | ツールごとの上限（接続・応答の合計。NFR-PERF-4）。読み取り（`sessions_list`・`report`・`pane_tail`・`ask_card`・`dir_list`）は 10 秒、`ask_pick`・`send_text`・`send_escape` は 10 秒 × 3 = 30 秒、`pane_start`・`pane_close` は 10 秒 × 5 = 50 秒（§1.2） |
| 本文上限 | リクエストは 256 KiB 未満（swarm-console 側の上限。viewer の引数は小さいため問題にならない） |
| リトライ | しない（次の周期で再取得する。NFR-AVL-2） |

### 1.2 呼び出すツール（種類別の許可リスト）

viewer が呼ぶツールは、接続の種類ごとの許可リストにあるものだけ。許可リストは `internal/repository/report`（Android は `data/report`）の種類別の定数で、それ以外のツール名はコードから呼べない（NFR-SEC-1・NFR-AND-SEC-5）。種類は接続の `url` の path から決める。

| 接続の種類 | 呼べるツール |
|-----------|-------------|
| 見るだけ（`/report`） | `sessions_list`・`report`・`pane_tail`・`ask_card`（4 つ） |
| 操作（`/ops`） | 上の 4 つ ＋ `ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list`（計 10 個） |

**ツール別のタイムアウト**: 操作ツールは swarm-console が画面の確認を待ってから応答する（`accept_timeout` 既定 10 秒・`start_timeout` 既定 30 秒。`pane_start`・`pane_close` は複数の手順を踏む）。viewer の上限が 10 秒のままだと、swarm-console 側の待ちより先に切れてしまうため、swarm-console に待つ余地を残すよう通信の上限（基準 10 秒）に倍率を掛ける: `ask_pick`・`send_text`・`send_escape` は 3 倍（30 秒）、`pane_start`・`pane_close` は 5 倍（50 秒）、読み取り系は 1 倍（10 秒）。倍率は Go の `internal/repository/report/tools.go`（`toolTimeoutScale`）と Kotlin の `ReportTools.timeoutScale` の定数で、2 つの実装で同じ。リトライはしない

| ツール | 種類 | 呼ぶ場面 | 引数 |
|--------|------|---------|------|
| `sessions_list` | 両方 | 接続の疎通確認（ペアリング）、メイン画面の描画（タブの取得）、通知用ポーリング | `{}`（`all` は指定しない。許可セッションのみ受け取る） |
| `report` | 両方 | 画面更新ポーリング、通知用ポーリング、ペイン詳細の状態の取り直し | `{"format": "json", "session": "<id>"}`（`lang`・`pane` は指定しない） |
| `pane_tail` | 両方 | ペイン詳細（機能要件 §4.5）。開くとき・更新・5 秒ごとの自動更新 | `{"session": "<id>", "pane": "<pos>", "lines": 30}`（`lines` は既定 30・最大 200。`raw` は付けない） |
| `ask_card` | 両方 | ペイン詳細で、`report` の状態が `asking` のときだけ | `{"session": "<id>", "pane": "<pos>"}` |
| `ask_pick` | 操作のみ | 質問カードの選択肢のボタン（押したらすぐ）・「その他」の送信 | `{"session", "pane", "n": 1}` または `{"session", "pane", "text": "…"}`（同時に指定しない。`text` は 8000 字まで） |
| `send_text` | 操作のみ | 指示の送信 | `{"session", "pane", "text": "…"}`（1〜8000 字。許可する制御文字は `\n`・`\t` だけ。`file` は使わない） |
| `send_escape` | 操作のみ | 確認ダイアログで確定した「中断」 | `{"session", "pane", "confirm": true}`（確認済みでなければ呼ばない。NFR-SEC-12） |
| `pane_start` | 操作のみ | 新しいセッション（機能要件 §4.8）の確定 | `{"session", "pane", "agent": "omp\|claude\|codex", "cwd": "<絶対パス>", "title": "…"}`（`title` は 1〜80 字） |
| `pane_close` | 操作のみ | 確認ダイアログで確定した「終了」 | `{"session", "pane"}` |
| `dir_list` | 操作のみ | フォルダピッカー（機能要件 §4.8） | `{"path": "<絶対パス>"}`（`path` 省略 = `allowed_roots`）。`session` は取らない。`limit` は指定しない |

- `session` は**常に明示する**（`report`・`pane_tail`・`ask_card`・`ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`。`dir_list` を除く）。許可セッションが 2 つ以上のトークンで省略すると `session_required` になるため、許可数に依存しない動作にする
- `pane` は `report` の `panes[].pos`（セッション内の位置。接頭辞なし）を使う
- 見るだけの接続で操作系ツール（`ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list`）を呼ぼうとしたときは、**通信せずに** `tool_not_allowed`（domain の `FetchError{Kind: protocol, Code: "tool_not_allowed"}`）で断る。送信内容の検査（Go は `roundTripper`、Android は `GuardInterceptor`）も、URL の path が `/report` なら 4 つ、`/ops` なら 10 個だけを通す（呼び出し口と合わせた二重の検査）
- 呼ばないツール: `dispatch`・`note_set`・`note_log`・`file_get`（範囲外。機能要件 §11）、`panes_status`・`host_status`・`report_read`（`report` の JSON に必要な値がすべて入っているため）、`tools/list`（許可リストを viewer 側で固定する）

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
| `sessions[].operable` | 使わない（操作できるかは接続の種類で決める） |

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
| HTTP 403 `forbidden_scope` | `forbidden_scope` | 同上（`/report` に report 以外のスコープで接続できない状態、または `/ops` に ops 以外の token（report スコープ）で接続した状態。スコープが失効・誤りの可能性）。操作の接続でも、その接続のポーリングと操作を止める |
| HTTP 404・405・413 | `protocol` | 「接続先がこのアプリと互換ではありません」を表示。ログにステータスコード |
| ツール `session_required` | `session_required` | viewer は常に `session` を明示するため通常は出ない。出たら `sessions_list` を再取得して選択を更新 |
| ツール `session_forbidden` | `session_forbidden` | タブ一覧を再取得（`sessions_list`）して許可外のタブを消す |
| ツール `invalid_args`（`details.field` が `"session"`。`session` の名前が `sessions` にない） | `session_missing` | 同上。`last_session` を先頭の許可セッションにフォールバック。`pane_tail`・`ask_card` と操作系ツールの `invalid_args` は、`details.field == "session"` のときだけ `session_missing`、それ以外（`pane`・`n`・`text` など）は `operation`（§1.5.1）。Go（`ops.Failure`）と Kotlin で同じ |
| ツール `tmux_error`・`internal` | `host_error` | 「ホスト側でエラー」＋ `code`。次の周期で再試行 |
| 操作ツールの `isError`（`invalid_state`・`not_asking`・`pane_not_found` など。§1.5.1 の一覧） | `operation`（`Code` を保つ） | 画面は固定文言。コードごとの動作は §1.5.1 |
| 見るだけの接続に対する操作系ツールの呼び出し（通信しない） | `protocol`（`Code: "tool_not_allowed"`） | 画面は「この接続では操作できません（見るだけの接続です）」。通常は UI に操作部品を出さないため起きない（NFR-SEC-14） |
| 上記以外の `isError`・JSON の形が想定外 | `protocol` | ログに `code` のみ（本文は出さない）。直前の表示は残す |

- 401 / 403 の接続は `connections` の状態を DB に持たず、メモリ上のフラグ（`needs_repair`）で保持する。アプリ再起動後は初回の取得で再度判定する
- エラーの `message` は画面には出さない（固定文言に置き換える）。`details` は `session_required` の `allowed_count` などを含むが、画面には出さずログにも本文を出さない（NFR-LOG-3）

### 1.5.1 操作ツールのエラー（`operation`）

操作ツールの失敗（HTTP 200 の `isError`）のうち、次のコード（`[a-z0-9_]{1,64}` に合うもの）は `operation`（`FetchError.Kind`）に分類し、`Code` を保つ。画面にはホストの `message` を出さず、[機能要件 §10.1](../requirements/functional.md) の固定文言を出す。

| `code` | 主に出るツール | 後続の動作 |
|--------|---------------|-----------|
| `invalid_state` | 操作系全部 | 状態を取り直す。ペインの状態が変わった |
| `not_asking`・`option_out_of_range`・`cursor_mismatch` | `ask_pick` | `ask_card` を取り直して最新の質問を出す（`cursor_mismatch` は何も送っていない） |
| `custom_not_allowed` | `ask_pick` | 「その他」を出さない（`allows_custom` を見る） |
| `confirm_required` | `send_escape`・`pane_close` | viewer（`PaneService`・`POST P/escape`・`POST P/close`）は確認なしでは通信しない。通信前の拒否にも同じコードを使う（`pane_close` も確認済みでなければ呼ばない）。ホストから返ったら不具合 |
| `forbidden_control_char`・`text_too_long`・`file_unreadable` | `send_text`・`ask_pick` | 入力欄に固定文言。入力は残す |
| `forbidden_path`・`path_forbidden`・`not_found` | `pane_start`・`dir_list` | フォルダの選び直し |
| `unsupported_agent` | `ask_card`・操作系 | `unknown` のペイン。`pane_tail` だけ出す |
| `pane_not_found` | `pane` を取る全部 | 画面を更新させる |
| `busy_timeout` | 操作系全部 | しばらくしてから再試行 |
| `exit_timeout` | `pane_close` | 「もう一度試す」。`pane_close` は呼び直してよい |
| `unknown_tool` | 操作系 | ホストの swarm-console が古い。固定文言 |
| `invalid_args` | 全部 | `details.field == "session"`（`session` の名前が `sessions` にない）のときだけ従来どおり `session_missing`（§1.5）。それ以外（`pane`・入力の不正）は `operation`（`ops.Failure`。Kotlin も同じ） |

- 上の一覧にないコードは従来どおり `protocol`。`session_forbidden`・`session_required`・`tmux_error`・`internal`・HTTP 401・403・404・405・413 は §1.5 のとおり（401・403 は操作の接続でもその接続のポーリングと操作を止める）
- `ok: false`・`accepted: false`・`card_closed: false`（エラーではない「送ったが期待した状態にならなかった」）は、エラーにせず結果のトーストで知らせる（機能要件 §10.1）

### 1.5.2 ペイン・操作ツールの結果

`content[0].text` の JSON から読む。未知のキーは無視し、形が想定外なら `protocol`（NFR-CMP-1）。

| ツール | 読むキー |
|--------|---------|
| `pane_tail` | `text`（文字列）・`truncated`（bool） |
| `ask_card` | `asking`・`question`・`options[]{n, label, recommended}`・`cursor`・`allows_custom` |
| `ask_pick` | `ok`・`state`・`picked{n, label}`・`card_closed` |
| `send_text` | `ok`・`state`・`accepted`・`chars` |
| `send_escape` | `ok`・`state`・`interrupted` |
| `pane_start` | `ok`・`state`・`agent`・`cwd`・`title` |
| `pane_close` | `ok`・`state`・`note_archived`（`steps` は読まない） |
| `dir_list` | `path`・`entries[]{name, path, git}`・`truncated` |

### 1.6 MCP ハンドシェイク

公式 Go SDK を使うため、`initialize` → `notifications/initialized` → `tools/call` の手順は SDK が行う。swarm-console はステートレス運用のため、SDK が張るセッション管理（`Mcp-Session-Id`、常駐 GET の SSE、終了時の `DELETE`）が 405 等で拒否される場合の挙動は**実機で確認して確定する**（[アーキテクチャ設計 §11](../architecture/overview.md)の未確認事項 1）。viewer のコードは、`GET` / `DELETE` の 405 を失敗として扱わない。

## 2. ペアリング形式

### 2.1 接続文字列（ディープリンクと同一）

接続文字列は `swarm-viewer://pair` の URL 1 本。貼り付けでもディープリンクでも同じパーサで解析する。

```
swarm-viewer://pair?v=1&url=<URLエンコード>&token=<token>[&name=<表示名>][&sessions=0,1]
```

例（見るだけ・操作）:

```
swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=<token>&name=devbox&sessions=0%2C1
swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Fops&token=<token>&name=devbox-ops&sessions=0%2C1
```

| パラメータ | 必須 | 内容 | 検証 |
|-----------|------|------|------|
| `v` | 必須 | ペイロード版。現在は `1` | `1` 以外は「アプリを更新してください」と表示して拒否（NFR-CMP-3） |
| `url` | 必須 | 接続先 URL。URL エンコードする。**path で接続の種類を決める**（`/report` = 見るだけ、`/ops` = 操作。形式は `--scope` によらず `v=1` のまま） | scheme は `http`、host は tailnet 範囲またはループバックの **IP リテラル**、port は `48120`（ループバックのみ任意）、path は `/report` または `/ops` ちょうど、userinfo・クエリ・フラグメントなし（NFR-SEC-4・5）。末尾スラッシュ・別表記は拒否 |
| `token` | 必須 | `/report` なら report スコープ、`/ops` なら ops スコープの token | 空でない。空白・制御文字を含まない |
| `name` | 任意 | 表示名 | 1〜100 文字。無ければ `url` の host で補う |
| `sessions` | 任意 | 許可セッション（カンマ区切り）。**表示のヒントのみ** | 保存しない。確認画面に「ホストが申告した許可セッション」として出すだけ。権限の正は `sessions_list` |

- 全体のサイズ上限は 4,096 バイト。超えたら拒否する
- 同じパラメータが複数あるときは拒否する。未知のパラメータは無視する（NFR-CMP-1）
- 文字列は前後の空白を除去してから解析する。scheme・host は小文字として扱う
- `token` の値は解析後も `url` と一緒にメモリ上の下書きにだけ置く（§4.2）。ログ・画面・URL 履歴に出さない（NFR-SEC-7）
- 確認画面は、操作の接続（`/ops`）のときだけ警告を出す（[機能要件 §4.2](../requirements/functional.md)・NFR-SEC-13）。token の漏えい・紛失時は、ホストで `swarm-console token revoke <名前>` を実行して失効する

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

## 3. swarm-console の `pair` コマンド

`swarm-console pair` は swarm-console 側が実装している（swarm-console の `docs/api/endpoints.md` §2、FR-PAIR-1〜7）。viewer が前提とするインターフェースを次に定める。

```
swarm-console pair [--scope report|ops] [--sessions 0,1] [--name <表示名>]
```

| 項目 | 内容 |
|------|------|
| 動作 | `token add <自動名> --scope <scope> [--sessions …]` 相当を内部で実行し、平文 token を **1 回だけ**標準出力に出す |
| `--scope` | `report`（既定。見るだけ）または `ops`（操作）。それ以外の値は使い方の誤りで、token を発行しない |
| `--sessions` | 許可セッション。省略時は `config.toml` の `sessions` 全部（`token add` と同じ） |
| `--name` | 表示名（`name` パラメータにそのまま入れる）。省略時は付けない（viewer が host で補う） |
| 出力 | §2.1 の接続文字列 1 行（`swarm-viewer://pair?v=1&url=…&token=…`。形式は `--scope` によらず `v=1`）。`url` は `listen_addr` の tailnet IP と `48120`、path（`report` なら `/report`、`ops` なら `/ops`）から組み立てる |
| token 名 | swarm-console 側が自動で決める。`report` は `viewer-<ホスト名>-<yyyymmdd>`、`ops` は `viewer-ops-<ホスト名>-<yyyymmdd>`（重なれば `-2`・`-3`…）。viewer は名前の規則に依存しない（失効のときに利用者が名前を探せればよい。`swarm-console token list` で確かめられる） |
| 有効期限 | 現状のトークンには期限がなく、失効は `token revoke` のみ。viewer は期限を扱わない |
| 注意 | 標準出力に token を含むため、コマンド履歴・端末ログ・チャットに貼らない。再利用せず、紛失時は `token revoke <名前>` して `pair` を再実行する。`--scope ops` の token はペインへの送信・中断・起動・終了ができるため、スマホを紛失したらすぐ `swarm-console token revoke <名前>` する（NFR-SEC-13） |

`pair` を使わない手順: `swarm-console token add <名前> --scope report|ops [--sessions …]` で token を発行し、§2.1 の形式を手で組み立てて貼り付ける（`url` の host は tailnet IP、path は scope に合わせて `/report` か `/ops`）。

## 4. 画面ルート（アプリ内 HTTP）

WebView と Go の間の HTTP。Wails のアセットハンドラ経由（in-process）で、TCP ポートを開かない。HALT の分類（ページルート / アクションルート）に従う。JSON API ルート（Huma）は持たない。

### 4.1 共通規約

| 項目 | 内容 |
|------|------|
| ページルート | `HX-Request` ヘッダで分岐する。ヘッダなし（`SetURL` による直接遷移・リロード）= レイアウト込みフルページ、あり = フラグメント |
| アクションルート | 常にフラグメント（または `HX-Redirect` ヘッダ）を返す。ただし、移動先が画面になるルート（`POST /pairing`・`confirm`・`discard`・`/recovery/reset`）は、`HX-Request` のない送信に限り移動先の画面をその場に描画する（下記「リダイレクトを使わない」） |
| リダイレクトを使わない | `http.Redirect`（302・303）は WebKit の WebView（macOS）で動かない（Wails のアセットサーバー `internal/assetserver/asset_fileserver.go` のコメント）。通常モードのルートは 3xx を返さない。ページ遷移（ヘッダなしのフルページ）は解決した画面をその場に 200 で描画し、HX リクエストの移動は `HX-Redirect` ヘッダ（htmx がクライアントで遷移する）で指示する。ポーリング URL・リンクは、要求された URL によらず正規のパス `/connections/{cid}/sessions/{sid}/snapshot` を使う（[アーキテクチャ設計 §11](../architecture/overview.md) #7） |
| ID | `{cid}` = `connections.id`（ULID）、`{sid}` = tmux セッション名（URL エンコード）、`{pos}` = ペインの位置（`report` の `panes[].pos`。URL エンコード）、`{did}` = ペアリング下書き ID（ランダム 128 bit、base64url） |
| CSRF | 状態変更ルート（`POST`・`PUT`・`DELETE`）は `X-CSRF-Token` を要求する。値はレイアウトが `<body hx-headers='{"X-CSRF-Token":"…"}'>` で配り、HTMX が全リクエストに付ける（JSON 属性のため eval 不要）。プロセス起動ごとに生成。不一致は 403 |
| `Cache-Control` | 全ルート `no-store`（スナップショットがキャッシュされないように） |
| ステータス | アプリ内の取得失敗（swarm-console の不達・認証エラー）は **200** のまま、状態を HTML で表現する（§4.4）。入力検証エラーは 422（フォームを再描画）。不正な `{cid}`・`{did}` は 404。**見るだけの接続で操作系のルート（§4.2 の `pick`・`send`・`escape`・`close`・`start`）を呼ぶと 403**（コンソールには通信せず、「この接続では操作できません（見るだけの接続です）」のフラグメントを返す。§4.6） |
| トースト | 保存成功・削除成功・操作の結果（機能要件 §10.1）は `hx-swap-oob` でトースト atom（1 行の短文）を返す。3 秒で `app.js` が消す。エラー（接続の削除失敗・ツールのエラーなど）はトーストにせず、フラグメント内のインラインアラートで示す（NFR-DSG-5） |
| 文字列 | すべて Templ のエスケープに任せる。`templ.Raw` を使わない（NFR-SEC-10） |

### 4.2 ルート一覧

| メソッド | パス | 種別 | 返却 | 説明 |
|---------|------|------|------|------|
| GET | `/` | ページ | フルページ / フラグメント（200） | 接続が 0 件なら `/pairing` と同じ画面（貼り付けフォーム）。1 件以上なら `/connections/{last}`（`settings.last_connection_id`、なければ先頭）と同じ画面。リダイレクトしない |
| GET | `/pairing` | ページ | フルページ / フラグメント | 接続文字列の貼り付けフォーム。`?link=invalid` を付けると、ディープリンクを解析できなかった旨の固定文言を出す（受け取った文字列は URL にも画面にも載せない。app が失敗時に `SetURL` する） |
| POST | `/pairing` | アクション | `HX-Redirect: /pairing/drafts/{did}`（`HX-Request` なしなら確認画面を描画）/ 422 フォーム | 貼り付けた接続文字列を 1 回だけ受け取り、解析・検証して下書きを作る（§2.1）。フォーム値は破棄し、応答に token を含めない |
| GET | `/pairing/drafts/{did}` | ページ | フルページ | 確認。貼り付け画面の上に中央モーダル（`<dialog>`）で開く。表示: 表示名、接続先ホスト（`url` の host のみ）、ホストが申告した許可セッション。**token は出さない**。外側クリック・Esc は「破棄」（`POST …/discard`）と同じ。保存するまで何も保存しない。`/pairing/drafts/{did}` はディープリンク・貼り付けの移動先として残す |
| POST | `/pairing/drafts/{did}/confirm` | アクション | `HX-Redirect: /connections/{cid}`（`HX-Request` なしならメイン画面を描画）/ 200 エラーフラグメント | `sessions_list` で疎通確認（FR-PAIR-2）。成功したら秘密領域に token、SQLite に接続を保存し、下書きを削除する（§4.3）。失敗は保存せず、下書きは残す（再試行可。期限は §4.3） |
| POST | `/pairing/drafts/{did}/discard` | アクション | `HX-Redirect: /pairing` または `/`（`HX-Request` なしならその画面を描画） | 下書きを即時削除する |
| GET | `/connections` | ページ | フルページ / フラグメント | 保存済み接続の一覧。表示名・URL の host のみ・「切替」・「削除」 |
| DELETE | `/connections/{cid}` | アクション | フラグメント（一覧）+ 成功時のみトースト OOB（失敗は一覧の上にインラインアラート） | 接続を削除する（SQLite 行を先に削除し、秘密領域を削除。[ER §4](../architecture/er.md)） |
| GET | `/connections/{cid}` | ページ | フルページ / フラグメント（200） | その接続の `last_session`（なければ先頭の許可セッション）のメイン画面をその場に描画する（リダイレクトしない）。初回描画の規則は下記「メイン画面の初回描画」。許可セッションが 0 件なら「許可されたセッションがありません」ページ |
| GET | `/connections/{cid}/sessions/{sid}` | ページ | フルページ / フラグメント | メイン画面。接続切替・セッションタブ・スナップショット領域を含む。`last_session` を表示するセッションに更新する。ホストとは通信しない（下記「メイン画面の初回描画」）。`{sid}` が許可外と分かっている（直近のタブに無い）ときは先頭の許可セッションを描画する |
| GET | `/connections/{cid}/sessions/{sid}/snapshot` | ページ（部分） | フラグメント | スナップショット領域（要対応バー・ペイングリッド・利用上限・ホスト負荷）。`report` を呼ぶ（同一 `(cid, sid)` は single-flight）。直近の `sessions_list` が無い・古い（30 秒超）ときは並行して引き直し、`#session-tabs` の OOB を返す。画面更新ポーリングと手動更新が叩く |
| GET | `/connections/{cid}/sessions/{sid}/panes/{pos}` | ページ | フルページ / フラグメント | ペイン詳細（S4）。以降のルートの `P` はこのパス。ヘッダ（位置・状態・タイトル・`cwd`・メモ・種類のバッジ）と、`P/live` を `hx-trigger="load, every 5s, refresh from:body"` で読む本文領域。見るだけの接続でも開ける |
| GET | `P/live?lines=30\|100\|200&first=1` | ページ（部分） | フラグメント | 状態ヘッダ・質問カード（`asking` のとき `ask_card`）・`pane_tail`（`lines` は 30・100・200 のどれか。それ以外・省略は 30）と、状態に応じた送信欄・中断・終了（操作の接続のみ）。`report`（single-flight）・`pane_tail`・`ask_card` を呼ぶ。見るだけの接続は操作部品を含まない。5 秒ごとの自動更新と手動更新が叩く。`first=1` は最初の読み込み（`P` の本文領域の `load`）で、取得に失敗したときもバナー付きの本文を返す。`first` が無い（自動更新・更新ボタン）失敗は、`HX-Reswap: none` とバナーの OOB だけを返し、直前の本文を残す（401・403 は自動更新の属性を外した本文） |
| POST | `P/pick` | アクション | フラグメント + トースト OOB | `n` または `text`（同時指定しない）、`lines`（取り直す `pane_tail` の行数）。`ask_pick`。**操作の接続のみ** |
| POST | `P/send` | アクション | フラグメント + トースト OOB | `text`、`lines`。`send_text`。受理されたとき入力欄を空にする。**操作の接続のみ** |
| POST | `P/escape` | アクション | フラグメント + トースト OOB | `confirm=true`（確認ダイアログで確定したフォームだけが付ける。無ければ `send_escape` を呼ばず `confirm_required`）、`lines`。**操作の接続のみ** |
| POST | `P/close` | アクション | `HX-Redirect: /connections/{cid}/sessions/{sid}`（成功）/ フラグメント | `confirm=true`（確認ダイアログで確定したフォームだけが付ける。無ければ `pane_close` を呼ばず `confirm_required`。`escape` と同じ）、`lines`。送る前に最新の `report` でペインの状態を再検査し、`idle`・`shell` 以外は通信せず `invalid_state`。成功はトーストなしでメイン画面へ。`exit_timeout` は「もう一度試す」付きの表示。**操作の接続のみ** |
| GET | `P/start` | ページ | フルページ / フラグメント | 新しいセッションの入力画面（S5。エージェント・フォルダピッカー・タイトル）。**操作の接続のみ**。`shell` のペインだけ（それ以外・`report` が取れないときは、入力画面でなく固定文言の案内） |
| GET | `P/start/dirs?path=&pick=1` | ページ（部分） | フラグメント | フォルダピッカー（`dir_list`。`path` 省略 = `allowed_roots`。省略のときは最近使ったフォルダ（`report` の `agent` があるペインの `cwd`、最大 5 件）も出す）。`pick=1` かつ `path` ありは「このフォルダを選ぶ」で、通信せずに選択済みの表示を返す（許可の判定は `pane_start` が行う）。**操作の接続のみ** |
| POST | `P/start` | アクション | フラグメント / `HX-Redirect: P`（成功） | `agent`・`cwd`・`title`。2 段階: `edit=1` は入力画面へ戻す、`confirm=1` が無ければ入力を検査して確認画面を返すだけ（`pane_start` は呼ばない。不正な入力は入力画面に固定文言）、`confirm=1` で `pane_start`。`confirm=1` でも送る前に最新の `report` でペインが `shell` であることを再検査し、違えば通信せず `invalid_state`。成功はトーストなしでペイン詳細へ。失敗（`ok: false`・エラー）は確認画面に固定文言。**操作の接続のみ** |
| GET | `/settings` | ページ | フルページ / フラグメント | 設定画面。OS が通知を許可していないときはその旨を表示する |
| PUT | `/settings` | アクション | フラグメント（設定フォーム）+ トースト OOB / 422 | 設定を保存する。許容値は[ER §3.2](../architecture/er.md)。許容外は 422 でフォームを再描画 |
| GET | `/static/*` | — | 埋め込みアセット | `htmx.min.js`・`app.js`（§4.5 の可視性制御）・`modal.js`（確認ダイアログとモーダル）・`app.css`。外部ホストは参照しない |
| GET / POST | `/`・`/recovery/reset` | ページ / アクション | フルページ / フラグメント | **recovery モードのみ**（DB が開けない・スキーマが新しすぎる。上記の画面ルートは登録しない）。`/` は初期化導線、`POST /recovery/reset` は保存データを退避して作り直し、成功したら `HX-Redirect: /`（`HX-Request` なしなら案内のみ）。それ以外の GET は初期化導線を 200 で描画し、その他のメソッドは 404 |

ペイン詳細・操作のルートの `P` は `/connections/{cid}/sessions/{sid}/panes/{pos}`。操作系（`pick`・`send`・`escape`・`close`・`start` の各ルート）は、接続の種類が操作でなければ 403 で断る（§4.6）。

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
| `#main` へ描画するリンク（ペイン詳細・新しいセッション・再ペアリングなど） | `hx-get` `hx-target="#main"` `hx-swap="innerHTML"` | `hx-swap` は祖先から継承される。ポーリング領域（`#snapshot`・`#pane-live`）やフォーム（`outerHTML`）の中のリンクは `innerHTML` を自分で持たせる（持たないと `<main id="main">` ごと差し替わり、以降の「メイン画面へ」が `htmx:targetError` で効かなくなる。Issue #100） |
| 接続切替 | `hx-get="/connections/{cid}"` `hx-target="#main"` | 切り替え（`last_session` のメイン画面フラグメントを返す） |
| 非表示中の停止・復帰時の即時取得 | 同梱の静的 `app.js`（下記） | フィルタ式の代わりにクライアントで制御する |
| 設定フォーム | `hx-put="/settings"` `hx-target="#settings-form"` `hx-target-422="#settings-form"` | `response-targets` 拡張で 422 のとき再描画先を分ける |
| 接続削除 | `hx-delete="/connections/{cid}"` `hx-confirm` | アクション |
| ペイン詳細の本文 | `hx-get="P/live"` `hx-trigger="load, every 5s, refresh from:body"` `hx-sync="this:drop"` | 開いている間だけ 5 秒ごとの自動更新（NFR-PERF-8）。非表示中、および確認ダイアログが開いている間は `app.js` が取り消す。フィルタ式は使わない。本文領域（`#pane-live`）ごと差し替える。「その他（自分で書く）」と指示の入力は `hx-preserve` で、再描画で入力途中の文字が消えない（送信が受理されたときだけ空にする） |
| 選択肢のボタン・「その他」の送信 | `hx-post="P/pick"` `hx-vals='{"n":"1","lines":"30"}'`（または `text` を含むフォーム）`hx-sync="#pane-live:replace"` `hx-disabled-elt="#pane-live button, #pane-live textarea"` | 押したらすぐ送る。同じペインの全ボタンを `hx-disabled-elt` で無効にし、進行中の自動更新より先に送る（NFR-SEC-12・FR-OPS-9） |
| 送信欄 | `hx-post="P/send"` `hx-disabled-elt="this"` | `send_text` |
| 中断・終了 | `hx-post="P/escape"`・`hx-post="P/close"` `hx-confirm` `data-confirm-title` `data-confirm-label` | 確認ダイアログ（`modal.js` の `htmx:confirm`。取り消しは「キャンセル」）を経ないと送らない（NFR-SEC-12）。確定したフォームだけが `confirm=true` を持つ |
| 新しいセッション | `hx-get="P/start/dirs?path=…"`（ピッカー）・`hx-post="P/start"`（入力 → 確認 `confirm=1` → 起動）`hx-disabled-elt` `hx-indicator` | 1 階層ずつたどり、確認画面を経て `pane_start`。待つ間はスピナー |
| 共通 | `hx-disabled-elt="this"`、`hx-indicator` | 二重送信防止・ローディング表示。Optimistic UI とスケルトンは使わない |

`static/dist/js/app.js` と `static/dist/js/modal.js`（外部ファイル。CSP の `script-src 'self'` で許可される。インラインではない）は次の 7 つだけを行う。`htmx:confirm` の項目は `modal.js`、ほかは `app.js`（行数の上限 300 行のため分けている）。

- `htmx:beforeRequest` で、`document.hidden` のあいだ `…/snapshot` と `…/live`（ペイン詳細の自動更新）へのリクエストを `preventDefault()` で取り消す（FR-POLL-3・NFR-PERF-8）
- `visibilitychange` で `document.hidden` が `false` になったら `htmx.trigger(document.body, 'refresh')` を発火し、即時取得する（NFR-AVL-4）
- `htmx:oobAfterSwap` で `#toasts` が差し替わったら、3 秒後にトーストへ `is-leaving`（フェードアウト 240ms）を付け、`animationend` で `#toasts` の中身を空にする。`prefers-reduced-motion: reduce` ではすぐ空にする（NFR-DSG-5・6）
- `htmx:afterSwap`・`htmx:oobAfterSwap` で、差し替え前から画面にあったアラート（`[data-alert]`）と同じ文言のものに `is-settled` を付け、ポーリングの再描画でフェードインを繰り返さない。`htmx:afterSettle` で「いま画面にあるアラートの文言」を記録し直す（NFR-DSG-6）
- `leave-fade` 拡張（`<body hx-ext="response-targets, leave-fade">`。`response-targets` が 422 の差し替え先を決めた後に動くよう後ろに置く）が `htmx:beforeSwap` で、差し替え先（主ターゲットと OOB の id が指す要素）の中にあって応答に同じ文言が無いアラート（`[data-alert]`）と、応答にモーダル（`dialog[data-modal]`）が無いときの開いているモーダルへ `is-leaving`（フェードアウト・スクラムも）を付け、`swapOverride` で `swap:` に CSS の `--motion-fade-out`（240ms。`getComputedStyle` で読む。トーストの 3 秒も `--toast-duration` から読む）を足してフェードが終わってから差し替える。reduced-motion では何もしない（NFR-DSG-6）
- `htmx:confirm` で、`hx-confirm` を持つ要素の操作を止めて確認ダイアログ（`#confirm-dialog`。ネイティブ `<dialog>` を `showModal()`）を開く。文言は `hx-confirm` の値、見出しとボタンのラベルは要素の `data-confirm-title`・`data-confirm-label`（`textContent` で入れる）。「確定」で閉じたときだけ `issueRequest(true)` で送る。外側クリック・Esc・ヘッダーの × ボタン（`data-modal-close`）・キャンセルは何も送らず閉じる。`open` 付きの `dialog[data-modal]`（ペアリング確認）は `showModal()` に引き上げ、外側クリック・Esc・× は `data-dismiss` のボタンの押下にする（NFR-DSG-8）
- `pointerdown` でボタン内にリップル（`.ripple`。280ms）を足す。円の大きさと位置は CSSOM（`element.style`）で決める（マークアップの `style` 属性ではないので CSP の `style-src 'self'` に反しない）。reduced-motion では足さない（NFR-DSG-6）

HTMX の設定は `<meta name="htmx-config" content='{"allowEval":false,"includeIndicatorStyles":false}'>` をレイアウトの `<head>` に置く。`allowEval:false` で eval を使う機能（トリガーフィルタ、`hx-on:`・`js:` 構文）を無効にし、`includeIndicatorStyles:false` で htmx が注入するインライン `<style>` を止める（`style-src 'self'` に反するため）。`.htmx-indicator` の表示切替は `app.css`（Tailwind）に書く。

- Lit コンポーネントは使わないため、HTMX ↔ Lit 境界ルールは適用しない（[アーキテクチャ設計 §8](../architecture/overview.md)）
- トーストの 3 秒消去・アラートとモーダルのフェード・ボタンのリップルだけをクライアントで処理する。通信障害（`htmx:responseError`）の汎用表示は持たない（アプリ内通信のため通常は起きない）

### 4.6 ペイン詳細・操作ルートの規約

- **見るだけの接続**: ペイン詳細（`P`・`P/live`）は開ける。`pick`・`send`・`escape`・`close`・`start`・`start/dirs` を呼ぶと、**コンソールに通信せず** 403 と固定文言「この接続では操作できません（見るだけの接続です）」のフラグメントを返す（NFR-SEC-14）。操作の部品（フォーム・ボタン・確認ダイアログ）は HTML に含めない
- **取得失敗**（`P/live`）: 200 のまま、直前の表示を残して固定文言のバナー（§4.4 と同じ規則）。`unauthorized` / `forbidden_scope` はバナーに「再ペアリング」リンクを出し、`hx-trigger` の自動更新属性を出さない。`pane_not_found` は「ペインが見つかりません。画面を更新してください」を出し、操作部品を出さない
- **操作の結果**: 成功・失敗の判定（`ok`・`accepted`・`card_closed`・`interrupted`）は固定文言のトースト OOB（機能要件 §10.1）、ツールのエラー（`operation`）はフラグメント内のインラインアラート。`cursor_mismatch`・`not_asking`・`option_out_of_range` のときは質問カードを取り直したフラグメントを返す
- **排他**: 同じ `(接続, ペイン)` の操作は service が排他にする。実行中に次の操作が来たら通信せず「前の操作がまだ終わっていません」を返す。見るだけの接続に対する排他の取得もしない（先に種類を確認する）
- **ログ**: `RequestLog` はメソッド・パス・ステータス・所要時間だけ。フォーム値・`pane_tail` と `ask_card` の本文・送信本文は出さない（NFR-SEC-11）
- **確認と状態の再検査**: `P/escape`・`P/close` は `confirm=true` が無ければ通信せず `confirm_required`（NFR-SEC-12）。`P/close`（`idle`・`shell`）・`P/start`（`shell`）は、画面の古い状態からの操作を通さないよう、送る前にサーバー側で最新の `report`（single-flight）のペイン状態を再検査する（`pane_not_found`・`invalid_state` は通信せず固定文言）

## 5. 変更の影響範囲

| 変更 | 影響 |
|------|------|
| swarm-console の `report` に新しい `state`・`attention.kind` が追加された | 未知値は `unknown`・生文字列として表示（NFR-CMP-1）。表示ラベルの追加が必要になる |
| ペイロード版 `v` の更新 | §2.1・§2.2 を更新し、パーサを版で分岐する。旧版アプリは「アプリを更新してください」を表示する |
| swarm-console が `/ops` 専用だったツールを `/report` に追加 | viewer は種類別の許可リストにあるツールしか呼ばないため影響なし |
| swarm-console が `dir_list` を持たない古い版（`/ops` の `unknown_tool`） | 新しいセッションのフォルダピッカーが使えない（固定文言「この接続では使えない操作です（ホストの swarm-console が古い可能性があります）」。他の操作は動く） |
| swarm-console が操作ツールの返り値のキーを変えた・足した | 未知のキーは無視する。想定するキーが無い・型が違うときは `protocol`（§1.5.2） |

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|----|------|---------|---------|
| 1.0 | 2026-11-03 | 新規作成 | 初版 |
| 1.1 | 2026-11-03 | トリガーフィルタ `[document.visibilityState…]` を廃止し、静的 `app.js` で非表示中の停止・復帰時の即時取得を行う方式に変更。CSRF 配布を `hx-headers` に訂正。htmx-config を明記 | CSP（`unsafe-eval` なし）で htmx のフィルタ式が動かないため |
| 1.2 | 2026-11-04 | 通常モードのルートから 302・303 を撤去（フルページはその場に描画、HX は `HX-Redirect`）。メイン画面の初回描画をホストへの通信なしにし、タブを snapshot 応答の OOB で更新する | WebKit の WebView が `http.Redirect` を扱えないため。到達不能なホストで初回表示が遅れるため |
| 1.3 | 2026-11-05 | `usage.claude[]`・`codex[]` に `reason`（2 枠とも取れない理由）を追加し、2 枠とも `available: false` は 1 行表示と明記 | Issue #11（swarm-console #87 の `reason` に備える） |
| 1.4 | 2026-11-05 | `report` の `host.mem` を読むキーに追加（無い古い swarm-console では「—」） | Issue #10（swarm-console #86 の `host.mem` に備える） |
| 1.5 | 2026-11-05 | `app.js` の役割に、トーストの退出フェード・アラートのフェードの重複抑止・ボタンのリップルを追記 | Issue #36（デザインガイド M4・M5） |
| 1.6 | 2026-11-05 | 接続の削除・保存データの初期化の確認を `window.confirm` から確認ダイアログに、ペアリング確認を全画面からモーダルに変更。`app.js` の役割に `htmx:confirm` とモーダルを追記 | Issue #37（デザインガイド O1・O2） |
| 1.7 | 2026-11-05 | `app.js` の役割に、差し替えで消えるアラート・モーダルの退出フェード（`leave-fade` 拡張）を追記。`<body>` の `hx-ext` を `response-targets, leave-fade` に | Issue #55（デザインガイド M5・O1） |
| 1.8 | 2026-11-05 | `app.js` の役割に、モーダルのヘッダーの × ボタン（`data-modal-close`）と、時間を CSS のトークンから読むことを追記 | Issue #62・#63（デザインガイド M1・O2） |
| 1.9 | 2026-11-05 | 確認ダイアログとモーダルの処理（`htmx:confirm` ほか）を `app.js` から `modal.js` に分け、`/static/*` とレイアウトの読み込みに `modal.js` を追加 | linterly の行数上限（300 行）への対処 |
| 2.0 | 2026-11-08 | 操作モード（Issue #97）。§1.1 に `/ops` の接続（種類・scope）、§1.2 を種類別の許可リストに変更（`pane_tail`・`ask_card`・`ask_pick`・`send_text`・`send_escape`・`pane_start`・`pane_close`・`dir_list` の引数）、§1.5 に `operation` と `tool_not_allowed`、§1.5.1・§1.5.2（操作ツールのエラー・結果）、§2.1 の `url` の path に `/ops`、§3 を `pair [--scope report\|ops]` に更新（swarm-console 側で実装済み）、§4 にペイン詳細・操作・新しいセッションのルートと HTMX パターン（§4.5・§4.6） | Issue #97（操作モード）、swarm-console endpoints.md 1.21（`dir_list`・`pair --scope ops`） |
| 2.1 | 2026-11-09 | 独立レビューの指摘とコード修正の反映。§1.1・§1.2 にツール別のタイムアウト（読み取り 10 秒、`ask_pick`・`send_text`・`send_escape` 30 秒、`pane_start`・`pane_close` 50 秒）、§4.2 のルート表を実装に合わせて訂正（`live?first=1`、`start/dirs?pick=1`、`POST P/start` の 2 段階、`lines`、`POST P/close` の `confirm=true`・状態の再検査）、§4.5 の `hx-sync`・`hx-preserve`・確認ダイアログ中の自動更新停止、§4.6 に確認と状態の再検査を追記 | Issue #97（独立レビュー ReviewDocs） |
