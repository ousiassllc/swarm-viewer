// Package service はユースケース層。domain と、自身が定義する interface（consumer-defined）だけに依存する。
//
// repository 実装・Wails・web には依存しない。repository 実装は Store・ReportClient・SecretStore を満たし、
// app が Notifier・Navigator・Clock・IDGenerator を実装して main.go で注入する。
//
// ログには token・完全な URL・ペイン本文を出さない（NFR-LOG-3・NFR-SEC-11）。接続は表示名と host だけで識別する。
package service
