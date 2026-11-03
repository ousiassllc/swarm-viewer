package handler

import (
	"context"
	"errors"
	"time"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
)

const (
	// tabsTimeout は sessions_list 1 回に待つ上限。到達不能なホストでも画面の表示を遅らせない（NFR-PERF-1・2）。
	tabsTimeout = 3 * time.Second
	// tabsMaxAge は直近のタブ取得をそのまま信じる期間。これを過ぎたら snapshot 応答のたびに引き直す。
	tabsMaxAge = 30 * time.Second
)

// firstPaintTabs はメイン画面の初回描画に使うタブと選択セッションを決める。ホストとは通信しない。
//
//   - 直近のタブ（ConnectionService のキャッシュ）があれば、requested が許可セッションならそれ、
//     許可外と分かっている（キャッシュが新しい）なら先頭の許可セッション（FR-SESS-3 のフォールバック）。
//   - キャッシュが無く requested（last_session・ディープリンク）があれば、requested 1 件のタブで描画する。
//     保存済みスナップショットをすぐ出し、本物のタブは snapshot 応答が反映する。
//   - 手掛かりが全く無い（初回）、または許可セッション 0 件のキャッシュのときだけ、短い上限付きで取得する。
//
// 描画できなければ、応答を書いて ok=false を返す。
func (h *Handler) firstPaintTabs(c *gin.Context, conn domain.Connection, requested string) (tabs []domain.Session, sid string, ok bool) {
	cached, fetchedAt, haveCache := h.conns.CachedTabs(conn.ID)
	switch {
	case len(cached) > 0:
		switch {
		case requested == "":
			return cached, cached[0].ID, true
		case hasSession(cached, requested):
			return cached, requested, true
		case h.clock.Now().Sub(fetchedAt) < tabsMaxAge:
			return cached, cached[0].ID, true
		}
		return append(cached, domain.Session{ID: requested, Allowed: true}), requested, true
	case requested != "" && !haveCache:
		return []domain.Session{{ID: requested, Allowed: true}}, requested, true
	}
	return h.fetchFirstTabs(c, conn, requested)
}

// fetchFirstTabs は手掛かりが無いときの取得。失敗は専用画面（再試行の導線）で返す。
func (h *Handler) fetchFirstTabs(c *gin.Context, conn domain.Connection, requested string) ([]domain.Session, string, bool) {
	tabs, err := h.loadTabs(c.Request.Context(), conn.ID)
	switch {
	case err == nil && len(tabs) == 0, errors.Is(err, service.ErrNoSessions):
		h.noSessions(c, conn)
	case err == nil:
		sid := tabs[0].ID
		if hasSession(tabs, requested) {
			sid = requested
		}
		return tabs, sid, true
	default:
		fe, isFetch := fetchError(err)
		if !isFetch {
			h.internal(c, "tabs", err)
			return nil, "", false
		}
		p := pages.ConnectionErrorProps{ConnectionID: conn.ID, Name: conn.Name, Err: fe}
		h.view(c, "接続できません", func(s layout.BaseProps) templ.Component { return pages.ConnectionErrorPage(s, p) }, pages.ConnectionErrorContent(p))
	}
	return nil, "", false
}

// loadTabs は sessions_list を短い上限付きで 1 回呼ぶ。
func (h *Handler) loadTabs(ctx context.Context, cid string) ([]domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, tabsTimeout)
	defer cancel()
	return h.conns.Tabs(ctx, cid)
}

// tabsFetch は snapshot 応答のためのタブ取得（report と並行して走る）。
type tabsFetch struct {
	done chan struct{}
	tabs []domain.Session
	err  error
}

// startTabsFetch は、タブを引き直す必要があるとき（直近の取得が無い・古い）だけ取得を始める。
// 再ペアリング待ちの接続では通信しない。不要なら nil。
func (h *Handler) startTabsFetch(ctx context.Context, cid string) *tabsFetch {
	if h.conns.NeedsRepair(cid) {
		return nil
	}
	if _, at, ok := h.conns.CachedTabs(cid); ok && h.clock.Now().Sub(at) < tabsMaxAge {
		return nil
	}
	f := &tabsFetch{done: make(chan struct{})}
	go func() {
		defer close(f.done)
		f.tabs, f.err = h.loadTabs(ctx, cid)
	}()
	return f
}

// wait は取得の完了を待つ。取得しなかった・失敗したときは ok=false。
func (f *tabsFetch) wait() (tabs []domain.Session, ok bool) {
	if f == nil {
		return nil, false
	}
	<-f.done
	return f.tabs, f.err == nil
}

func hasSession(tabs []domain.Session, id string) bool {
	for _, t := range tabs {
		if t.ID == id {
			return true
		}
	}
	return false
}
