package handler_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/servicetest"
	"github.com/ousiassllc/swarm-viewer/internal/web/webtest"
)

func paste(raw string) reqOpt { return form(url.Values{"pairing": {raw}}) }

// pasteDraft は貼り付けて下書きを作り、その ID を返す。
func pasteDraft(t *testing.T, a *app) string {
	t.Helper()
	w := a.do(http.MethodPost, "/pairing", hx(), paste(servicetest.PairingStr))
	wantStatus(t, w, http.StatusOK)
	loc := w.Header().Get("HX-Redirect")
	if w.Header().Get("Location") != "" {
		t.Fatal("リダイレクト（Location）を使ってはいけない")
	}
	id, ok := strings.CutPrefix(loc, "/pairing/drafts/")
	if !ok || id == "" {
		t.Fatalf("Location = %q", loc)
	}
	return id
}

func TestPairingPage(t *testing.T) {
	a := newApp(t)
	full := wantStatus(t, a.get("/pairing"), http.StatusOK)
	webtest.MustContain(t, full, "<html", `id="pairing-form"`, `hx-post="/pairing"`, `name="pairing"`)
	webtest.MustNotContain(t, full, "保存済みの接続")
	frag := wantStatus(t, a.get("/pairing", hx()), http.StatusOK)
	webtest.MustNotContain(t, frag, "<html")

	a.env.AddConn("c1", "devbox", servicetest.URL1, true)
	webtest.MustContain(t, wantStatus(t, a.get("/pairing", hx()), http.StatusOK), "保存済みの接続")
}

func TestPairingPageInvalidLink(t *testing.T) {
	a := newApp(t)
	body := wantStatus(t, a.get("/pairing?link=invalid&token="+servicetest.Token), http.StatusOK)
	webtest.MustContain(t, body, "リンクを開けませんでした")
	webtest.MustNotContain(t, body, servicetest.Token)
	// 他の値では出さない。
	webtest.MustNotContain(t, wantStatus(t, a.get("/pairing?link=other"), http.StatusOK), "リンクを開けませんでした")
}

func TestPairingPasteInvalid(t *testing.T) {
	cases := []struct {
		name, raw, msg string
	}{
		{"形式不正", "not a pairing string", "接続文字列の形式が正しくありません"},
		{"版が違う", "swarm-viewer://pair?v=2&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport&token=" + servicetest.Token, "アプリを更新してください"},
		{"範囲外の URL", "swarm-viewer://pair?v=1&url=http%3A%2F%2F8.8.8.8%3A48120%2Freport&token=" + servicetest.Token, "接続先が許可された範囲外です"},
		{"token なし", "swarm-viewer://pair?v=1&url=http%3A%2F%2F100.64.0.5%3A48120%2Freport", "token が含まれていません"},
		{"空", "", "接続文字列の形式が正しくありません"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t)
			w := a.do(http.MethodPost, "/pairing", hx(), paste(tc.raw))
			body := wantStatus(t, w, http.StatusUnprocessableEntity)
			webtest.MustContain(t, body, tc.msg, `hx-post="/pairing"`)
			webtest.MustNotContain(t, body, "<html", servicetest.Token, "swarm-viewer://pair?v", "8.8.8.8", "not a pairing string")
		})
	}
}

func TestPairingPasteTooLarge(t *testing.T) {
	a := newApp(t)
	w := a.do(http.MethodPost, "/pairing", hx(), paste(strings.Repeat("a", 64<<10)))
	body := w.Body.String()
	if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
	webtest.MustNotContain(t, body, "aaaaaaaa")
}

func TestDraftPage(t *testing.T) {
	a := newApp(t)
	id := pasteDraft(t, a)

	full := wantStatus(t, a.get("/pairing/drafts/"+id), http.StatusOK)
	webtest.MustContain(t, full, "<html", "devbox", "100.64.0.5", "0, 1", "有効期限: あと 5 分",
		`hx-post="/pairing/drafts/`+id+`/confirm"`, `hx-post="/pairing/drafts/`+id+`/discard"`)
	// token・URL 全体は出さない。
	webtest.MustNotContain(t, full, servicetest.Token, "48120", "/report", "swarm-viewer://")

	frag := wantStatus(t, a.get("/pairing/drafts/"+id, hx()), http.StatusOK)
	webtest.MustNotContain(t, frag, "<html", servicetest.Token)
}

func TestDraftExpiredAndUnknown(t *testing.T) {
	a := newApp(t)
	id := pasteDraft(t, a)
	a.env.Clock.Advance(service.DraftTTL + time.Second)

	for _, target := range []string{"/pairing/drafts/" + id, "/pairing/drafts/unknown"} {
		full := wantStatus(t, a.get(target), http.StatusOK) // 404 にしない
		webtest.MustContain(t, full, "<html", "もう一度ペアリングしてください")
		frag := wantStatus(t, a.get(target, hx()), http.StatusOK)
		webtest.MustContain(t, frag, "もう一度ペアリングしてください")
		webtest.MustNotContain(t, frag, "<html")
	}
	// 確定も同じ画面。
	body := wantStatus(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/confirm", hx()), http.StatusOK)
	webtest.MustContain(t, body, "もう一度ペアリングしてください")
}

func TestDraftConfirm(t *testing.T) {
	a := newApp(t)
	id := pasteDraft(t, a)
	w := a.do(http.MethodPost, "/pairing/drafts/"+id+"/confirm", hx())
	wantStatus(t, w, http.StatusOK)
	if w.Header().Get("Location") != "" {
		t.Fatal("リダイレクト（Location）を使ってはいけない")
	}
	loc := w.Header().Get("HX-Redirect")
	cid, ok := strings.CutPrefix(loc, "/connections/")
	if !ok || cid == "" {
		t.Fatalf("Location = %q", loc)
	}
	conn, found := a.env.Store.Conn(cid)
	if !found || conn.Name != "devbox" {
		t.Fatalf("conn = %+v found=%v", conn, found)
	}
	if tok, _ := a.env.Secrets.Token(cid); tok != servicetest.Token {
		t.Fatal("token が秘密領域に保存されていない")
	}
	// 下書きは消える。
	webtest.MustContain(t, wantStatus(t, a.get("/pairing/drafts/"+id), http.StatusOK), "もう一度ペアリングしてください")
}

func TestDraftConfirmFailureKeepsDraft(t *testing.T) {
	a := newApp(t)
	id := pasteDraft(t, a)
	a.env.Report.ProbeErr = &domain.FetchError{Kind: domain.ErrUnreachable}

	body := wantStatus(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/confirm", hx()), http.StatusOK)
	webtest.MustContain(t, body, `role="alert"`, "ホストに届きません", "devbox", `hx-post="/pairing/drafts/`+id+`/confirm"`)
	webtest.MustNotContain(t, body, servicetest.Token, "<html")
	if a.env.Store.ConnCount() != 0 {
		t.Fatal("失敗した確定で接続を保存した")
	}
	// 再試行できる。
	a.env.Report.ProbeErr = nil
	wantStatus(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/confirm", hx()), http.StatusOK)
	if a.env.Store.ConnCount() != 1 {
		t.Fatal("再試行で保存されない")
	}
}

func TestDraftDiscard(t *testing.T) {
	a := newApp(t)
	id := pasteDraft(t, a)
	wantHXRedirect(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/discard", hx()), "/pairing")
	webtest.MustContain(t, wantStatus(t, a.get("/pairing/drafts/"+id), http.StatusOK), "もう一度ペアリングしてください")

	// 接続があるときは / へ。
	a.env.AddConn("c1", "devbox", servicetest.URL1, true)
	id = pasteDraft(t, a)
	wantHXRedirect(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/discard", hx()), "/")
}

// HX でないフォーム送信は、移動先をその場に描画する（3xx を使わない）。
func TestPairingFormWithoutHXRendersTargetInPlace(t *testing.T) {
	a := newApp(t)
	draft := a.do(http.MethodPost, "/pairing", paste(servicetest.PairingStr))
	body := wantStatus(t, draft, http.StatusOK)
	webtest.MustContain(t, body, "<html", "devbox", "接続の確認")
	webtest.MustNotContain(t, body, servicetest.Token)
	id := draftID(t, body)

	w := a.do(http.MethodPost, "/pairing/drafts/"+id+"/confirm")
	confirmed := wantStatus(t, w, http.StatusOK)
	if w.Header().Get("Location") != "" {
		t.Fatal("リダイレクト（Location）を使ってはいけない")
	}
	webtest.MustContain(t, confirmed, "<html", "devbox")

	id = pasteDraft(t, a)
	discarded := wantStatus(t, a.do(http.MethodPost, "/pairing/drafts/"+id+"/discard"), http.StatusOK)
	webtest.MustContain(t, discarded, "<html", "devbox")
}

// draftID は確認画面の本文から下書き ID を取り出す。
func draftID(t *testing.T, body string) string {
	t.Helper()
	const prefix = `hx-post="/pairing/drafts/`
	i := strings.Index(body, prefix)
	if i < 0 {
		t.Fatalf("確認画面に下書きの操作が無い:\n%s", body)
	}
	rest := body[i+len(prefix):]
	return rest[:strings.Index(rest, "/")]
}
