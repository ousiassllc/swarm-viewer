# テストベクタ

swarm-viewer のデスクトップ版（Go）と Android 版（Kotlin）が**同じファイルを読み**、同じ入力に同じ結果を返すことを確かめるためのテストケース（JSON）。Go のコードは共有しない（[アーキテクチャ設計 §12.4](../../docs/architecture/overview.md)）ため、挙動の一致はこのベクタで保つ。仕様の正本は `docs/`。

| ファイル | 対象 | 仕様 |
|----------|------|------|
| `url.json` | `ValidateURL` / `validateUrl` | NFR-SEC-4・5 |
| `pairing.json` | `ParsePairing` / `parsePairing` | API 仕様 §2.1・§2.2 |
| `report.json` | `ParseReport`・`ParseSessions` / `parseReport`・`parseSessions`、`Layout.Valid`、state の表示ラベル | API 仕様 §1.3・§1.4、機能要件 §3 |
| `detect.json` | `Detect` / `detect` | ER §3.4、機能要件 §8.2 |
| `classify.json` | `Classify` / `classify` | API 仕様 §1.5 |
| `usage_tone.json` | `UsageTone`・`PercentTone` / `usageTone`・`percentTone` | 機能要件 §5.3 |

## 読むテスト

| 言語 | テスト |
|------|--------|
| Go | `internal/domain/vectors/*_test.go`（表示ラベルだけ `internal/web/format/vectors_test.go`）。`go test ./internal/domain/... ./internal/web/format/...` |
| Kotlin | `android/app/src/test/kotlin/com/ousiassllc/swarmviewer/domain/Vectors*Test.kt`。`android/` で `./gradlew test` |

- どちらも、ファイルごとに**ケース数の下限**をテストコードに持つ。ファイルが空・読めない・キーの綴り違いでケースが消えても、静かに空振りしない（下限を下げるときはレビューで理由を確認する）
- ケースを足したら、Go と Kotlin の両方のテストが通ることを確かめる。片方だけ落ちるときは、仕様に照らしてどちらが正しいかを決める。Go の実装が仕様と違うなら、実装の変更は別 Issue に切り出す

## 共通の約束

- 1 ファイルは 300 行以内（linterly の行数上限）。ケースは 1 件 1 行で書く（`name` が失敗時にどのケースか示す）
- 文字列は UTF-8 の JSON。制御文字は `\u0000` のようにエスケープする
- 時刻は UTC・秒精度の RFC 3339（`2026-10-31T13:10:05Z`）に正規化して比べる
- 数値は倍精度の浮動小数として比べる

## 形式

### `url.json`

```json
{"description": "...", "cases": [{"name": "v4 下端", "input": "http://100.64.0.0:48120/report", "want": "http://100.64.0.0:48120/report"}]}
```

`want` が文字列なら成功して正規化後の URL、`null` なら拒否。成功のケースは、`want` をもう一度検証しても同じ値になること（冪等）も確かめる。

### `pairing.json`

`cases[]` の各ケースは `name`・`input` と、次のどれか 1 つ。

- `want`: 成功。`{"version", "url", "token", "name", "sessions_hint"}`
- `want_token_length`: 成功で、token が `a` をその文字数だけ並べた文字列（サイズ上限の境界用）
- `error`: 失敗の種別。`format`（形式の誤り）・`version`（アプリの更新が必要）・`token`・`invalid_url`

`pad` があるときは、`input` の後ろに `pad.fill` を、全体が `pad.to_bytes`（UTF-8 のバイト数）になるまで足し、最後に `pad.suffix` を付けたものを入力にする。

失敗時は例外・エラーに入力（token を含みうる）が含まれないことも確かめる。

### `report.json`

- `parse_report[]`: `name`・`input`（JSON の値）・`expect`。`input` を JSON 文字列にして解析し、結果を次の**正規化した Report** にして、`expect` が**部分木**として含まれることを確かめる
  - オブジェクト: `expect` のキーがすべて存在し、値が一致する（`expect` に無いキーは見ない）
  - 配列: 長さが等しく、要素ごとに部分木として一致する
  - スカラー: 等しい（`null` は `null` と等しい）
- 正規化した Report（`null` になりうるのは `at`・`agent`・`fetched_at`・`resets_at`・`expires_at`・`note`・`window`・アカウントの `reason`・枠の `used` だけ。ほかの文字列は空文字、配列は空配列、数値は 0 に正規化済み）:

```
at, layout{rows, cols[]},
panes[]{pos, id, agent, state, title, cwd, elapsed, activity, truncated, changed_ago, note{from, repo, items[], next}},
attention[]{kind, pane, detail}, warnings[]{code, message},
host{cpu{available, used_percent}, mem{available, total_gb, used_gb, used_percent}, swap{同じ形}, disk[]{mount, available, free_gb, total_gb, used_percent}},
usage{status, reason, fetched_at, claude[]{account, five_hour, seven_day, reason}, codex[]{同じ形}, cursor[]{account, window, models, reason}},
auth{agents[]{agent, status, expires_at}}
枠（five_hour・seven_day・models）: {available, used, unit, status, resets_at}
```

- `parse_report_invalid[]`: `name`・`input_raw`（本文そのまま）。解析が `ErrInvalidResponse` / `InvalidResponseException` で失敗し、エラーに本文が含まれないこと
- `parse_sessions[]`: `name`・`input`・`want`（許可セッションの `id` の配列）。`parse_sessions_invalid[]` は `parse_report_invalid` と同じ形
- `layout_valid[]`: `name`・`rows`・`cols`・`pane_count`・`want`
- `state_labels[]`: `state`（文字列）・`label`。`state` を `PaneState` に変換して表示ラベルを比べる。未知の値は「判定できない」

### `detect.json`

- `cases[]`: `name`・`known`・`panes`・`baselining`・`notify`・`insert`・`delete`。`known`・`insert`・`delete` は `{"pane_id", "state"}`、`panes` は `{"id", "pos", "state"}`、`notify` は `{"pane_id", "state"}`（通知は `pane_id` と `state` だけ比べる）。`state` は文字列で、未知の値は `unknown` として扱う。`insert`・`delete`・`notify` は**順序も含めて**一致すること。入力（`known`・`panes`）は変更されないこと
- `detect_cycles[]`: `name`・`steps[]{state, notify}`。1 つのペイン（`%1`）を `state` の列で観測し、毎回 `detect` の結果を `known` に適用（`delete` を除いて `insert` を足す）して次へ進む。各ステップの通知の件数が `notify` と等しいこと

### `classify.json`

`cases[]`: `name`・`error`・`http_status`・`tool_code`・`want`。

- `error`: `none`（エラーなし）・`network`（タイムアウト・DNS・route なしなどの一般のエラー）・`invalid_response`（応答の解析失敗）・`fetch_error`（分類済みの失敗。`fetch_kind`・`fetch_code` を持つ）
- `http_status` が 0 のときは HTTP の応答がない
- `want`: `null` なら成功（分類なし）、それ以外は `{"kind", "code"}`。`kind` は種別名（`unreachable`・`unauthorized`・`forbidden_scope`・`session_required`・`session_forbidden`・`session_missing`・`host_error`・`protocol`）。エラーの本文（`Error()` / `message`）は種別名と `code` だけで、入力の `tool_code` の本文を含まないこと

### `usage_tone.json`

- `cases[]`: `name`・`slot{available, used, unit, status}`・`want`（`normal`・`warning`・`error`）
- `percent_cases[]`: `percent`・`want`

## 既知の差

- Go の `encoding/json` はキーの大文字小文字を区別せずに読むが、Kotlin 側は区別する。swarm-console の応答のキーは小文字固定のため、ベクタには含めない
