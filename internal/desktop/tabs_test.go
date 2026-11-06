package desktop

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestTabShowReusesScreenAndReplacesPreview(t *testing.T) {
	s := newTabStrip()
	pods, created, _ := s.Show("pods", true)
	if !created || !s.Ephemeral(pods) || s.SelectedScreen() != "pods" {
		t.Fatalf("preview pods: created %v ephemeral %v screen %s", created, s.Ephemeral(pods), s.SelectedScreen())
	}
	nodes, _, closed := s.Show("nodes", true)
	if closed != pods || s.tab(pods) != nil {
		t.Fatalf("preview replace left pods, closed %q", closed)
	}
	if s.Selected() != nodes || s.Len() != 2 {
		t.Fatalf("strip %+v selected %s", screens(s), s.Selected())
	}
	again, created, _ := s.Show("nodes", true)
	if created || again != nodes {
		t.Fatal("same screen opened a second tab")
	}
	if _, created, _ := s.Show("nodes", false); created || s.Ephemeral(nodes) {
		t.Fatal("double click did not pin the open tab")
	}
}

func TestTabInsertsBesideSelected(t *testing.T) {
	s := newTabStrip()
	pods, _, _ := s.Show("pods", false)
	s.Select(s.At(0))
	nodes, _, _ := s.Show("nodes", false)
	got := screens(s)
	if strings.Join(got, ",") != "overview,nodes,pods" {
		t.Fatalf("order %v, pods %s nodes %s", got, pods, nodes)
	}
}

func TestTabCloseRestoresHistory(t *testing.T) {
	s := newTabStrip()
	pods, _, _ := s.Show("pods", false)
	deps, _, _ := s.Show("deployments", false)
	if s.Close(deps) != pods || s.SelectedScreen() != "pods" {
		t.Fatalf("close selected went to %s", s.SelectedScreen())
	}
	svcs, _, _ := s.Show("services", false)
	s.Select(pods)
	if s.Close(svcs) != pods {
		t.Fatal("closing a background tab changed the selection")
	}
	if len(s.CloseRight(pods)) != 0 {
		t.Fatal("right of the last tab was not empty")
	}
	s.Show("events", false)
	dead := s.CloseRight(pods)
	if len(dead) != 1 || s.HasRight(pods) {
		t.Fatalf("close right left %v", screens(s))
	}
	if others := s.CloseOthers(pods); len(others) != 1 || s.Len() != 1 || s.Selected() != pods {
		t.Fatalf("close others left %v", screens(s))
	}
	if s.Close(pods) != "" || s.Len() != 0 {
		t.Fatal("closing the last tab left one")
	}
	if s.Next() != "" || s.At(0) != "" || s.At(-1) != "" {
		t.Fatal("empty strip still had a step")
	}
}

func TestTabSwitchWraps(t *testing.T) {
	s := newTabStrip()
	s.Show("pods", false)
	s.Show("nodes", false)
	if s.Screen(s.Next()) != "overview" || s.Screen(s.Prev()) != "pods" {
		t.Fatalf("wrap from %s next %s prev %s", s.SelectedScreen(), s.Screen(s.Next()), s.Screen(s.Prev()))
	}
	if s.Screen(s.At(0)) != "overview" || s.Screen(s.At(-1)) != "nodes" {
		t.Fatalf("index map %v", screens(s))
	}
}

func TestShowScreenKeepsRow(t *testing.T) {
	a := New(cluster.NewSession())
	a.showScreen("pods", false)
	a.row = 3
	a.logLines = []LogLine{{Pod: "web", Text: "ready"}}
	a.showScreen("deployments", true)
	if a.row != 0 || len(a.logLines) != 0 {
		t.Fatalf("new tab inherited pods state row %d lines %d", a.row, len(a.logLines))
	}
	a.showScreen("pods", true)
	if a.row != 3 || len(a.logLines) != 1 || a.logLines[0].Text != "ready" {
		t.Fatalf("pods tab lost its place: row %d lines %+v", a.row, a.logLines)
	}
	if a.Selected() != "pods" {
		t.Fatalf("selected %s", a.Selected())
	}
}

func TestPreviewClickReplacesUnpinnedTab(t *testing.T) {
	a := New(cluster.NewSession())
	a.showScreen("pods", true)
	a.showScreen("nodes", true)
	if a.tabs.Len() != 2 || a.Selected() != "nodes" {
		t.Fatalf("tabs %v selected %s", screens(a.tabs), a.Selected())
	}
	if _, ok := a.tabs.byScreen("pods"); ok {
		t.Fatal("preview pods stayed open")
	}
}

func TestClosingLastTabRestoresOverview(t *testing.T) {
	a := New(cluster.NewSession())
	a.showScreen("pods", false)
	a.closeTab(a.tabs.Selected())
	if a.Selected() != "overview" || a.tabs.Len() != 1 {
		t.Fatalf("after close: %s tabs %v", a.Selected(), screens(a.tabs))
	}
}

func TestTabStripDrawsOpenScreens(t *testing.T) {
	a := New(cluster.NewSession())
	tt := ui.NewTester(a.view, 1100, 720)
	if n := countText(tt, "Overview"); n < 2 {
		t.Fatalf("overview tab missing, texts %q", tt.Texts())
	}
	if err := tt.Click("Pods"); err != nil {
		t.Fatal(err)
	}
	if a.Selected() != "pods" {
		t.Fatalf("click selected %s", a.Selected())
	}
	if n := countText(tt, "Pods"); n < 2 {
		t.Fatalf("pods tab missing, texts %q", tt.Texts())
	}
	if n := countText(tt, "Overview"); n < 2 {
		t.Fatalf("overview tab was replaced, texts %q", tt.Texts())
	}
}

func TestShowDetailLeavesThePreview(t *testing.T) {
	s := newTabStrip()
	pods, _, _ := s.Show("pods", true)
	id, created := s.ShowDetail("logs", "logs\x00Pod/ns/web", "Logs: web", "pods")
	if !created || s.tab(pods) == nil || !s.Ephemeral(pods) || s.Selected() != id {
		t.Fatalf("preview replaced: present %v ephemeral %v selected %s", s.tab(pods) != nil, s.Ephemeral(pods), s.Selected())
	}
	again, created := s.ShowDetail("logs", "logs\x00Pod/ns/web", "Logs: web", "pods")
	if created || again != id || s.Len() != 3 {
		t.Fatalf("reopen created %v id %s len %d", created, again, s.Len())
	}
}

func TestMoveReordersAndPins(t *testing.T) {
	s := newTabStrip()
	pods, _, _ := s.Show("pods", true)
	nodes, _, _ := s.Show("nodes", false)
	if !s.Move(pods, 2) {
		t.Fatal("move")
	}
	vis := s.Visible()
	if len(vis) != 3 || vis[0].screen != "overview" || vis[1].id != nodes || vis[2].id != pods || s.Ephemeral(pods) {
		t.Fatalf("order %v ephemeral %v", screens(s), s.Ephemeral(pods))
	}
	if s.Move(pods, 2) {
		t.Fatal("move onto itself")
	}
}

func TestDetailTabSurvivesTheRouter(t *testing.T) {
	a := New(cluster.NewSession())
	a.showScreen("pods", false)
	pods := a.active
	a.openDetail("logs", "Pod", "ns", "web", "Logs: web")
	logs := a.active
	if logs == pods || a.tabs.Len() != 3 || a.tabs.Detail(logs) == nil {
		t.Fatalf("logs tab active %s pods %s len %d", logs, pods, a.tabs.Len())
	}
	a.followRouter()
	if a.active != logs {
		t.Fatalf("router left logs for %s", a.tabs.Screen(a.active))
	}
	a.openDetail("logs", "Pod", "ns", "web", "Logs: web")
	if a.active != logs || a.tabs.Len() != 3 {
		t.Fatal("same logs opened a second tab")
	}
	a.activate(pods)
	if a.Selected() != "pods" || a.tabs.Detail(logs) == nil {
		t.Fatal("the list or the logs tab was lost")
	}
}

func TestDetailTabDrawsBesideTheList(t *testing.T) {
	a := New(cluster.NewSession())
	tt := ui.NewTester(a.view, 1100, 720)
	if err := tt.Click("Pods"); err != nil {
		t.Fatal(err)
	}
	a.openDetail("logs", "Pod", "ns", "web", "Logs: web")
	tt.Frame()
	if !tt.HasText("Logs: web") {
		t.Fatalf("logs tab missing, texts %q", tt.Texts())
	}
	if n := countText(tt, "Pods"); n < 2 {
		t.Fatalf("pods tab missing, texts %q", tt.Texts())
	}
}

func screens(s *tabStrip) []string {
	vis := s.Visible()
	out := make([]string, len(vis))
	for i, t := range vis {
		out[i] = t.screen
	}
	return out
}

func countText(tt *ui.Tester, s string) int {
	n := 0
	for _, text := range tt.Texts() {
		if text == s {
			n++
		}
	}
	return n
}
