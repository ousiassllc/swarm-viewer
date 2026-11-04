package servicetest

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
)

// Report は ReportClient のフェイク。gate を設定すると Report は gate が閉じるまで待つ。
type Report struct {
	rec *Recorder

	Mu          sync.Mutex
	SessionList map[string][]domain.Session // 接続 ID → 一覧
	SessionsErr map[string]error
	Reports     map[SnapKey]domain.Report
	ReportErr   map[SnapKey]error
	ProbeErr    error
	ProbeCalls  []ProbeCall
	reportCalls int

	Gate        chan struct{} // 非 nil の間、Report は閉じるまで待つ
	Entered     chan struct{} // Report に入るたびに通知
	inflight    atomic.Int32
	MaxInflight atomic.Int32
}

type ProbeCall struct{ URL, Token string }

func NewReport(rec *Recorder) *Report {
	return &Report{
		rec:         rec,
		SessionList: map[string][]domain.Session{},
		SessionsErr: map[string]error{},
		Reports:     map[SnapKey]domain.Report{},
		ReportErr:   map[SnapKey]error{},
		Entered:     make(chan struct{}, 64),
	}
}

func (f *Report) Sessions(_ context.Context, conn domain.Connection) ([]domain.Session, error) {
	f.rec.Add("report.Sessions:" + conn.ID)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if err := f.SessionsErr[conn.ID]; err != nil {
		return nil, err
	}
	return append([]domain.Session(nil), f.SessionList[conn.ID]...), nil
}

func (f *Report) Report(_ context.Context, conn domain.Connection, session string) (domain.Report, error) {
	n := f.inflight.Add(1)
	defer f.inflight.Add(-1)
	for {
		m := f.MaxInflight.Load()
		if n <= m || f.MaxInflight.CompareAndSwap(m, n) {
			break
		}
	}
	f.rec.Add("report.Report:" + conn.ID + "/" + session)
	f.Mu.Lock()
	f.reportCalls++
	gate := f.Gate
	f.Mu.Unlock()
	f.Entered <- struct{}{}
	if gate != nil {
		<-gate
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	k := SnapKey{conn.ID, session}
	if err := f.ReportErr[k]; err != nil {
		return domain.Report{}, err
	}
	return f.Reports[k], nil
}

func (f *Report) Probe(_ context.Context, url, token string) ([]domain.Session, error) {
	f.rec.Add("report.Probe")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.ProbeCalls = append(f.ProbeCalls, ProbeCall{url, token})
	if f.ProbeErr != nil {
		return nil, f.ProbeErr
	}
	return []domain.Session{{ID: "0", Allowed: true}}, nil
}

// SetSessions は接続の sessions_list を全て許可で設定する。
func (f *Report) SetSessions(connID string, ids ...string) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	ss := make([]domain.Session, len(ids))
	for i, id := range ids {
		ss[i] = domain.Session{ID: id, Allowed: true}
	}
	f.SessionList[connID] = ss
}

// SetReport は (接続, セッション) の report を設定する。
func (f *Report) SetReport(connID, session string, r domain.Report) {
	f.Mu.Lock()
	f.Reports[SnapKey{connID, session}] = r
	f.Mu.Unlock()
}

// SetReportErr は (接続, セッション) の report の失敗を設定する。nil で解除。
func (f *Report) SetReportErr(connID, session string, err error) {
	f.Mu.Lock()
	f.ReportErr[SnapKey{connID, session}] = err
	f.Mu.Unlock()
}

// ReportCount は Report の呼び出し回数。
func (f *Report) ReportCount() int {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.reportCalls
}

// Secrets は SecretStore のフェイク。
type Secrets struct {
	rec *Recorder

	Mu        sync.Mutex
	Tokens    map[string]string
	ErrSet    error
	ErrDelete error
}

func NewSecrets(rec *Recorder) *Secrets {
	return &Secrets{rec: rec, Tokens: map[string]string{}}
}

func (s *Secrets) Set(connID, token string) error {
	s.rec.Add("secret.Set:" + connID)
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.ErrSet != nil {
		return s.ErrSet
	}
	s.Tokens[connID] = token
	return nil
}

func (s *Secrets) Get(connID string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Tokens[connID], nil
}

func (s *Secrets) Delete(connID string) error {
	s.rec.Add("secret.Delete:" + connID)
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.ErrDelete != nil {
		return s.ErrDelete
	}
	delete(s.Tokens, connID)
	return nil
}

// Token は保存済みの token。
func (s *Secrets) Token(connID string) (string, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	t, ok := s.Tokens[connID]
	return t, ok
}

// Notifier は Notifier のフェイク。
type Notifier struct {
	mu   sync.Mutex
	sent []service.Notification
	Err  error
}

func (n *Notifier) Notify(_ context.Context, x service.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, x)
	return n.Err
}

func (n *Notifier) Permitted() service.PermissionState { return service.PermissionGranted }

// Sent は送出された通知。
func (n *Notifier) Sent() []service.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]service.Notification(nil), n.sent...)
}

// IDs は連番の IDGenerator。
type IDs struct{ n atomic.Int32 }

// NewID は id-1, id-2, ... を返す。
func (f *IDs) NewID() string {
	return "id-" + string(rune('0'+f.n.Add(1)))
}
