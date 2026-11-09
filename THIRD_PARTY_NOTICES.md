# サードパーティの通知

swarm-viewer が同梱する第三者の成果物と、そのライセンス。

## Nerd Fonts「Symbols Only」（Symbols Nerd Font Mono）

| 項目 | 内容 |
|------|------|
| 同梱物（Web） | `static/src/fonts/SymbolsNerdFontMono-Regular.woff2`（ビルドで `static/dist/fonts/` へコピーし、バイナリに埋め込む） |
| 同梱物（Android） | `android/app/src/main/res/font/symbols_nerd_font_mono_regular.ttf`（アーカイブの `SymbolsNerdFontMono-Regular.ttf` をそのまま。APK に含む） |
| 元 | [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) v3.5.1 の `NerdFontsSymbolsOnly.tar.xz` に入っている `SymbolsNerdFontMono-Regular.ttf` |
| 加工 | Web は TTF を woff2 に変換しただけ（fontTools + brotli）。Android は加工なしの TTF。どちらも字形の追加・削除・サブセット化はしていない |
| 用途 | omp のペインタイトルに入る Nerd Font のアイコン（私用領域 PUA）の表示 |
| ライセンス | MIT（Copyright (c) 2014 Ryan L McIntyre）。原文は [`static/src/fonts/LICENSE-NerdFonts.txt`](static/src/fonts/LICENSE-NerdFonts.txt)（アーカイブの `LICENSE`） |

アイコンの元（Codicons・Font Awesome・Material Design Icons・Octicons・Weather Icons など）は、それぞれ別のライセンス（CC BY 4.0・Apache 2.0・MIT・SIL OFL 1.1 など）で公開されている。一覧と表示義務の要約は Nerd Fonts の [`license-audit.md`](https://github.com/ryanoasis/nerd-fonts/blob/v3.5.1/license-audit.md)（v3.5.1）を参照する。

## Sora・Noto Sans JP・JetBrains Mono（Ousiass デザインガイドの書体）

| 項目 | 内容 |
|------|------|
| 同梱物（Web） | `static/src/fonts/` の `Sora-Regular.woff2`・`Sora-SemiBold.woff2`・`NotoSansJP-Regular.woff2`・`NotoSansJP-SemiBold.woff2`・`JetBrainsMono-Regular.woff2`（ビルドで `static/dist/fonts/` へコピーし、バイナリに埋め込む） |
| 同梱物（Android） | `android/app/src/main/res/font/` の `sora_regular.ttf`・`sora_semibold.ttf`・`noto_sans_jp_regular.ttf`・`noto_sans_jp_semibold.ttf`・`jetbrains_mono_regular.ttf` |
| 元 | [google/fonts](https://github.com/google/fonts) の `ofl/sora`・`ofl/notosansjp`・`ofl/jetbrainsmono`（可変フォント `[wght].ttf`、2026-11-05 取得） |
| 加工 | 可変フォントから Regular（400）・SemiBold（600）を固定インスタンス化（fontTools の `instancer`）。Noto Sans JP は JIS X 0208 の全文字・ASCII・半角カナ・矢印などにサブセット化し、ヒンティングを除いた（Sora・JetBrains Mono は全グリフ）。Web は woff2、Android は ttf |
| 用途 | UI の本文（Sora + Noto Sans JP）、コードと桁を揃える数値（JetBrains Mono） |
| ライセンス | SIL Open Font License 1.1。原文は [`static/src/fonts/LICENSE-OFL-sora.txt`](static/src/fonts/LICENSE-OFL-sora.txt)・[`LICENSE-OFL-notosansjp.txt`](static/src/fonts/LICENSE-OFL-notosansjp.txt)・[`LICENSE-OFL-jetbrainsmono.txt`](static/src/fonts/LICENSE-OFL-jetbrainsmono.txt) |

## Iconoir（UI アイコン）

| 項目 | 内容 |
|------|------|
| 同梱物（Web） | `internal/web/atoms/icon.templ` に埋め込んだ `warning-circle`・`warning-triangle`・`info-circle`・`check-circle`・`check` の線画（インライン SVG） |
| 同梱物（Android） | `android/app/src/main/res/drawable/` の `ic_alert_danger.xml`・`ic_alert_warning.xml`・`ic_alert_info.xml`・`ic_alert_success.xml`・`ic_nav_arrow_down.xml`・`ic_notification.xml`（`bell`。vector drawable に変換） |
| 元 | [Iconoir](https://github.com/iconoir-icons/iconoir) |
| 加工 | 線の太さ 1.5・丸い端のまま、色を `currentColor`（Android は使う側の tint。`ic_notification.xml` は OS が単色に置き換える通知の小アイコンなので白の線） |
| 用途 | インラインアラートの種別アイコン、メニューの矢印、Android の通知の小アイコン |
| ライセンス | MIT（Copyright (c) 2021 Luca Burgio） |
