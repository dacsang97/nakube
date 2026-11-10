package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ErrPromNotFound means no known Prometheus service and no override.
var ErrPromNotFound = errors.New("prometheus not found")

// ErrPromListDenied means the account may not list services across
// namespaces, so the service cannot be found without an override.
var ErrPromListDenied = errors.New("cannot list services to find prometheus")

// PromSource is a Prometheus service reached through the API server's
// service proxy. Port is a number or a port name.
type PromSource struct {
	Namespace string
	Service   string
	Port      string
}

func (p PromSource) String() string {
	return p.Namespace + "/" + p.Service + ":" + p.Port
}

var promSourceRE = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?)/([a-z0-9]([-a-z0-9.]*[a-z0-9])?):([a-z0-9-]+)$`)

// ParsePromSource reads "namespace/service:port".
func ParsePromSource(s string) (PromSource, error) {
	m := promSourceRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return PromSource{}, fmt.Errorf("want namespace/service:port")
	}
	return PromSource{Namespace: m[1], Service: m[3], Port: m[5]}, nil
}

// PromPoint is one slot of a range. OK false is a gap, not a zero.
type PromPoint struct {
	At    time.Time
	Value float64
	OK    bool
}

// PromForbidden means the account may not use services/proxy.
func PromForbidden(err error) bool {
	return apierrors.IsForbidden(err)
}

// promCandidates are the service names the common installs create, in
// the order they are tried. kube-prometheus-stack names its service after
// the release, so both common release names are listed.
var promCandidates = []struct{ name, port string }{
	{"prometheus-operated", "9090"},
	{"kube-prometheus-stack-prometheus", "9090"},
	{"prometheus-kube-prometheus-prometheus", "9090"},
	{"prometheus-server", "80"},
	{"prometheus", "80"},
}

// DiscoverPrometheus returns override when it is set, else the first known
// service. Within one name, the monitoring namespace wins, then the
// namespace that sorts first, so the choice is stable.
func DiscoverPrometheus(ctx context.Context, cs kubernetes.Interface, override string) (PromSource, error) {
	if strings.TrimSpace(override) != "" {
		return ParsePromSource(override)
	}
	for _, cand := range promCandidates {
		list, err := cs.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + cand.name,
		})
		if apierrors.IsForbidden(err) {
			return PromSource{}, fmt.Errorf("%w: %w", ErrPromListDenied, err)
		}
		if err != nil {
			return PromSource{}, err
		}
		if len(list.Items) == 0 {
			continue
		}
		items := list.Items
		sort.Slice(items, func(i, j int) bool {
			mi, mj := items[i].Namespace == "monitoring", items[j].Namespace == "monitoring"
			if mi != mj {
				return mi
			}
			return items[i].Namespace < items[j].Namespace
		})
		svc := &items[0]
		return PromSource{Namespace: svc.Namespace, Service: svc.Name, Port: servicePort(svc, cand.port)}, nil
	}
	return PromSource{}, ErrPromNotFound
}

// servicePort keeps the expected port when the service has it, else its first.
func servicePort(svc *corev1.Service, want string) string {
	for _, p := range svc.Spec.Ports {
		if strconv.Itoa(int(p.Port)) == want {
			return want
		}
	}
	if len(svc.Spec.Ports) > 0 {
		return strconv.Itoa(int(svc.Spec.Ports[0].Port))
	}
	return want
}

// QueryRange runs one range query through the service proxy and returns
// one point per step from start to end. Every series in the result is
// summed into its slot. A slot nothing reported is a gap.
func QueryRange(ctx context.Context, client rest.Interface, src PromSource, query string, start, end time.Time, step time.Duration) ([]PromPoint, error) {
	if step <= 0 || !end.After(start) {
		return nil, errors.New("empty range")
	}
	body, err := client.Get().
		AbsPath("/api/v1/namespaces", src.Namespace, "services", src.Service+":"+src.Port, "proxy", "api/v1/query_range").
		Param("query", query).
		Param("start", strconv.FormatInt(start.Unix(), 10)).
		Param("end", strconv.FormatInt(end.Unix(), 10)).
		Param("step", strconv.Itoa(int(step/time.Second))).
		Do(ctx).
		Raw()
	if err != nil {
		return nil, err
	}
	return parseMatrix(body, start, end, step)
}

type promResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Values [][2]json.RawMessage `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func parseMatrix(body []byte, start, end time.Time, step time.Duration) ([]PromPoint, error) {
	var resp promResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("prometheus response: %w", err)
	}
	if resp.Status != "success" {
		if resp.Error == "" {
			resp.Error = "query failed"
		}
		return nil, errors.New(resp.Error)
	}
	n := int(end.Sub(start)/step) + 1
	points := make([]PromPoint, n)
	for i := range points {
		points[i].At = start.Add(time.Duration(i) * step)
	}
	for _, series := range resp.Data.Result {
		for _, pair := range series.Values {
			var ts float64
			var raw string
			if json.Unmarshal(pair[0], &ts) != nil || json.Unmarshal(pair[1], &raw) != nil {
				continue
			}
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			at := time.Unix(0, int64(ts*float64(time.Second)))
			i := int(math.Round(float64(at.Sub(start)) / float64(step)))
			if i < 0 || i >= n {
				continue
			}
			points[i].Value += v
			points[i].OK = true
		}
	}
	return points, nil
}
