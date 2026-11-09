package remote

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindRejectsNonTailscale(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "127.0.0.1", "192.168.1.8", "::", ""} {
		if _, err := Bind(host); err == nil {
			t.Fatalf("Bind(%q) accepted", host)
		}
	}
	addr, err := Bind("100.64.1.2")
	if err != nil || addr != "100.64.1.2:47623" {
		t.Fatalf("tailscale bind = %q %v", addr, err)
	}
}

type fakeBackend struct {
	logsContainer    string
	workloadContext  string
	workloadResource string
	mutationContext  string
}

func (f *fakeBackend) Contexts(context.Context) ([]Context, error) {
	return []Context{{Name: "dev", Cluster: "development", Current: true}}, nil
}

func (f *fakeBackend) ContextOverview(ctx context.Context, name string) (Overview, error) {
	body, err := f.Overview(ctx)
	body.Context = name
	return body, err
}

func (f *fakeBackend) Overview(context.Context) (Overview, error) {
	return Overview{Context: "dev", Nodes: "1/1", Pods: "2/3", Warnings: "0", CPU: "metrics unavailable", Memory: "metrics unavailable"}, nil
}

func (f *fakeBackend) ContextNodes(context.Context, string) ([]Node, error) {
	return []Node{{Name: "worker", Ready: true}}, nil
}

func (f *fakeBackend) ContextNamespaces(context.Context, string) ([]Namespace, error) {
	return []Namespace{{Name: "default", Phase: "Active"}}, nil
}

func (f *fakeBackend) ContextEvents(context.Context, string) ([]Event, error) {
	return []Event{{Namespace: "default", Name: "scheduled", Type: "Normal"}}, nil
}

func (f *fakeBackend) ContextLeases(context.Context, string) ([]Lease, error) {
	return []Lease{{Namespace: "kube-node-lease", Name: "worker", Holder: "worker"}}, nil
}

func (f *fakeBackend) PortForwards(context.Context) ([]PortForward, error) {
	return []PortForward{{Namespace: "default", Pod: "web", Local: 8080, Remote: 80}}, nil
}

func (f *fakeBackend) ContextWorkloads(_ context.Context, name, resource string) ([]Workload, error) {
	f.workloadContext = name
	f.workloadResource = resource
	return []Workload{{
		Resource: "pods", Kind: "Pod", Namespace: "ns", Name: "web", Status: "Healthy",
		Ready: 1, Desired: 1, Containers: []WorkloadContainer{{Name: "app", Image: "web:v1", Ready: true}},
		Owned: true,
	}}, nil
}

func (f *fakeBackend) ContextLogs(_ context.Context, contextName, _, _, container string) (io.ReadCloser, error) {
	f.workloadContext = contextName
	f.logsContainer = container
	if container == "" {
		return nil, &StatusError{Status: http.StatusConflict, Msg: "container choice is ambiguous", Body: map[string]any{"containers": []string{"a", "b"}}}
	}
	return io.NopCloser(strings.NewReader("line\n")), nil
}

func (f *fakeBackend) RestartOwnedPod(_ context.Context, contextName, _, _ string) error {
	f.mutationContext = contextName
	return nil
}

func (f *fakeBackend) DeletePod(_ context.Context, contextName, _, _ string) error {
	f.mutationContext = contextName
	return nil
}
func (f *fakeBackend) ArgoApplications(context.Context) ([]ArgoApp, error) {
	return []ArgoApp{{Name: "shop", Namespace: "argo", Sync: "Synced", Health: "Healthy"}}, nil
}

func TestRoutes(t *testing.T) {
	// The test listener is loopback. Production bind is Bind, which rejects it.
	f := &fakeBackend{}
	srv := httptest.NewServer(Handler(f))
	t.Cleanup(srv.Close)
	if _, err := Bind("127.0.0.1"); err == nil {
		t.Fatal("production bind accepted loopback")
	}
	for _, path := range []string{"/v1/overview?context=dev", "/v1/pods"} {
		response, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("legacy route %s status %d", path, response.StatusCode)
		}
	}

	ov := getJSON[Overview](t, srv.URL+"/v1/cluster/overview?context=dev")
	if ov.Context != "dev" || ov.CPU != "metrics unavailable" || ov.Nodes != "1/1" {
		t.Fatalf("%+v", ov)
	}
	nodes := getJSON[[]Node](t, srv.URL+"/v1/cluster/nodes?context=dev")
	if len(nodes) != 1 || nodes[0].Name != "worker" || !nodes[0].Ready {
		t.Fatalf("%+v", nodes)
	}
	namespaces := getJSON[[]Namespace](t, srv.URL+"/v1/cluster/namespaces?context=dev")
	events := getJSON[[]Event](t, srv.URL+"/v1/cluster/events?context=dev")
	leases := getJSON[[]Lease](t, srv.URL+"/v1/cluster/leases?context=dev")
	forwards := getJSON[[]PortForward](t, srv.URL+"/v1/port-forwards")
	if len(namespaces) != 1 || len(events) != 1 || len(leases) != 1 || len(forwards) != 1 {
		t.Fatalf("cluster routes missing rows: namespaces=%+v events=%+v leases=%+v forwards=%+v", namespaces, events, leases, forwards)
	}
	workloads := getJSON[[]Workload](t, srv.URL+"/v1/workloads/deployments?context=dev")
	if len(workloads) != 1 || workloads[0].Name != "web" || f.workloadContext != "dev" || f.workloadResource != "deployments" {
		t.Fatalf("workloads=%+v context=%q resource=%q", workloads, f.workloadContext, f.workloadResource)
	}
	apps := getJSON[[]ArgoApp](t, srv.URL+"/v1/argo/applications")
	if len(apps) != 1 || apps[0].Sync != "Synced" {
		t.Fatalf("%+v", apps)
	}
	raw, _ := json.Marshal(apps[0])
	if strings.Contains(string(raw), "operation") {
		t.Fatalf("argo row has an operation field: %s", raw)
	}

	res, err := http.Get(srv.URL + "/v1/workloads/pods/ns/web/logs?context=dev&container=app")
	if err != nil {
		t.Fatal(err)
	}
	logBody, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK || string(logBody) != "line\n" ||
		f.workloadContext != "dev" || f.logsContainer != "app" {
		t.Fatalf("logs status=%d body=%q context=%q container=%q err=%v", res.StatusCode, logBody, f.workloadContext, f.logsContainer, err)
	}
	res, err = http.Post(srv.URL+"/v1/workloads/pods/ns/web/restart?context=dev", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || f.mutationContext != "dev" {
		t.Fatalf("restart status=%d context=%q", res.StatusCode, f.mutationContext)
	}
	res, err = http.Post(srv.URL+"/v1/argo/sync", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("sync status %d", res.StatusCode)
	}
	res, err = http.Post(srv.URL+"/v1/apply", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("apply status %d", res.StatusCode)
	}
}

func TestLogRouteFlushesStreamedLines(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/workloads/pods/ns/web/logs?context=dev&container=app",
		nil,
	)
	Handler(&fakeBackend{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "line\n" || !recorder.Flushed {
		t.Fatalf("status=%d body=%q flushed=%v", recorder.Code, recorder.Body.String(), recorder.Flushed)
	}
}

func TestChooseContainer(t *testing.T) {
	name, _, err := ChooseContainer([]string{"istio-proxy", "app"})
	if err != nil || name != "app" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	_, list, err := ChooseContainer([]string{"app", "proxy"})
	if err == nil || len(list) != 2 {
		t.Fatalf("ambiguous err=%v list=%v", err, list)
	}
}

func getJSON[T any](t *testing.T, url string) T {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s status %d", url, res.StatusCode)
	}
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestHandlerOnLoopbackListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: Handler(&fakeBackend{})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	res, err := http.Get("http://" + ln.Addr().String() + "/v1/workloads/pods?context=dev")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}
