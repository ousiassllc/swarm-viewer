# swarm-viewer

デスクトップから、[swarm-console](https://github.com/ousiassllc/swarm-console) が管理する tmux セッションの状態と、Claude Code・Codex の利用上限を一目で確認できる読み取り専用アプリです（Wails v3 / Go）。操作・送信・指示出しはしません。

- tailnet 経由で swarm-console の `POST /report` にだけ接続し、読み取り専用トークン（scope=`report`）を使います
- tmux セッションごとの上段タブ、ペイン一覧、利用上限、ホスト負荷、要対応（`asking`・`stalled`）の OS 通知
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

swarm-console が発行する接続文字列（`swarm-viewer://pair?v=1&url=…&token=…`）を、アプリの貼り付け欄に入れるか、ディープリンクで開いて接続を追加します。

- **発行した接続文字列は再利用しないでください。** 接続文字列とディープリンクには token が含まれ、ディープリンクは OS のプロセス引数や履歴に残ることがあります。ペアリングに使った文字列はチャットやメモに残さず、再度ペアリングするときは新しく発行した文字列を使ってください
- token は OS の秘密領域（macOS Keychain / Windows Credential Manager）にだけ保存し、SQLite・ログ・画面には出しません

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
- WebKit WebView（macOS）でのリダイレクト・`HX-Redirect` の挙動
