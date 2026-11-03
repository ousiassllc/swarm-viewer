package deeplink

import (
	"log/slog"
	"sync"

	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// invalidLinkPath は解析に失敗したときに開く貼り付けページ（固定の汎用メッセージを出す）。
// 入力の内容は URL に載せない。
const invalidLinkPath = paths.Pairing + "?link=invalid"

// Parser は接続文字列から下書きを作る（*service.PairingService が満たす）。
type Parser interface {
	Parse(raw string) (service.DraftID, error)
}

// Handler は受け取ったディープリンク 1 本を処理する。
type Handler struct {
	parser Parser
	nav    service.Navigator
	log    *slog.Logger
}

// NewHandler は Handler を返す。
func NewHandler(p Parser, nav service.Navigator, log *slog.Logger) *Handler {
	return &Handler{parser: p, nav: nav, log: log}
}

// Handle は raw（引数やイベントで受け取った生の文字列）を処理する。
//
//   - 他の scheme・不正な形式は破棄してログだけ残す（遷移しない）
//   - 解析に成功したら下書きの確認画面へ遷移する（token は WebView に渡さない）
//   - 解析に失敗したら貼り付けページへ遷移する（汎用メッセージ。理由は URL に載せない）
func (h *Handler) Handle(raw string) {
	link, ok := Normalize(raw)
	if !ok {
		h.log.Warn("deeplink ignored: not a swarm-viewer link")
		return
	}
	id, err := h.parser.Parse(link)
	if err != nil {
		// エラーは入力値を含まない（domain.ParsePairing の規約）。URL は host 部分だけ出す。
		h.log.Warn("deeplink rejected", "host", HostOf(link), "err", err)
		h.nav.Open(invalidLinkPath)
		return
	}
	h.nav.Open(paths.Draft(string(id)))
}

// maxPending は Bind 前に溜めるリンクの上限。起動直後の連打で無限に増やさない。
const maxPending = 4

// Dispatcher はハンドラの登録前に届いたリンクを溜め、登録時に流す。
// Wails の単一インスタンス・URL イベントは app.New 直後から届きうるが、
// サービスの組み立て（SQLite を開く）はその後になるため。
type Dispatcher struct {
	mu      sync.Mutex
	fn      func(string)
	pending []string
}

// Submit は raw を登録済みの関数へ渡す。未登録なら溜める（上限を超えたら古いものから捨てる）。
func (d *Dispatcher) Submit(raw string) {
	d.mu.Lock()
	fn := d.fn
	if fn == nil {
		d.pending = append(d.pending, raw)
		if len(d.pending) > maxPending {
			d.pending = d.pending[len(d.pending)-maxPending:]
		}
	}
	d.mu.Unlock()
	if fn != nil {
		fn(raw)
	}
}

// Bind は処理関数を登録し、溜まっていたリンクを登録順に流す。
func (d *Dispatcher) Bind(fn func(string)) {
	d.mu.Lock()
	d.fn = fn
	queued := d.pending
	d.pending = nil
	d.mu.Unlock()
	for _, raw := range queued {
		fn(raw)
	}
}
