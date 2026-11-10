# swarm-viewer

デスクトップ（Wails v3 / Go）から、[swarm-console](https://github.com/ousiassllc/swarm-console) が管理する tmux セッションの状態と、Claude Code・Codex の利用上限を一目で確認できるアプリです。既定は見るだけで、接続を「操作」として登録したときだけ、ペインの中身を見る・質問に答える・指示を送る・中断する・新しいセッションを始める・終了することもできます。Android 版は `android/` にあります。

- tailnet 経由で swarm-console の `POST /report`（見るだけ。読み取り専用トークン scope=`report`）か `POST /ops`（操作。scope=`ops` のトークン）に接続します。どちらかは接続文字列の `url` の path で決まり、呼ぶツールは接続の種類ごとの許可リストだけです
- tmux セッションごとの上段タブ、ペイン一覧、利用上限、ホスト負荷、要対応（`asking`・`stalled`）の OS 通知
- ペインをクリックすると詳細（`pane_tail` の中身。見るだけの接続でも見られます）。操作の接続では、質問カードへの回答、指示の送信、中断（確認あり）、新しいセッションの起動（エージェント → フォルダ → タイトル）、終了（確認あり）ができます。見るだけの接続には操作の UI を出しません
- 新しい版の確認と、アプリ内からの更新（Windows・macOS・Android。[新しい版への更新](#新しい版への更新)）
- 設計は [docs/overview.md](docs/overview.md) 以下を参照してください

## インストール

[GitHub Releases](https://github.com/ousiassllc/swarm-viewer/releases) から、最新の Release の次のファイルをダウンロードします（`vX.Y.Z` は版）。

| OS | ファイル |
|----|---------|
| Windows（64 ビット） | `swarm-viewer_vX.Y.Z_windows_amd64_installer.exe`（NSIS インストーラ。管理者権限でインストールします） |
| macOS（Apple Silicon・Intel 共通） | `swarm-viewer_vX.Y.Z_darwin_universal.zip`（展開すると `swarm-viewer.app`） |
| 共通 | `SHA256SUMS`（上の 2 ファイルのチェックサム） |

実行前に、ダウンロードしたファイルが `SHA256SUMS` と一致することを確かめてください。

- macOS / Linux: `SHA256SUMS` と同じフォルダで `sha256sum -c --ignore-missing SHA256SUMS`（macOS に `sha256sum` が無ければ `shasum -a 256 -c --ignore-missing SHA256SUMS`）
- Windows（PowerShell）: `Get-FileHash .\swarm-viewer_vX.Y.Z_windows_amd64_installer.exe -Algorithm SHA256` の値を、`SHA256SUMS` の同じファイル名の行と見比べます

配布物はコード署名・公証をしていません。初回起動の警告の扱いは[署名されていないアプリの開き方](#署名されていないアプリの開き方)を参照してください。

## 新しい版への更新

アプリは起動時と 6 時間ごとに GitHub Releases の最新版を確認し、実行中の版より新しいときだけ、メイン画面の上部に「新しい版 vX.Y.Z があります」と「更新する」を出します（設定の「自動で確認する」。既定はオンで、オフにすると確認の通信をしません）。確認の失敗は小さく表示するだけで、ペイン一覧・操作は止めません。

「更新する」を押すと、その環境の更新ファイルと `SHA256SUMS` を取得し、**SHA-256 が `SHA256SUMS` と一致したものだけ**を入れ替えます。一致しない・途中で切れた・オフラインのときは何も入れ替えず、取得したファイルを消します。

| OS | 入れ替え | 制限 |
|----|---------|------|
| Windows | NSIS インストーラ（`…_windows_amd64_installer.exe`）を起動し、アプリを終了します（管理者権限の確認は OS が出します） | 実機での確認は未実施です |
| macOS | zip を実行中の `.app` と同じフォルダへ展開して `.app` を差し替え、アプリの終了後に開き直します | `.app` と同じフォルダに書き込めることが必要です（書き込めないと「更新を始められませんでした」）。実機での確認は未実施です |
| Android | 署名が今のアプリと同じ APK だけを、システムのインストーラで開きます（初回は「このアプリからのインストール」の許可が必要） | 実機（Galaxy Fold7）・実際の GitHub での確認は未実施です |
| Linux | 更新ファイルが無いため、確認も通知もしません | — |

- GitHub への通信は、swarm-console への接続とは別のクライアントで、`api.github.com` と Releases の配信元への https だけに限ります。swarm-console の token・認証情報は GitHub に送りません。このため、**リポジトリが公開されているときだけ**最新版を取得できます（private の間は新しい版なしと同じ扱いです）
- 配布物はコード署名・公証をしていないため、更新後も初回起動の警告が出ることがあります（[署名されていないアプリの開き方](#署名されていないアプリの開き方)）。ロールバックと、無人の更新（「更新する」を押さずに入れ替わること）はありません
- `wails3 dev`・手元の `go build` のビルド（版が `dev`）は、確認も更新もしません。仕様は [docs/requirements/functional.md §4.10](docs/requirements/functional.md)、手元での確かめ方は [docs/environment/setup.md §5.6](docs/environment/setup.md)

## ビルドと実行

必要なツール（詳細は [docs/environment/setup.md](docs/environment/setup.md)）: Go 1.26 以降、[bun](https://bun.sh)、[Task](https://taskfile.dev/installation/)（`wails3 task` でも可）、Wails CLI（`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`）、各 OS の WebView 依存（macOS は Xcode Command Line Tools、Windows は WebView2 Runtime）。

配布物の作り方（リリースの流れ）は [docs/environment/setup.md §5.5](docs/environment/setup.md) にあります。

```sh
task setup       # bun install --frozen-lockfile → lefthook install → task generate（clone 直後に 1 回）
task generate    # *_templ.go と static/dist/ を作る（go build / vet / test の前に必要。コミットしない）
task dev         # wails3 dev で開発モードを起動
task build       # 生成物を作ってから wails3 でビルド
task package     # Windows は NSIS インストーラ、macOS は .app
task run         # ビルドしたアプリを実行
task test        # task generate → go test ./...
task lint        # golangci-lint・templ fmt・Biome・linterly
```

## ペアリング

swarm-console が発行する接続文字列（`swarm-viewer://pair?v=1&url=…&token=…`。見るだけは `swarm-console pair`、操作は `swarm-console pair --scope ops`）を、アプリの貼り付け欄に入れるか、ディープリンクで開いて接続を追加します。

- **発行した接続文字列は再利用しないでください。** 接続文字列とディープリンクには token が含まれ、ディープリンクは OS のプロセス引数や履歴に残ることがあります。ペアリングに使った文字列はチャットやメモに残さず、再度ペアリングするときは新しく発行した文字列を使ってください
- token は OS の秘密領域（macOS Keychain / Windows Credential Manager）にだけ保存し、SQLite・ログ・画面には出しません
- **操作の接続（`/ops`）の token は、ペインへの送信・中断・起動・終了ができます。** ペアリング確認の画面には、操作の接続のときだけ警告が出ます。token が漏れた・端末をなくしたときは、ホストで `swarm-console token revoke <名前>` を実行して失効し（名前は `swarm-console token list` で確かめられます。`pair --scope ops` は `viewer-ops-<ホスト名>-<yyyymmdd>` を付けます）、新しく発行し直してください。失効した接続は 401 になり、アプリは再ペアリングを案内します
- 送信した本文・ペインの中身（`pane_tail`・質問）は保存せず、ログにも出しません（ログに残るのは接続 ID・ツール名・種別・コード・ペイン位置まで）

## データと注意点

- **スナップショットは平文で保存されます。** 直近の取得結果（`title`・`cwd`・`activity`・`note` などホスト側の作業内容）が、接続×セッションごとに最新 1 件だけ SQLite に残ります。DB は `os.UserConfigDir()/swarm-viewer/viewer.db`（所有者のみ読める権限 `0600`）です。暗号化はしないため、ディスク暗号化（FileVault / BitLocker）は OS の機能に任せてください。接続を削除すると、その接続のスナップショットも消えます
- **孤立した秘密領域のエントリの削除。** 接続の削除は SQLite を先に消してから秘密領域を消します。秘密領域の削除に失敗すると（ログに警告が残ります）、エントリが孤立することがあります。サービス名は `swarm-viewer`、アカウント名は `connection:<接続 ID>` です。手動で削除してください。
  - macOS: Keychain Access で `swarm-viewer` を検索して削除するか、次を実行します

    ```sh
    security delete-generic-password -s swarm-viewer -a connection:<接続 ID>
    ```

  - Windows: 「資格情報マネージャー」→「Windows 資格情報」の `swarm-viewer:connection:<接続 ID>` を削除します

## 署名されていないアプリの開き方

初版のバイナリはコード署名・公証をしていません（unsigned。GitHub Releases の配布物も同じです）。初回起動時に OS の警告が出ます。

- **macOS（Gatekeeper）**: ダウンロードした `.app` をダブルクリックすると警告が出て開けません。`.app` を右クリック（Control+クリック）して「開く」を選び、確認ダイアログで「開く」を選びます。または「システム設定」→「プライバシーとセキュリティ」の「このまま開く」を使います。ターミナルから隔離属性を外す場合は `xattr -dr com.apple.quarantine /path/to/swarm-viewer.app` です。自分で署名したビルドでない限り、入手元（GitHub Releases）を確認してから実行してください
- **Windows（SmartScreen）**: インストーラを開くと「Windows によって PC が保護されました」が表示されます。「詳細情報」→「実行」を選びます

## 実機での確認が必要な項目

次の項目は実機での確認が済んでいません（Release の成果物で確認します。[docs/environment/setup.md §5.4](docs/environment/setup.md)）。確認済みとは見なさないでください。一覧と確認内容は [docs/architecture/overview.md §11](docs/architecture/overview.md) にあります。

- go-sdk と swarm-console の組み合わせ（ステートレス運用）
- Wails v3 のトレイ・通知（Windows 11 / macOS）
- URL スキーム `swarm-viewer://` の実機登録（Release のインストーラ・`.app` をインストールして確認）
- `SetURL` による遷移
- WebView の `visibilityState`（最小化・トレイ格納時）
- OS サスペンドからの復帰
- 更新（Windows のインストーラの起動・macOS の `.app` の差し替えと開き直し・Android のシステムのインストーラ・実際の GitHub Release に対する更新。[docs/architecture/overview.md §11](docs/architecture/overview.md) の #18〜#20）
- WebKit WebView（macOS）でのリダイレクト・`HX-Redirect` の挙動
