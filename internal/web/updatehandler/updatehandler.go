// Package updatehandler は新しい版の通知と更新の開始の画面ルート（Issue #109）。linterly の 1 ディレクトリの行数上限のため handler とは別のパッケージ。
//
// service/updater と domain にだけ依存し、repository・app は import しない。応答は Templ のコンポーネントだけで作り、
// 失敗の原因（URL・ファイル名・エラー文）は画面に出さず、種類ごとの固定文言にする。
// どのルートも、失敗してもメイン画面を壊さない（通知の領域だけを返す）。
package updatehandler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/service/updater"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms"
)

// Service は更新の service（service/updater.Service が満たす）。
type Service interface {
	Enabled() bool
	Status() updater.Status
	ApplyAsync() (<-chan struct{}, error)
}

// Responder は応答の書き出し（handler パッケージの共通の応答処理を借りる）。
type Responder interface {
	Render(c *gin.Context, status int, comp templ.Component)
}

// Handler は新しい版の通知と更新のルート。
type Handler struct {
	resp Responder
	svc  Service
}

// New は Handler を作る。svc が nil なら（recovery モードなど）通知を出さない。
func New(resp Responder, svc Service) *Handler { return &Handler{resp: resp, svc: svc} }

// Enabled は通知の領域をメイン画面に出すか（リリースの版・更新ファイルのある OS のとき）。
func (h *Handler) Enabled() bool { return h != nil && h.svc != nil && h.svc.Enabled() }

// Register は `GET /update/notice`（状態の読み直し）と `POST /update/apply`（更新の開始）を登録する。
func (h *Handler) Register(r gin.IRoutes) {
	r.GET("/update/notice", h.notice)
	r.POST("/update/apply", h.apply)
}

// notice は通知の領域を、いまの状態で描画する。
func (h *Handler) notice(c *gin.Context) {
	h.resp.Render(c, http.StatusOK, organisms.UpdateNotice(h.props()))
}

// apply は更新を始める（別 goroutine。取得・検証・適用の進み具合は notice が読み直して出す）。
// すでに更新中・新しい版が無いときは、何も始めずに状態を返す。
func (h *Handler) apply(c *gin.Context) {
	if h.Enabled() {
		// 返るエラー（ErrBusy・ErrNotAvailable）はどちらも「始めない」だけで、画面はいまの状態を出す。
		_, _ = h.svc.ApplyAsync()
	}
	h.resp.Render(c, http.StatusOK, organisms.UpdateNotice(h.props()))
}

// props は updater.Status を画面の入力（固定文言）に解決する。無効な環境では空（何も出さない）。
func (h *Handler) props() organisms.UpdateNoticeProps {
	if !h.Enabled() {
		return organisms.UpdateNoticeProps{}
	}
	return Props(h.svc.Status())
}

// Props は updater.Status から通知の入力を作る（進み具合は全体のサイズが分かるときだけ。分からなければ負にして出さない）。
func Props(st updater.Status) organisms.UpdateNoticeProps {
	p := organisms.UpdateNoticeProps{
		Available:   st.Available,
		Latest:      st.Latest,
		Updating:    st.Updating,
		Percent:     -1,
		FailMessage: failMessage(st.ApplyFailed),
		CheckFailed: st.CheckFailed,
	}
	if st.Updating && st.Total > 0 {
		p.Percent = float64(st.Done) * 100 / float64(st.Total)
	}
	return p
}

// failMessage は更新の失敗の種類ごとの固定文言。
func failMessage(k updater.FailKind) string {
	switch k {
	case updater.FailNone:
		return ""
	case updater.FailDownload:
		return "更新ファイルを取得できませんでした。ネットワークを確認して、もう一度お試しください"
	case updater.FailVerify:
		return "更新ファイルを検証できなかったため、更新しませんでした"
	default:
		return "更新を始められませんでした。もう一度お試しください"
	}
}
