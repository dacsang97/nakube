package cluster

import (
	"strings"
	"testing"
	"time"
)

func TestPodQueriesCPU(t *testing.T) {
	got := PodQueries(MetricCPU, "shop", []string{"web-1"}, "", time.Minute)
	want := []MetricQuery{
		{Name: "Usage", Expr: `sum(rate(container_cpu_usage_seconds_total{namespace="shop",pod=~"^(web-1)$",container!="",container!="POD",image!=""}[120s]))`},
		{Name: "Requests", Expr: `sum(kube_pod_container_resource_requests{namespace="shop",pod=~"^(web-1)$",resource="cpu"})`, Line: true},
		{Name: "Limits", Expr: `sum(kube_pod_container_resource_limits{namespace="shop",pod=~"^(web-1)$",resource="cpu"})`, Line: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d queries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("query %d\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

func TestPodQueriesContainer(t *testing.T) {
	got := PodQueries(MetricMemory, "shop", []string{"web-1"}, "app", time.Minute)
	if !strings.Contains(got[0].Expr, `container="app",image!=""`) {
		t.Errorf("usage ignores the container: %s", got[0].Expr)
	}
	if !strings.Contains(got[1].Expr, `container="app",resource="memory"`) {
		t.Errorf("requests ignore the container: %s", got[1].Expr)
	}
	net := PodQueries(MetricNetwork, "shop", []string{"web-1"}, "app", time.Minute)
	if strings.Contains(net[0].Expr, "container=") {
		t.Errorf("network is pod level: %s", net[0].Expr)
	}
}

func TestPodQueriesQuotesNames(t *testing.T) {
	got := PodQueries(MetricFilesystem, "a", []string{"x.y", "z"}, "", time.Minute)
	if !strings.Contains(got[0].Expr, `pod=~"^(x\\.y|z)$"`) {
		t.Errorf("pod names are not quoted: %s", got[0].Expr)
	}
	if len(got) != 3 || !got[1].Line || !got[2].Line || got[0].Line {
		t.Errorf("filesystem series: %+v", got)
	}
}

func TestPodQueriesEmpty(t *testing.T) {
	if q := PodQueries(MetricCPU, "a", nil, "", time.Minute); q != nil {
		t.Errorf("no pods must be no query, got %v", q)
	}
	if q := PodQueries("disk", "a", []string{"p"}, "", time.Minute); q != nil {
		t.Errorf("unknown tab must be no query, got %v", q)
	}
}

func TestMetricStep(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		time.Minute:        time.Minute,
		time.Hour:          time.Minute,
		6 * time.Hour:      6 * time.Minute,
		24 * time.Hour:     24 * time.Minute,
		7 * 24 * time.Hour: 168 * time.Minute,
	}
	for rng, want := range cases {
		if got := MetricStep(rng); got != want {
			t.Errorf("MetricStep(%v) = %v, want %v", rng, got, want)
		}
	}
	if got := RateWindow(6 * time.Minute); got != "720s" {
		t.Errorf("RateWindow = %s", got)
	}
}
