package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
)

func TestPhoneContextOverviewUsesSelectedCluster(t *testing.T) {
	deleted := make(chan string, 1)
	kube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"mobile-node","labels":{"node-role.kubernetes.io/worker":""}},"status":{"allocatable":{"cpu":"2","memory":"4Gi"},"conditions":[{"type":"Ready","status":"True"}],"nodeInfo":{"kubeletVersion":"v1.35.0"}}}]}`)
		case "/api/v1/namespaces":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"NamespaceList","items":[{"metadata":{"name":"default"},"status":{"phase":"Active"}}]}`)
		case "/api/v1/pods":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"web","namespace":"default","ownerReferences":[{"apiVersion":"apps/v1","kind":"Deployment","name":"web","uid":"1","controller":true}]},"spec":{"containers":[{"name":"app","image":"web:v1"}]},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}],"containerStatuses":[{"name":"app","image":"web:v1","imageID":"web@sha256:1","ready":true,"restartCount":2}]}}]}`)
		case "/api/v1/namespaces/default/pods/web":
			if r.Method == http.MethodDelete {
				deleted <- r.URL.Path
				fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success","code":200}`)
				return
			}
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default","ownerReferences":[{"apiVersion":"apps/v1","kind":"Deployment","name":"web","uid":"1","controller":true}]},"spec":{"containers":[{"name":"app","image":"web:v1"}]},"status":{"phase":"Running"}}`)
		case "/api/v1/namespaces/default/pods/bare":
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"bare","namespace":"default"},"spec":{"containers":[{"name":"app","image":"web:v1"}]},"status":{"phase":"Running"}}`)
		case "/apis/apps/v1/deployments":
			fmt.Fprint(w, `{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"web","namespace":"default"},"spec":{"replicas":2,"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"app","image":"web:v1"}]}}},"status":{"readyReplicas":1,"updatedReplicas":2,"availableReplicas":1}}]}`)
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
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
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
	response, err = http.Get(server.URL + "/v1/cluster/overview?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	var body remote.Overview
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || body.Context != "mobile/team" || body.Nodes != "1/1" || body.CPU != "metrics unavailable" {
		t.Fatalf("status=%d overview=%+v", response.StatusCode, body)
	}
	response.Body.Close()
	response, err = http.Get(server.URL + "/v1/cluster/nodes?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	var nodes []remote.Node
	if err := json.NewDecoder(response.Body).Decode(&nodes); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(nodes) != 1 || nodes[0].Name != "mobile-node" ||
		!nodes[0].Ready || nodes[0].CPU.Allocatable != 2000 || nodes[0].CPU.Used != nil {
		t.Fatalf("status=%d nodes=%+v", response.StatusCode, nodes)
	}
	response, err = http.Get(server.URL + "/v1/cluster/namespaces?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	var namespaces []remote.Namespace
	if err := json.NewDecoder(response.Body).Decode(&namespaces); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(namespaces) != 1 || namespaces[0].Name != "default" ||
		namespaces[0].Labels == nil {
		t.Fatalf("status=%d namespaces=%+v", response.StatusCode, namespaces)
	}
	response, err = http.Get(server.URL + "/v1/workloads/deployments?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	rawWorkloads, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var workloads []remote.Workload
	if err := json.Unmarshal(rawWorkloads, &workloads); err != nil {
		t.Fatalf("status=%d body=%q decode=%v", response.StatusCode, rawWorkloads, err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(workloads) != 1 || workloads[0].Name != "web" ||
		workloads[0].Ready != 1 || workloads[0].Desired != 2 {
		t.Fatalf("status=%d workloads=%+v", response.StatusCode, workloads)
	}
	response, err = http.Post(
		server.URL+"/v1/workloads/pods/default/web/restart?context=mobile%2Fteam",
		"application/json", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	restartBody, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("restart status=%d body=%q read=%v", response.StatusCode, restartBody, readErr)
	}
	if got := <-deleted; got != "/api/v1/namespaces/default/pods/web" {
		t.Fatalf("deleted path %q", got)
	}
	response, err = http.Post(
		server.URL+"/v1/workloads/pods/default/bare/restart?context=mobile%2Fteam",
		"application/json", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict || len(deleted) != 0 {
		t.Fatalf("bare restart status=%d delete-count=%d", response.StatusCode, len(deleted))
	}
	response, err = http.Get(server.URL + "/v1/workloads/secrets?context=mobile%2Fteam")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown workload status=%d", response.StatusCode)
	}
	selected, err := os.ReadFile(state)
	if err != nil || string(selected) != "desktop\n" || session.Current() != "desktop" {
		t.Fatalf("desktop changed: state=%q err=%v", selected, err)
	}
}
