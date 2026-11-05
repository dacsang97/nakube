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
	logsContainer string
}

func (f *fakeBackend) Overview(context.Context) (Overview, error) {
	return Overview{Context: "dev", Nodes: "1/1", Pods: "2/3", Warnings: "0", CPU: "metrics unavailable", Memory: "metrics unavailable"}, nil
}

func (f *fakeBackend) Pods(context.Context) ([]Pod, error) {
	return []Pod{{Namespace: "ns", Name: "web", Phase: "Running", Containers: []string{"app", "istio-proxy"}, Owned: true}}, nil
}

func (f *fakeBackend) Logs(_ context.Context, _, _, container string) (io.ReadCloser, error) {
	f.logsContainer = container
	if container == "" {
		return nil, &StatusError{Status: http.StatusConflict, Msg: "container choice is ambiguous", Body: map[string]any{"containers": []string{"a", "b"}}}
	}
	return io.NopCloser(strings.NewReader("line\n")), nil
}

func (f *fakeBackend) RestartOwnedPod(context.Context, string, string) error { return nil }
func (f *fakeBackend) DeletePod(context.Context, string, string) error       { return nil }
func (f *fakeBackend) ArgoApplications(context.Context) ([]ArgoApp, error) {
	return []ArgoApp{{Name: "shop", Namespace: "argo", Sync: "Synced", Health: "Healthy"}}, nil
}

func TestRoutes(t *testing.T) {
	// The test listener is loopback. Production bind is Bind, which rejects it.
	srv := httptest.NewServer(Handler(&fakeBackend{}))
	t.Cleanup(srv.Close)
	if _, err := Bind("127.0.0.1"); err == nil {
		t.Fatal("production bind accepted loopback")
	}

	ov := getJSON[Overview](t, srv.URL+"/v1/overview")
	if ov.CPU != "metrics unavailable" || ov.Nodes != "1/1" {
		t.Fatalf("%+v", ov)
	}
	apps := getJSON[[]ArgoApp](t, srv.URL+"/v1/argo/applications")
	if len(apps) != 1 || apps[0].Sync != "Synced" {
		t.Fatalf("%+v", apps)
	}
	raw, _ := json.Marshal(apps[0])
	if strings.Contains(string(raw), "operation") {
		t.Fatalf("argo row has an operation field: %s", raw)
	}

	res, err := http.Post(srv.URL+"/v1/argo/sync", "application/json", nil)
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
	res, err := http.Get("http://" + ln.Addr().String() + "/v1/pods")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}
