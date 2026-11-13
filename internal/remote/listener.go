package remote

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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

var tailscaleCGNAT = mustCIDR("100.64.0.0/10")

// Endpoint is one address encoded in the pairing QR and shown by Settings.
type Endpoint struct {
	Network string `json:"network"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

// NewPairingToken returns a bearer credential suitable for one desktop.
func NewPairingToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
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

// Pod is a phone list row. It has no secret values.
type Pod struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Phase      string   `json:"phase"`
	Containers []string `json:"containers"`
	Owned      bool     `json:"owned"`
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

// ArgoApp is the phone Argo row. It has no operation the phone could post back.
type ArgoApp struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Sync      string `json:"sync"`
	Health    string `json:"health"`
}

// Context is a credential-free row in the mobile context catalog.
type Context struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	Current bool   `json:"current"`
}

// Backend is the desktop session surface the phone may call.
type Backend interface {
	Contexts(ctx context.Context) ([]Context, error)
	ContextOverview(ctx context.Context, name string) (Overview, error)
	Overview(ctx context.Context) (Overview, error)
	Pods(ctx context.Context) ([]Pod, error)
	Logs(ctx context.Context, namespace, name, container string) (io.ReadCloser, error)
	RestartOwnedPod(ctx context.Context, namespace, name string) error
	DeletePod(ctx context.Context, namespace, name string) error
	ArgoApplications(ctx context.Context) ([]ArgoApp, error)
}

// Authorized requires the token transferred in the pairing QR. The token is
// never accepted in a URL, where logs and browser history could retain it.
func Authorized(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// PrivateClients admits loopback tests, private LAN peers, and Tailscale
// peers. A wildcard listener therefore does not expose the API to a public
// interface even when a machine has one.
func PrivateClients(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || (!ip.IsLoopback() && !ip.IsPrivate() && !isTailscale(ip)) {
			http.Error(w, "private network required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler serves authenticated phone routes. Sync, apply, exec, and Helm are absent.
func Handler(token string, b Backend) http.Handler {
	return Authorized(token, routes(b))
}

func routes(b Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/contexts", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.Contexts(r.Context())
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/overview", func(w http.ResponseWriter, r *http.Request) {
		if name := r.URL.Query().Get("context"); name != "" {
			body, err := b.ContextOverview(r.Context(), name)
			writeJSON(w, body, err)
			return
		}
		body, err := b.Overview(r.Context())
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/pods", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.Pods(r.Context())
		writeJSON(w, body, err)
	})
	mux.HandleFunc("GET /v1/pods/{namespace}/{name}/logs", func(w http.ResponseWriter, r *http.Request) {
		stream, err := b.Logs(r.Context(), r.PathValue("namespace"), r.PathValue("name"), r.URL.Query().Get("container"))
		if err != nil {
			writeErr(w, err)
			return
		}
		defer stream.Close()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.Copy(w, stream)
	})
	mux.HandleFunc("POST /v1/pods/{namespace}/{name}/restart", func(w http.ResponseWriter, r *http.Request) {
		err := b.RestartOwnedPod(r.Context(), r.PathValue("namespace"), r.PathValue("name"))
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
	mux.HandleFunc("POST /v1/pods/{namespace}/{name}/delete", func(w http.ResponseWriter, r *http.Request) {
		err := b.DeletePod(r.Context(), r.PathValue("namespace"), r.PathValue("name"))
		writeJSON(w, map[string]bool{"ok": err == nil}, err)
	})
	mux.HandleFunc("GET /v1/argo/applications", func(w http.ResponseWriter, r *http.Request) {
		body, err := b.ArgoApplications(r.Context())
		writeJSON(w, body, err)
	})
	return mux
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

// Listen serves authenticated phone requests on LAN and Tailscale interfaces.
// PrivateClients still rejects a request whose source is not private.
func Listen(ctx context.Context, handler http.Handler) (*http.Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: PrivateClients(handler), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}
