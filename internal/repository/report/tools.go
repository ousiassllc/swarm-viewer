package report

// 呼び出すツール名（API 仕様 §1.2）。ツール名は定数で、外部から渡す経路を公開しない（NFR-SEC-1）。
const (
	toolSessionsList = "sessions_list"
	toolReport       = "report"
)

// allowedTools は呼んでよいツールの許可リスト。callTool が送信前に、roundTripper が送信内容に対して二重に検査する。
// `/ops` 専用ツールや `tools/list` を呼ぶコードパスは存在しない。
var allowedTools = map[string]struct{}{toolSessionsList: {}, toolReport: {}}

func toolAllowed(name string) bool {
	_, ok := allowedTools[name]
	return ok
}
