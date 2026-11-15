package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dacsang97/nakube/internal/cluster"
)

const Port = 47623

// tailscaleCGNAT is the IPv4 range Tailscale assigns.
var tailscaleCGNAT = mustCIDR("100.64.0.0/10")

// Bind refuses any address that is not a Tailscale IP. Production listens
// only on the address from `tailscale ip -4`.
func Bind(host string) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil || !isTailscale(ip) {
		return "", fmt.Errorf("refusing non-tailscale bind %q", host)
	}
	return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", Port)), nil
}

func isTailscale(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		return tailscaleCGNAT.Contains(v4)
	}
	// Tailscale IPv6 ULA prefix fd7a:115c:a1e0::/48.
	if len(ip) == net.IPv6len && ip[0] == 0xfd && ip[1] == 0x7a && ip[2] == 0x11 && ip[3] == 0x5c && ip[4] == 0xa1 && ip[5] == 0xe0 {
		return true
	}
	return false
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// Workload is a safe mobile projection shared by the Workloads inventory and
// each designed workload-kind screen.
type Workload struct {
	Resource   string              `json:"resource"`
	Kind       string              `json:"kind"`
	Namespace  string              `json:"namespace"`
	Name       string              `json:"name"`
	Status     string              `json:"status"`
	Ready      int32               `json:"ready"`
	Desired    int32               `json:"desired"`
	CreatedAt  time.Time           `json:"createdAt"`
	Containers []WorkloadContainer `json:"containers"`
	Facts      []WorkloadFact      `json:"facts"`
	Owned      bool                `json:"owned"`
}

type WorkloadContainer struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Ready    bool   `json:"ready"`
	Restarts int32  `json:"restarts"`
}

type WorkloadFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Overview matches the desktop cards, including the empty metrics state.
type Overview struct {
	Context  string          `json:"context"`
	Nodes    string          `json:"nodes"`
	Pods     string          `json:"pods"`
	Warnings string          `json:"warnings"`
	CPU      string          `json:"cpu"`
	Memory   string          `json:"memory"`
	Charts   *OverviewCharts `json:"charts,omitempty"`
}

// ResourceReading keeps an unavailable metric distinct from a measured zero.
type ResourceReading struct {
	Used        *int64 `json:"used"`
	Allocatable int64  `json:"allocatable"`
}

// OverviewCharts contains numeric observations, never parsed display strings.
type OverviewCharts struct {
	NodesReady  int             `json:"nodesReady"`
	NodesTotal  int             `json:"nodesTotal"`
	PodsRunning int             `json:"podsRunning"`
	PodsTotal   int             `json:"podsTotal"`
	CPU         ResourceReading `json:"cpu"`
	Memory      ResourceReading `json:"memory"`
}

// ArgoApplication is the read-only mobile projection of an Argo CD Application.
// It intentionally contains no operation or credential-bearing fields.
type ArgoApplication struct {
	Name             string         `json:"name"`
	Namespace        string         `json:"namespace"`
	Project          string         `json:"project"`
	Sync             string         `json:"sync"`
	Health           string         `json:"health"`
	HealthMessage    string         `json:"healthMessage"`
	Revision         string         `json:"revision"`
	TargetRevision   string         `json:"targetRevision"`
	Destination      string         `json:"destination"`
	Policy           string         `json:"policy"`
	Source           string         `json:"source"`
	Sources          []ArgoSource   `json:"sources"`
	Resources        []ArgoResource `json:"resources"`
	History          []ArgoHistory  `json:"history"`
	Conditions       []string       `json:"conditions"`
	OperationPhase   string         `json:"operationPhase"`
	OperationMessage string         `json:"operationMessage"`
	OperationBy      string         `json:"operationBy"`
	Images           []string       `json:"images"`
	SourceType       string         `json:"sourceType"`
	CreatedAt        time.Time      `json:"createdAt"`
}

type ArgoSource struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	Chart          string `json:"chart"`
	TargetRevision string `json:"targetRevision"`
	Ref            string `json:"ref"`
}

type ArgoResource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Sync      string `json:"sync"`
	Health    string `json:"health"`
	Hook      bool   `json:"hook"`
}

type ArgoHistory struct {
	ID       int64  `json:"id"`
	Revision string `json:"revision"`
	By       string `json:"by"`
}

type ArgoApplicationSet struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Generators  string    `json:"generators"`
	Project     string    `json:"project"`
	Destination string    `json:"destination"`
	Conditions  []string  `json:"conditions"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ArgoProject struct {
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	Description  string    `json:"description"`
	SourceRepos  []string  `json:"sourceRepos"`
	Destinations []string  `json:"destinations"`
	Roles        int       `json:"roles"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ArgoRepository struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	Project   string `json:"project"`
}

type ArgoCluster struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}

type ArgoSyncRequest struct {
	Revision string `json:"revision"`
	Strategy string `json:"strategy"`
	DryRun   bool   `json:"dryRun"`
	Prune    bool   `json:"prune"`
}

type ArgoRefreshRequest struct {
	Kind string `json:"kind"`
}

type ArgoRollbackRequest struct {
	ID    int64 `json:"id"`
	Prune bool  `json:"prune"`
}

type ArgoDeleteRequest struct {
	Cascade string `json:"cascade"`
}

type ArgoFleetRequest struct {
	Prune bool `json:"prune"`
}

// Context is a credential-free row in the mobile context catalog.
type Context struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	Current bool   `json:"current"`
}

// Node is a credential-free cluster node summary for the phone.
type Node struct {
	Name       string          `json:"name"`
	Ready      bool            `json:"ready"`
	Roles      []string        `json:"roles"`
	Version    string          `json:"version"`
	Conditions []string        `json:"conditions"`
	Taints     []string        `json:"taints"`
	CPU        ResourceReading `json:"cpu"`
	Memory     ResourceReading `json:"memory"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// Namespace is a cluster namespace summary for the phone.
type Namespace struct {
	Name       string            `json:"name"`
	Phase      string            `json:"phase"`
	Labels     map[string]string `json:"labels"`
	Finalizers []string          `json:"finalizers"`
	CreatedAt  time.Time         `json:"createdAt"`
}

// Event is a Kubernetes event summary for the phone.
type Event struct {
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Reason     string    `json:"reason"`
	Object     string    `json:"object"`
	APIVersion string    `json:"apiVersion"`
	Message    string    `json:"message"`
	Source     string    `json:"source"`
	Count      int32     `json:"count"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
}

// Lease is a coordination lease summary for the phone.
type Lease struct {
	Namespace       string     `json:"namespace"`
	Name            string     `json:"name"`
	Holder          string     `json:"holder"`
	RenewTime       *time.Time `json:"renewTime"`
	DurationSeconds *int32     `json:"durationSeconds"`
	Transitions     *int32     `json:"transitions"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// PortForward is one process-owned forward. It is read-only on the phone.
type PortForward struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Local     uint16 `json:"local"`
	Remote    uint16 `json:"remote"`
}

// Backend is the desktop session surface the phone may call.
type Backend interface {
	Contexts(ctx context.Context) ([]Context, error)
	ContextOverview(ctx context.Context, name string) (Overview, error)
	ContextNodes(ctx context.Context, name string) ([]Node, error)
	ContextNamespaces(ctx context.Context, name string) ([]Namespace, error)
	ContextEvents(ctx context.Context, name string) ([]Event, error)
	ContextLeases(ctx context.Context, name string) ([]Lease, error)
	PortForwards(ctx context.Context) ([]PortForward, error)
	ContextWorkloads(ctx context.Context, name, resource string) ([]Workload, error)
	ContextLogs(ctx context.Context, contextName, namespace, name, container string) (io.ReadCloser, error)
	RestartOwnedPod(ctx context.Context, contextName, namespace, name string) error
	DeletePod(ctx context.Context, contextName, namespace, name string) error
	ContextArgoApplications(ctx context.Context, name string) ([]ArgoApplication, error)
	ContextArgoApplicationSets(ctx context.Context, name string) ([]ArgoApplicationSet, error)
	ContextArgoProjects(ctx context.Context, name string) ([]ArgoProject, error)
	ContextArgoRepositories(ctx context.Context, name string) ([]ArgoRepository, error)
	ContextArgoClusters(ctx context.Context, name string) ([]ArgoCluster, error)
	SyncArgoApplication(ctx context.Context, contextName, namespace, name string, request ArgoSyncRequest) error
	RefreshArgoApplication(ctx context.Context, contextName, namespace, name string, request ArgoRefreshRequest) error
	RollbackArgoApplication(ctx context.Context, contextName, namespace, name string, request ArgoRollbackRequest) error
	DeleteArgoResource(ctx context.Context, contextName, resource, namespace, name string, request ArgoDeleteRequest) error
	SyncOutOfSyncArgoApplications(ctx context.Context, contextName string, request ArgoFleetRequest) error
	RefreshAllArgoApplications(ctx context.Context, contextName string) error
}

// Handler serves the phone routes. Argo exposes only designed reads and confirmed safe actions; apply, exec, Helm, YAML editing, and credential mutations are absent.
func Handler(b Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/contexts", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.Contexts(r.Context())
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/cluster/overview", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.ContextOverview(r.Context(), r.URL.Query().Get("context"))
		writeJSON(w, body, err)
	})
	handleContextList(mux, "GET /v1/cluster/nodes", b.ContextNodes)
	handleContextList(mux, "GET /v1/cluster/namespaces", b.ContextNamespaces)
	handleContextList(mux, "GET /v1/cluster/events", b.ContextEvents)
	handleContextList(mux, "GET /v1/cluster/leases", b.ContextLeases)
	mux.HandleFunc("GET /v1/port-forwards", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.PortForwards(r.Context())
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/workloads", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.ContextWorkloads(r.Context(), r.URL.Query().Get("context"), "")
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/workloads/{resource}", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.ContextWorkloads(r.Context(), r.URL.Query().Get("context"), r.PathValue("resource"))
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/workloads/pods/{namespace}/{name}/logs", func(w http.ResponseWriter, r *http.Request) {
		stream, err := b.ContextLogs(
			r.Context(), r.URL.Query().Get("context"), r.PathValue("namespace"),
			r.PathValue("name"), r.URL.Query().Get("container"),
		)
		if err != nil {
			writeErr(w, err)
			return
		}
		defer stream.Close()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.Copy(flushResponseWriter{ResponseWriter: w}, stream)
	})
	handlePodMutation(mux, "restart", b.RestartOwnedPod)
	handlePodMutation(mux, "delete", b.DeletePod)
	mux.HandleFunc("GET /v1/argo/applications", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.ContextArgoApplications(r.Context(), r.URL.Query().Get("context"))
		writeJSON(w, body, err)
	})
	handleContextList(mux, "GET /v1/argo/applicationsets", b.ContextArgoApplicationSets)
	handleContextList(mux, "GET /v1/argo/appprojects", b.ContextArgoProjects)
	handleContextList(mux, "GET /v1/argo/repositories", b.ContextArgoRepositories)
	handleContextList(mux, "GET /v1/argo/clusters", b.ContextArgoClusters)
	handleArgoMutation(mux, "POST /v1/argo/applications/{namespace}/{name}/sync", func(ctx context.Context, contextName, namespace, name string, request ArgoSyncRequest) error {
		return b.SyncArgoApplication(ctx, contextName, namespace, name, request)
	})
	handleArgoMutation(mux, "POST /v1/argo/applications/{namespace}/{name}/refresh", func(ctx context.Context, contextName, namespace, name string, request ArgoRefreshRequest) error {
		return b.RefreshArgoApplication(ctx, contextName, namespace, name, request)
	})
	handleArgoMutation(mux, "POST /v1/argo/applications/{namespace}/{name}/rollback", func(ctx context.Context, contextName, namespace, name string, request ArgoRollbackRequest) error {
		return b.RollbackArgoApplication(ctx, contextName, namespace, name, request)
	})
	handleArgoMutation(mux, "POST /v1/argo/applications/{namespace}/{name}/delete", func(ctx context.Context, contextName, namespace, name string, request ArgoDeleteRequest) error {
		return b.DeleteArgoResource(ctx, contextName, "applications", namespace, name, request)
	})
	handleArgoMutation(mux, "POST /v1/argo/applicationsets/{namespace}/{name}/delete", func(ctx context.Context, contextName, namespace, name string, request ArgoDeleteRequest) error {
		return b.DeleteArgoResource(ctx, contextName, "applicationsets", namespace, name, request)
	})
	handleArgoMutation(mux, "POST /v1/argo/appprojects/{namespace}/{name}/delete", func(ctx context.Context, contextName, namespace, name string, request ArgoDeleteRequest) error {
		return b.DeleteArgoResource(ctx, contextName, "appprojects", namespace, name, request)
	})
	mux.HandleFunc("POST /v1/argo/applications/sync-out-of-sync", func(w http.ResponseWriter, r *http.Request) {
		var request ArgoFleetRequest
		if err := decodeRequest(r, &request); err != nil {
			writeErr(w, err)
			return
		}
		err := b.SyncOutOfSyncArgoApplications(r.Context(), r.URL.Query().Get("context"), request)
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
	mux.HandleFunc("POST /v1/argo/applications/refresh-all", func(w http.ResponseWriter, r *http.Request) {
		if err := requireEmptyRequest(r); err != nil {
			writeErr(w, err)
			return
		}
		err := b.RefreshAllArgoApplications(r.Context(), r.URL.Query().Get("context"))
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
	return mux
}

func handleContextList[T any](
	mux *http.ServeMux,
	pattern string,
	load func(context.Context, string) ([]T, error),
) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		body, err := load(r.Context(), r.URL.Query().Get("context"))
		writeJSON(w, body, err)
	})
}

func handlePodMutation(
	mux *http.ServeMux,
	action string,
	mutate func(context.Context, string, string, string) error,
) {
	mux.HandleFunc("POST /v1/workloads/pods/{namespace}/{name}/"+action, func(w http.ResponseWriter, r *http.Request) {
		err := mutate(
			r.Context(), r.URL.Query().Get("context"), r.PathValue("namespace"), r.PathValue("name"),
		)
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
}

func handleArgoMutation[T any](
	mux *http.ServeMux,
	pattern string,
	mutate func(context.Context, string, string, string, T) error,
) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		var request T
		if err := decodeRequest(r, &request); err != nil {
			writeErr(w, err)
			return
		}
		err := mutate(
			r.Context(), r.URL.Query().Get("context"),
			r.PathValue("namespace"), r.PathValue("name"), request,
		)
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
}

const maxMutationBody = 1 << 20

func decodeRequest(r *http.Request, target any) error {
	body, err := boundedRequestBody(r)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return badRequest("invalid JSON request")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	if err := validateArgoRequest(target); err != nil {
		return badRequest(err.Error())
	}
	return nil
}

func requireEmptyRequest(r *http.Request) error {
	body, err := boundedRequestBody(r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) != 0 {
		return badRequest("request body must be empty")
	}
	return nil
}

func boundedRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxMutationBody+1))
	if err != nil {
		return nil, badRequest("could not read request body")
	}
	if len(body) > maxMutationBody {
		return nil, &StatusError{Status: http.StatusRequestEntityTooLarge, Msg: "request body is too large"}
	}
	return body, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return badRequest("request body must contain exactly one JSON value")
	}
	return nil
}

func validateArgoRequest(target any) error {
	switch request := target.(type) {
	case *ArgoSyncRequest:
		if request.Strategy != "" && request.Strategy != "hook" && request.Strategy != "apply" {
			return fmt.Errorf("strategy must be empty, hook, or apply")
		}
	case *ArgoRefreshRequest:
		if request.Kind != "normal" && request.Kind != "hard" {
			return fmt.Errorf("refresh kind must be normal or hard")
		}
	case *ArgoRollbackRequest:
		if request.ID <= 0 {
			return fmt.Errorf("rollback id must be greater than zero")
		}
	case *ArgoDeleteRequest:
		switch request.Cascade {
		case "", "foreground", "background", "non-cascading":
		default:
			return fmt.Errorf("cascade must be empty, foreground, background, or non-cascading")
		}
	}
	return nil
}

func badRequest(message string) error {
	return &StatusError{Status: http.StatusBadRequest, Msg: message}
}

type flushResponseWriter struct {
	http.ResponseWriter
}

func (w flushResponseWriter) Write(body []byte) (int, error) {
	n, err := w.ResponseWriter.Write(body)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}

// StatusError is an HTTP status the handler should return as-is.
type StatusError struct {
	Status int
	Msg    string
	Body   any
}

func (e *StatusError) Error() string { return e.Msg }

func writeJSON(w http.ResponseWriter, body any, err error) {
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, err error) {
	var se *StatusError
	if errors.As(err, &se) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(se.Status)
		payload := se.Body
		if payload == nil {
			payload = map[string]string{"error": se.Msg}
		}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// ChooseContainer picks the only non-sidecar container. More than one is ambiguous.
func ChooseContainer(names []string) (string, []string, error) {
	var apps []string
	for _, name := range names {
		if name != "" && !IsSidecar(name) {
			apps = append(apps, name)
		}
	}
	if len(apps) == 1 {
		return apps[0], names, nil
	}
	return "", names, &StatusError{
		Status: http.StatusConflict,
		Msg:    "container choice is ambiguous",
		Body:   map[string]any{"error": "container choice is ambiguous", "containers": names},
	}
}

// IsSidecar reports injected sidecars the phone must not stream by default.
func IsSidecar(name string) bool {
	switch name {
	case "istio-proxy", "istio-init", "istio-validation", "linkerd-proxy", "linkerd-init", "vault-agent", "consul-connect":
		return true
	default:
		return strings.HasPrefix(name, "istio-")
	}
}

// RestartRefused is returned for a bare pod. The phone should use delete.
var RestartRefused = &StatusError{Status: http.StatusConflict, Msg: "pod has no controller; use delete"}

// Owned reports whether a pod has a controller owner. Restart deletes only those.
func Owned(owners []cluster.Identity) bool {
	return len(owners) > 0
}

// ListenTailscale runs tailscale ip -4 and binds that address. A missing
// CLI or a down tailnet returns an error and does not listen.
func ListenTailscale(ctx context.Context, lookup func(context.Context) (string, error), handler http.Handler) (*http.Server, error) {
	ip, err := lookup(ctx)
	if err != nil {
		return nil, err
	}
	addr, err := Bind(strings.TrimSpace(ip))
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}
