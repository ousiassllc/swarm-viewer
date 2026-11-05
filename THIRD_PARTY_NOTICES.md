# サードパーティの通知

swarm-viewer が同梱する第三者の成果物と、そのライセンス。

## Nerd Fonts「Symbols Only」（Symbols Nerd Font Mono）

| 項目 | 内容 |
|------|------|
| 同梱物 | `static/src/fonts/SymbolsNerdFontMono-Regular.woff2`（ビルドで `static/dist/fonts/` へコピーし、バイナリに埋め込む） |
| 元 | [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) v3.5.1 の `NerdFontsSymbolsOnly.tar.xz` に入っている `SymbolsNerdFontMono-Regular.ttf` |
| 加工 | TTF を woff2 に変換しただけ（fontTools + brotli）。字形の追加・削除・サブセット化はしていない |
| 用途 | omp のペインタイトルに入る Nerd Font のアイコン（私用領域 PUA）の表示。日本語・英字は OS のフォントで表示する |
| ライセンス | MIT（Copyright (c) 2014 Ryan L McIntyre）。原文は [`static/src/fonts/LICENSE-NerdFonts.txt`](static/src/fonts/LICENSE-NerdFonts.txt)（アーカイブの `LICENSE`） |

アイコンの元（Codicons・Font Awesome・Material Design Icons・Octicons・Weather Icons など）は、それぞれ別のライセンス（CC BY 4.0・Apache 2.0・MIT・SIL OFL 1.1 など）で公開されている。一覧と表示義務の要約は Nerd Fonts の [`license-audit.md`](https://github.com/ryanoasis/nerd-fonts/blob/v3.5.1/license-audit.md)（v3.5.1）を参照する。
