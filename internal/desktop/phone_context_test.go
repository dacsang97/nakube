package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
)

func TestPhoneContextOverviewUsesSelectedCluster(t *testing.T) {
	kube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"mobile-node"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`)
		case "/api/v1/pods":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"PodList","items":[]}`)
		case "/api/v1/events":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"EventList","items":[]}`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`)
		}
	}))
	defer kube.Close()
	dir := t.TempDir()
	config := fmt.Sprintf("apiVersion: v1\nkind: Config\ncurrent-context: desktop\ncontexts:\n- name: desktop\n  context:\n    cluster: offline\n- name: mobile/team\n  context:\n    cluster: live\nclusters:\n- name: offline\n  cluster:\n    server: http://127.0.0.1:1\n- name: live\n  cluster:\n    server: %s\n", kube.URL)
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	session := cluster.NewSession()
	state := filepath.Join(dir, "selected")
	session.SetStatePath(state)
	if err := session.Use(context.Background(), "desktop"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(remote.Handler(Phone{App: New(session)}))
	defer server.Close()
	response, err := http.Get(server.URL + "/v1/contexts")
	if err != nil {
		t.Fatal(err)
	}
	var rows []remote.Context
	err = json.NewDecoder(response.Body).Decode(&rows)
	response.Body.Close()
	if err != nil || len(rows) != 2 || !rows[0].Current || rows[1].Name != "mobile/team" {
		t.Fatalf("catalog=%+v err=%v", rows, err)
	}
	response, err = http.Get(server.URL + "/v1/overview?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body remote.Overview
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || body.Context != "mobile/team" || body.Nodes != "1/1" || body.CPU != "metrics unavailable" {
		t.Fatalf("status=%d overview=%+v", response.StatusCode, body)
	}
	selected, err := os.ReadFile(state)
	if err != nil || string(selected) != "desktop\n" || session.Current() != "desktop" {
		t.Fatalf("desktop changed: state=%q err=%v", selected, err)
	}
}
