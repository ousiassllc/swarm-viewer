package panehandler

import (
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service/paneservice"
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

// startSkills は `GET …/start/skills?agent=`（スキルの選択部品の部分描画）。操作の接続のみ（見るだけは通信せず 403）。
// skill_list で agent のスキルを取る。取れなくても（unknown_tool・失敗）固定文言を返し、「スキルなし」で起動は続けられる。
// 結果は保持しない（画面を開いている間のメモリ＝この応答の HTML だけ）。
func (h *PaneHandler) startSkills(c *gin.Context) {
	t, ok := h.loadOpsTarget(c)
	if !ok {
		return
	}
	agent := c.Query(panedetail.FieldAgent)
	if !ops.IsAgent(agent) {
		h.resp.Fail(c, http.StatusBadRequest, format.OperationMessage("invalid_args"))
		return
	}
	p := panedetail.SkillPickerProps{Ref: t.ref, Agent: agent}
	list, err := h.panes.Skills(c.Request.Context(), t.conn.ID, agent)
	if h.connGone(c, err) {
		return
	}
	if err != nil {
		p.Error, p.Reload = format.SkillListFailureMessage(err)
	} else {
		p.Skills = list
	}
	h.resp.Render(c, http.StatusOK, panedetail.SkillPicker(p))
}

// startSubmit は `POST …/start`。agent・cwd・title と、任意の最初のタスク（skill・task）を受け、段階で応答を分ける。操作の接続のみ。
//   - resend=1: 最初のタスクを send_text だけで送り直す（「もう一度送る」。pane_start は呼ばない）
//   - edit: 入力画面へ戻す
//   - confirm なし: 入力を検査して確認画面を返す（pane_start も send_text も呼ばない）
//   - confirm=1: pane_start。idle で最初のタスクがあれば続けて send_text。成功は HX-Redirect でペイン詳細、
//     失敗（ok:false・エラー）は確認画面に固定文言、最初のタスクを送っていない・送れなかったときは結果の表示
func (h *PaneHandler) startSubmit(c *gin.Context) {
	t, _, ok := h.opsForm(c)
	if !ok {
		return
	}
	first := ops.FirstTask{Skill: formText(c, panedetail.FieldSkill), Task: formText(c, panedetail.FieldTask)}
	if c.PostForm(panedetail.FieldResend) == "1" {
		h.resend(c, t, first)
		return
	}
	req, code := ops.StartRequest{
		Agent: c.PostForm(panedetail.FieldAgent), Cwd: c.PostForm(panedetail.FieldCwd), Title: c.PostForm(panedetail.FieldTitle),
	}.Normalize()
	if code == "" {
		_, code = first.Text() // 最初のタスクの長さ・制御文字は、pane_start の前に入力欄の固定文言で断る
	}
	form := panedetail.StartFormProps{Ref: t.ref, Agent: req.Agent, Cwd: req.Cwd, Title: req.Title, Skill: first.Skill, Task: first.Task}
	switch {
	case c.PostForm(panedetail.FieldEdit) == "1":
		h.resp.Render(c, http.StatusOK, panedetail.StartForm(form))
	case code != "":
		form.Notice = format.OperationMessage(code)
		h.resp.Render(c, http.StatusOK, panedetail.StartForm(form))
	case c.PostForm(panedetail.FieldConfirm) != "1":
		h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, first, "")))
	default:
		h.start(c, t, req, first)
	}
}

// formText は改行を \n に揃えたフォームの値（ブラウザのフォームは CRLF で送る）。
func formText(c *gin.Context, key string) string {
	return strings.ReplaceAll(c.PostForm(key), "\r\n", "\n")
}

func confirmProps(t target, req ops.StartRequest, first ops.FirstTask, notice string) panedetail.StartConfirmProps {
	return panedetail.StartConfirmProps{Ref: t.ref, Agent: req.Agent, Cwd: req.Cwd, Title: req.Title, Skill: first.Skill, Task: first.Task, Notice: notice}
}

// start は確認済みの入力で pane_start を呼ぶ（最初のタスクがあれば、idle のときだけ続けて send_text）。
// 起動できるのは shell のペインだけで、送る前にサーバー側でも最新の状態を確かめる。
func (h *PaneHandler) start(c *gin.Context, t target, req ops.StartRequest, first ops.FirstTask) {
	if err := h.requireState(c, t, func(s domain.PaneState) bool { return s == domain.StateShell }); err != nil {
		h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, first, noticeOf(err))))
		return
	}
	out, err := h.panes.Start(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, req, first)
	if h.connGone(c, err) {
		return
	}
	notice := ""
	switch {
	case err != nil:
		notice = noticeOf(err)
	case !out.Start.OK:
		notice = format.StartToast(out.Start)
	case out.Task == paneservice.TaskNotIdle:
		// HX-Redirect ではトーストが消えるため、遷移せずこの表示で知らせる。本文はこの表示の間だけ持ち、「もう一度送る」は出さない。
		h.resp.Render(c, http.StatusOK, panedetail.StartTaskResult(panedetail.StartTaskResultProps{
			Ref: t.ref, Title: format.FirstTaskNotIdleMessage(out.Start.State),
		}))
		return
	case out.Task == paneservice.TaskFailed:
		reason := format.SendToast(out.Send)
		if out.Err != nil {
			reason = noticeOf(out.Err)
		}
		h.renderTaskFailed(c, t, first, reason, true)
		return
	default:
		h.redirectToPane(c, t)
		return
	}
	h.resp.Render(c, http.StatusOK, panedetail.StartConfirm(confirmProps(t, req, first, notice)))
}

// resend は「もう一度送る」。最初のタスクを send_text だけで送り直す（pane_start は呼ばない。自動の再送はしない）。
// 送る前に最新の report でペインが idle かを確かめ、idle でなければ通信せず invalid_state で断る。
func (h *PaneHandler) resend(c *gin.Context, t target, first ops.FirstTask) {
	if _, code := first.Text(); code != "" || first.Empty() {
		if code == "" {
			code = "invalid_args"
		}
		h.renderTaskFailed(c, t, first, format.OperationMessage(code), false)
		return
	}
	if err := h.requireState(c, t, func(s domain.PaneState) bool { return s == domain.StateIdle }); err != nil {
		h.renderTaskFailed(c, t, first, noticeOf(err), true)
		return
	}
	res, err := h.panes.Resend(c.Request.Context(), t.conn.ID, t.ref.Session, t.ref.Pos, first)
	if h.connGone(c, err) {
		return
	}
	switch {
	case err != nil:
		h.renderTaskFailed(c, t, first, noticeOf(err), true)
	case !res.Accepted:
		h.renderTaskFailed(c, t, first, format.SendToast(res), true)
	default:
		h.redirectToPane(c, t)
	}
}

// renderTaskFailed は最初のタスクを送れなかったときの結果の表示（起動は取り消さない）。retry が true なら「もう一度送る」を出す。
func (h *PaneHandler) renderTaskFailed(c *gin.Context, t target, first ops.FirstTask, reason string, retry bool) {
	h.resp.Render(c, http.StatusOK, panedetail.StartTaskResult(panedetail.StartTaskResultProps{
		Ref: t.ref, Title: format.FirstTaskFailedMessage, Reason: reason, Resend: retry, Skill: first.Skill, Task: first.Task,
	}))
}

// redirectToPane は成功時のペイン詳細への遷移（トーストは出さない）。
func (h *PaneHandler) redirectToPane(c *gin.Context, t target) {
	c.Header("HX-Redirect", paths.Pane(t.conn.ID, t.ref.Session, t.ref.Pos))
	c.Status(http.StatusOK)
}
