package panehandler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
	"github.com/ousiassllc/swarm-viewer/internal/web/organisms/panedetail"
	"github.com/ousiassllc/swarm-viewer/internal/web/pages"
	"github.com/ousiassllc/swarm-viewer/internal/web/paths"
)

// maxRecentFolders は「最近使ったフォルダ」の件数の上限。
const maxRecentFolders = 5

// recentFolders は現在の report の panes[] のうち agent があるペインの cwd を、重複なしで最大 5 件返す（保存はしない）。
func recentFolders(panes []domain.Pane) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, p := range panes {
		if p.Agent == nil || *p.Agent == "" || p.CWD == "" {
			continue
		}
		if _, dup := seen[p.CWD]; dup {
			continue
		}
		seen[p.CWD] = struct{}{}
		if out = append(out, p.CWD); len(out) == maxRecentFolders {
			break
		}
	}
	return out
}

// startPage は `GET …/start`（新しいセッション）。操作の接続で、ペインが空き（shell）のときだけ入力画面を出す。
// 状態は report で確かめる（取得に失敗したら固定文言で案内する）。
func (h *PaneHandler) startPage(c *gin.Context) {
	t, ok := h.loadOpsTarget(c)
	if !ok {
		return
	}
	p := pages.PaneStartProps{Form: panedetail.StartFormProps{Ref: t.ref}}
	res := h.snaps.Get(c.Request.Context(), t.conn.ID, t.ref.Session)
	pane, found := findPane(res.Snapshot.Report.Panes, t.ref.Pos)
	switch {
	case res.Err != nil:
		p.Blocked = format.FetchErrorMessage(res.Err)
	case !found:
		p.Blocked = format.OperationMessage("pane_not_found")
	case pane.State != domain.StateShell:
		p.Blocked = format.OperationMessage("invalid_state")
	}
	p.Form.Recent = recentFolders(res.Snapshot.Report.Panes)
	h.resp.View(c, "新しいセッション",
		func(s layout.BaseProps) templ.Component { return pages.PaneStartPage(s, p) },
		pages.PaneStartContent(p))
}

// startDirs は `GET …/start/dirs?path=&pick=1`（フォルダピッカーの部分描画）。操作の接続のみ。
// pick=1 は「このフォルダを選ぶ」（一覧で見たフォルダ、または最近使ったフォルダ）で、選択済みの表示を返す（許可の判定は pane_start が行う）。
// それ以外は dir_list で path 直下を一覧する（path 省略 = 許可されたフォルダの根）。
func (h *PaneHandler) startDirs(c *gin.Context) {
	t, ok := h.loadOpsTarget(c)
	if !ok {
		return
	}
	path := c.Query(panedetail.FieldPath)
	if c.Query(panedetail.FieldPick) == "1" && path != "" {
		h.resp.Render(c, http.StatusOK, panedetail.DirSelected(t.ref, path))
		return
	}
	p := panedetail.DirPickerProps{Ref: t.ref, Path: path}
	if last, ok := h.snaps.Last(c.Request.Context(), t.conn.ID, t.ref.Session); ok && path == "" {
		p.Recent = recentFolders(last.Report.Panes)
	}
	dir, err := h.panes.Dirs(c.Request.Context(), t.conn.ID, path)
	if h.connGone(c, err) {
		return
	}
	if err != nil {
		p.Error = noticeOf(err)
	} else {
		p.Dir = dir
	}
	h.resp.Render(c, http.StatusOK, panedetail.DirPicker(p))
}

// startSubmit は `POST …/start`。agent・cwd・title を受け、段階で応答を分ける。操作の接続のみ。
//   - edit: 入力画面へ戻す
//   - confirm なし: 入力を検査して確認画面を返す（pane_start は呼ばない）
//   - confirm=1: pane_start。成功は HX-Redirect でペイン詳細、失敗（ok:false・エラー）は確認画面に固定文言
func (h *PaneHandler) startSubmit(c *gin.Context) {
	t, _, ok := h.opsForm(c)
	if !ok {
		return
	}
	req, code := ops.StartRequest{
		Agent: c.PostForm(panedetail.FieldAgent), Cwd: c.PostForm(panedetail.FieldCwd), Title: c.PostForm(panedetail.FieldTitle),
	}.Normalize()
	form := panedetail.StartFormProps{Ref: t.ref, Agent: req.Agent, Cwd: req.Cwd, Title: req.Title}
	switch {
	case c.PostForm(panedetail.FieldEdit) == "1":
		h.resp.Render(c, http.StatusOK, panedetail.StartForm(form))
	case code != "":
		form.Notice = format.OperationMessage(code)
		h.resp.Render(c, http.StatusOK, panedetail.StartForm(form))
	case c.PostForm(panedetail.FieldConfirm) != "1":
		h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, "")))
	default:
		h.start(c, t, req)
	}
}

func confirmProps(t target, req ops.StartRequest, notice string) panedetail.StartConfirmProps {
	return panedetail.StartConfirmProps{Ref: t.ref, Agent: req.Agent, Cwd: req.Cwd, Title: req.Title, Notice: notice}
}

// start は確認済みの入力で pane_start を呼ぶ。
// 起動できるのは shell のペインだけで、送る前にサーバー側でも最新の状態を確かめる。
func (h *PaneHandler) start(c *gin.Context, t target, req ops.StartRequest) {
	if err := h.requireState(c, t, func(s domain.PaneState) bool { return s == domain.StateShell }); err != nil {
		h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, noticeOf(err))))
		return
	}
	res, err := h.panes.Start(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, req)
	if h.connGone(c, err) {
		return
	}
	notice := ""
	switch {
	case err != nil:
		notice = noticeOf(err)
	case res.OK:
		c.Header("HX-Redirect", paths.Pane(t.conn.ID, t.ref.Session, t.ref.Pos))
		c.Status(http.StatusOK)
		return
	default:
		notice = format.StartToast(res)
	}
	h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, notice)))
}
