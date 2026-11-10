package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestParsePromSource(t *testing.T) {
	src, err := ParsePromSource(" monitoring/prometheus-operated:9090 ")
	if err != nil || src != (PromSource{"monitoring", "prometheus-operated", "9090"}) {
		t.Fatalf("got %+v, %v", src, err)
	}
	if src.String() != "monitoring/prometheus-operated:9090" {
		t.Errorf("String = %s", src)
	}
	for _, bad := range []string{"", "prometheus:9090", "ns/svc", "NS/svc:1", "ns/svc:9090/x"} {
		if _, err := ParsePromSource(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func svc(ns, name string, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: port}}},
	}
}

func TestDiscoverPrometheus(t *testing.T) {
	ctx := context.Background()

	cs := k8sfake.NewClientset(
		svc("zz", "prometheus-operated", 9090),
		svc("monitoring", "prometheus-operated", 9090),
		svc("obs", "prometheus-server", 80),
	)
	got, err := DiscoverPrometheus(ctx, cs, "")
	if err != nil || got.String() != "monitoring/prometheus-operated:9090" {
		t.Fatalf("got %v, %v", got, err)
	}

	got, err = DiscoverPrometheus(ctx, cs, "obs/thanos:10902")
	if err != nil || got.String() != "obs/thanos:10902" {
		t.Fatalf("override: got %v, %v", got, err)
	}

	got, err = DiscoverPrometheus(ctx, k8sfake.NewClientset(svc("obs", "prometheus-server", 8080)), "")
	if err != nil || got.String() != "obs/prometheus-server:8080" {
		t.Fatalf("first port: got %v, %v", got, err)
	}

	if _, err := DiscoverPrometheus(ctx, k8sfake.NewClientset(), ""); !errors.Is(err, ErrPromNotFound) {
		t.Fatalf("empty cluster: %v", err)
	}

	denied := k8sfake.NewClientset()
	denied.PrependReactor("list", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "", errors.New("no"))
	})
	if _, err := DiscoverPrometheus(ctx, denied, ""); !errors.Is(err, ErrPromListDenied) {
		t.Fatalf("denied list: %v", err)
	}
}

func proxyClient(t *testing.T, h http.HandlerFunc) rest.Interface {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cs, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return cs.CoreV1().RESTClient()
}

func TestQueryRange(t *testing.T) {
	start := time.Unix(1000, 0)
	end := start.Add(4 * time.Minute)
	src := PromSource{"monitoring", "prometheus-operated", "9090"}
	client := proxyClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/monitoring/services/prometheus-operated:9090/proxy/api/v1/query_range" {
			t.Errorf("path %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("query") != "up" || q.Get("start") != "1000" || q.Get("end") != "1240" || q.Get("step") != "60" {
			t.Errorf("params %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		// Two series summed into slot 0; slot 2 missing; slot 3 NaN; one
		// point past the end.
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
			{"metric":{},"values":[[1000,"1"],[1060,"2"],[1180,"NaN"],[1300,"9"]]},
			{"metric":{},"values":[[1000,"0.5"],[1240,"4"]]}]}}`))
	})
	got, err := QueryRange(context.Background(), client, src, "up", start, end, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	want := []PromPoint{
		{At: start, Value: 1.5, OK: true},
		{At: start.Add(time.Minute), Value: 2, OK: true},
		{At: start.Add(2 * time.Minute)},
		{At: start.Add(3 * time.Minute)},
		{At: start.Add(4 * time.Minute), Value: 4, OK: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d points", len(got))
	}
	for i := range want {
		if !got[i].At.Equal(want[i].At) || got[i].Value != want[i].Value || got[i].OK != want[i].OK {
			t.Errorf("point %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestQueryRangeErrors(t *testing.T) {
	start := time.Unix(0, 0)
	end := start.Add(time.Hour)
	src := PromSource{"m", "p", "9090"}

	forbidden := proxyClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := QueryRange(context.Background(), forbidden, src, "up", start, end, time.Minute); !PromForbidden(err) {
		t.Errorf("403 should be forbidden, got %v", err)
	}

	bad := proxyClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error","error":"parse error"}`))
	})
	if _, err := QueryRange(context.Background(), bad, src, "up(", start, end, time.Minute); err == nil || err.Error() != "parse error" {
		t.Errorf("query error: %v", err)
	}
}
