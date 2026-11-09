// Package paneservice はペイン詳細の閲覧（pane_tail・ask_card）と、操作の接続でだけ使える操作（選択・送信・中断・起動（最初のタスクを含む）・終了・フォルダ一覧・スキル一覧）を扱う
// （Issue #97。linterly の 1 ディレクトリの行数上限のため、service とは別のパッケージ）。
//
//   - 操作は接続の種類（URL の path）が操作のときだけ行い、見るだけは通信せず ErrOpsRequired で拒否する。
//   - 同じ (接続, セッション, ペイン) への操作は同時に 1 つだけ。実行中に次が来たら通信せず ErrBusy。
//   - 401/403 は needs_repair を立てる（その接続のポーリングと操作を止め、再ペアリングへ）。needs_repair の接続は通信しない。
//   - ログは接続 ID・ツール名・セッション・ペイン位置・種別・コードまで。token・送信本文（最初のタスクの本文を含む）・スキル名・invoke・pane_tail・ask_card の本文は出さない。
package paneservice

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/domain/ops"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

var (
	// ErrOpsRequired は見るだけの接続（`/report`）で操作を求められたとき。swarm-console には通信しない。
	ErrOpsRequired = errors.New("paneservice: connection is view-only")
	// ErrBusy は同じペインへの操作がまだ終わっていないとき（同時に 2 つ送らない）。通信しない。
	ErrBusy = errors.New("paneservice: another operation on this pane is still running")
)

// Client はペインを見る・操作する swarm-console クライアント（repository/report.Client が満たす）。
// service.ReportClient とは別の interface にして、取得だけの既存のフェイクを壊さない。見るだけの接続で操作ツールを
// 呼んだときの拒否は、このサービス（接続の種類の確認）と client（許可リスト）の両方が行う。
type Client interface {
	PaneTail(ctx context.Context, conn domain.Connection, session, pane string, lines int) (ops.PaneTail, error)
	AskCard(ctx context.Context, conn domain.Connection, session, pane string) (ops.AskCard, error)
	AskPick(ctx context.Context, conn domain.Connection, session, pane string, pick ops.Pick) (ops.PickResult, error)
	SendText(ctx context.Context, conn domain.Connection, session, pane, text string) (ops.SendResult, error)
	SendEscape(ctx context.Context, conn domain.Connection, session, pane string, confirmed bool) (ops.EscapeResult, error)
	PaneStart(ctx context.Context, conn domain.Connection, session, pane string, req ops.StartRequest) (ops.StartResult, error)
	PaneClose(ctx context.Context, conn domain.Connection, session, pane string) (ops.CloseResult, error)
	DirList(ctx context.Context, conn domain.Connection, path string) (ops.DirList, error)
	SkillList(ctx context.Context, conn domain.Connection, agent string) (ops.SkillList, error)
}

// PaneService はペインの閲覧と操作。並行に使える。
type PaneService struct {
	client Client
	conns  *service.ConnectionService
	log    *slog.Logger

	mu   sync.Mutex
	busy map[string]struct{} // paneKey → 操作中
}

// New は PaneService を作る。
func New(client Client, conns *service.ConnectionService, log *slog.Logger) *PaneService {
	return &PaneService{client: client, conns: conns, log: log, busy: make(map[string]struct{})}
}

func paneKey(connID, session, pane string) string { return connID + "\x00" + session + "\x00" + pane }

// tryAcquire は (接続, セッション, ペイン) の操作枠を取る。操作中なら false。
func (s *PaneService) tryAcquire(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.busy[key]; taken {
		return false
	}
	s.busy[key] = struct{}{}
	return true
}

func (s *PaneService) release(key string) {
	s.mu.Lock()
	delete(s.busy, key)
	s.mu.Unlock()
}

// callMode は呼び出しの種別。view は見るだけの接続でも使える閲覧、opsRead は操作の接続だけの読み取り（ペインに何も送らない）、
// opsWrite は操作の接続だけのペインへの送信（同じペインで排他）。
type callMode int

const (
	view callMode = iota
	opsRead
	opsWrite
)

// call は 1 回のペイン呼び出しの共通処理（接続の取得・種類の確認・needs_repair・排他・失敗の分類とログ）。
// 取得失敗は *domain.FetchError、種類違いは ErrOpsRequired、排他は ErrBusy、接続が無ければ service.ErrConnectionNotFound。
func call[T any](ctx context.Context, s *PaneService, connID, tool, session, pane string, mode callMode, do func(conn domain.Connection) (T, error)) (T, error) {
	var zero T
	conn, err := s.conns.Get(ctx, connID)
	if err != nil {
		return zero, err
	}
	if mode != view && conn.Kind() != domain.KindOps {
		s.log.Warn("operation refused on a view-only connection", "connection_id", connID, "tool", tool)
		return zero, ErrOpsRequired
	}
	if s.conns.NeedsRepair(connID) {
		return zero, &domain.FetchError{Kind: domain.ErrUnauthorized, Code: "needs_repair"}
	}
	if mode == opsWrite {
		key := paneKey(connID, session, pane)
		if !s.tryAcquire(key) {
			s.log.Warn("operation refused: pane is busy", "connection_id", connID, "tool", tool, "session", session, "pane", pane)
			return zero, ErrBusy
		}
		defer s.release(key)
	}
	v, err := do(conn)
	if err != nil {
		return zero, s.failure(connID, tool, session, pane, err)
	}
	if mode == opsWrite {
		s.log.Info("pane operation done", "connection_id", connID, "tool", tool, "session", session, "pane", pane)
	}
	return v, nil
}

// failure は呼び出しの失敗を *domain.FetchError に分類してログに残す。401・403 は needs_repair を立てる。
func (s *PaneService) failure(connID, tool, session, pane string, err error) *domain.FetchError {
	fe := domain.Classify(err, 0, "")
	if fe == nil {
		fe = &domain.FetchError{Kind: domain.ErrProtocol}
	}
	if fe.Kind == domain.ErrUnauthorized || fe.Kind == domain.ErrForbiddenScope {
		s.conns.MarkNeedsRepair(connID)
	}
	s.log.Warn("pane call failed", "connection_id", connID, "tool", tool, "session", session, "pane", pane, "kind", fe.Kind.String(), "code", fe.Code)
	return fe
}

// Tail は pane_tail（最後の lines 行）を取得する。見るだけの接続でも使える。
func (s *PaneService) Tail(ctx context.Context, connID, session, pane string, lines int) (ops.PaneTail, error) {
	return call(ctx, s, connID, "pane_tail", session, pane, view, func(c domain.Connection) (ops.PaneTail, error) {
		return s.client.PaneTail(ctx, c, session, pane, lines)
	})
}

// Card は ask_card（質問カード）を取得する。見るだけの接続でも使える。
func (s *PaneService) Card(ctx context.Context, connID, session, pane string) (ops.AskCard, error) {
	return call(ctx, s, connID, "ask_card", session, pane, view, func(c domain.Connection) (ops.AskCard, error) {
		return s.client.AskCard(ctx, c, session, pane)
	})
}

// Pick は質問カードの選択肢（番号）か「その他」の文字列を送る。操作の接続だけ。
func (s *PaneService) Pick(ctx context.Context, connID, session, pane string, pick ops.Pick) (ops.PickResult, error) {
	return call(ctx, s, connID, "ask_pick", session, pane, opsWrite, func(c domain.Connection) (ops.PickResult, error) {
		return s.client.AskPick(ctx, c, session, pane, pick)
	})
}

// Send はペインへ指示を送る。操作の接続だけ。
func (s *PaneService) Send(ctx context.Context, connID, session, pane, text string) (ops.SendResult, error) {
	return call(ctx, s, connID, "send_text", session, pane, opsWrite, func(c domain.Connection) (ops.SendResult, error) {
		return s.client.SendText(ctx, c, session, pane, text)
	})
}

// Escape は作業中のエージェントを中断する。操作の接続だけ。confirmed が false（確認ダイアログを経ていない）なら通信せず
// confirm_required を返す。
func (s *PaneService) Escape(ctx context.Context, connID, session, pane string, confirmed bool) (ops.EscapeResult, error) {
	if !confirmed {
		return ops.EscapeResult{}, ops.Error("confirm_required")
	}
	return call(ctx, s, connID, "send_escape", session, pane, opsWrite, func(c domain.Connection) (ops.EscapeResult, error) {
		return s.client.SendEscape(ctx, c, session, pane, confirmed)
	})
}

// TaskStatus は新しいセッションの起動時の、最初のタスクの扱い。
type TaskStatus int

// TaskStatus の値。
const (
	// TaskNone は最初のタスクが無い（起動だけ）、または pane_start が ok: false で送らなかった。
	TaskNone TaskStatus = iota
	// TaskSent は send_text が成功した（accepted: true）。
	TaskSent
	// TaskNotIdle は pane_start に成功したが state が idle 以外のため、送らなかった。
	TaskNotIdle
	// TaskFailed は send_text が失敗または accepted: false だった。起動は取り消していない。
	TaskFailed
)

// StartOutcome は pane_start の結果と、最初のタスクの扱い。pane_start の失敗は error で返し、最初のタスクは送らない。
type StartOutcome struct {
	Start ops.StartResult
	Task  TaskStatus
	// Send は Task が TaskSent・TaskFailed のときの send_text の結果（Err があるときは零値）。
	Send ops.SendResult
	// Err は Task が TaskFailed のときの send_text の失敗（*domain.FetchError）。accepted: false のときは nil。
	Err error
}

// Start は shell のペインでエージェントを起動する。操作の接続だけ。first が空でなければ、pane_start が ok: true で state が
// idle のときだけ、同じ排他を保ったまま続けて send_text で最初のタスクを送る（自動の再送はしない。send_text が失敗しても
// pane_close は呼ばず、起動を取り消さない）。first の検査（長さ・制御文字）は pane_start より前に行い、違反なら何も呼ばない。
func (s *PaneService) Start(ctx context.Context, connID, session, pane string, req ops.StartRequest, first ops.FirstTask) (StartOutcome, error) {
	return call(ctx, s, connID, "pane_start", session, pane, opsWrite, func(c domain.Connection) (StartOutcome, error) {
		text, code := first.Text()
		if code != "" {
			return StartOutcome{}, ops.Error(code)
		}
		res, err := s.client.PaneStart(ctx, c, session, pane, req)
		out := StartOutcome{Start: res}
		switch {
		case err != nil:
			return StartOutcome{}, err
		case text == "" || !res.OK:
			return out, nil
		case res.State != string(domain.StateIdle):
			out.Task = TaskNotIdle
			return out, nil
		}
		send, err := s.client.SendText(ctx, c, session, pane, text)
		switch {
		case err != nil:
			out.Task, out.Err = TaskFailed, s.failure(connID, "send_text", session, pane, err)
		case !send.Accepted:
			out.Task, out.Send = TaskFailed, send
		default:
			out.Task, out.Send = TaskSent, send
		}
		return out, nil
	})
}

// Resend は最初のタスクを send_text だけで送り直す（「もう一度送る」。pane_start は呼ばない）。操作の接続だけ。
// ペインが idle かの検査は、送る前に呼び出し側が最新の report で行う。
func (s *PaneService) Resend(ctx context.Context, connID, session, pane string, first ops.FirstTask) (ops.SendResult, error) {
	return call(ctx, s, connID, "send_text", session, pane, opsWrite, func(c domain.Connection) (ops.SendResult, error) {
		text, code := first.Text()
		if code == "" && text == "" {
			code = "invalid_args"
		}
		if code != "" {
			return ops.SendResult{}, ops.Error(code)
		}
		return s.client.SendText(ctx, c, session, pane, text)
	})
}

// Close はペインのエージェントを終了する。操作の接続だけ。確認済み（画面の確認ダイアログを経た）でなければ、
// 通信の前に confirm_required で断る（Escape と同じ形）。
func (s *PaneService) Close(ctx context.Context, connID, session, pane string, confirmed bool) (ops.CloseResult, error) {
	if !confirmed {
		return ops.CloseResult{}, ops.Error("confirm_required")
	}
	return call(ctx, s, connID, "pane_close", session, pane, opsWrite, func(c domain.Connection) (ops.CloseResult, error) {
		return s.client.PaneClose(ctx, c, session, pane)
	})
}

// Dirs は dir_list（pane_start の cwd に選べるフォルダ）を取得する。操作の接続だけ。ペインに何も送らないので排他は取らない。
func (s *PaneService) Dirs(ctx context.Context, connID, path string) (ops.DirList, error) {
	return call(ctx, s, connID, "dir_list", "", "", opsRead, func(c domain.Connection) (ops.DirList, error) {
		return s.client.DirList(ctx, c, path)
	})
}

// Skills は skill_list（新しいセッションの最初のタスクに選べるスキル）を取得する。操作の接続だけ。ペインに何も送らないので
// 排他は取らない。結果は保持しない（呼び出し側が画面を開いている間のメモリだけに持つ）。
func (s *PaneService) Skills(ctx context.Context, connID, agent string) (ops.SkillList, error) {
	return call(ctx, s, connID, "skill_list", "", "", opsRead, func(c domain.Connection) (ops.SkillList, error) {
		return s.client.SkillList(ctx, c, agent)
	})
}
