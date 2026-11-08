package panehandler_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/web/webtest/opsconsole"
)

const (
	opsWarning  = "この接続はペインに送信・中断・起動・終了できます"
	opsLeakNote = "token が漏れると、この Tailscale 内からペインを操作されます。紛失したらホストで `swarm-console token revoke <名前>` を実行してください"
)

func TestConnectionBadges_ListAndMainHeaderShowKindWithText(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	s.AddConnection("c2", "viewer", pathReport)

	list := body(t, s.Get("/connections"), 200)
	row := func(name string) string {
		i := strings.Index(list, ">"+name+"</p>")
		if i < 0 {
			t.Fatalf("%s の行が無い:\n%s", name, list)
		}
		return list[i : i+700]
	}
	mustContain(t, row("stub"), "操作")
	mustNotContain(t, row("stub")[:300], "見るだけ")
	mustContain(t, row("viewer"), "見るだけ")

	for cid, want := range map[string]string{"c1": "操作", "c2": "見るだけ"} {
		main := body(t, s.Get("/connections/"+cid+"/sessions/0"), 200)
		head := main[:strings.Index(main, "</header>")]
		mustContain(t, head, want)
	}
}

func TestPairingConfirmation_OpsConnectionShowsWarning_ViewOnlyDoesNot(t *testing.T) {
	s := opsconsole.NewStack(t, pathOps)
	draft := func(path string) string {
		raw := "swarm-viewer://pair?v=1&url=" + url.QueryEscape("http://127.0.0.1:9"+path) + "&token=" + opsconsole.Token + "&name=x"
		id, err := s.Svc.Pairing.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return body(t, s.Get("/pairing/drafts/"+string(id)), 200)
	}
	ops := draft("/ops")
	mustContain(t, ops, "data-ops-warning", opsWarning, strings.ReplaceAll(strings.ReplaceAll(opsLeakNote, "<", "&lt;"), ">", "&gt;"), "操作")
	mustNotContain(t, ops, opsconsole.Token)

	view := draft("/report")
	mustContain(t, view, "見るだけ")
	mustNotContain(t, view, "data-ops-warning", opsWarning, opsconsole.Token)
}
