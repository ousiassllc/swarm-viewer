// Package middleware は画面ルート共通の Gin ミドルウェア（docs/components/overview.md §6.2）。
//
// CSRF・NoStore・CSP・Recover・RequestLog。応答の本文を作るときは Templ のコンポーネントを使い、
// panic 値・リクエストの本文・クエリ・ヘッダは画面にもログにも出さない（NFR-LOG-3、NFR-SEC-11）。
package middleware
