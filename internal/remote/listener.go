package remote

import (
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
	Context  string `json:"context"`
	Nodes    string `json:"nodes"`
	Pods     string `json:"pods"`
	Warnings string `json:"warnings"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

// ArgoApp is the phone Argo row. It has no operation the phone could post back.
type ArgoApp struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Sync      string `json:"sync"`
	Health    string `json:"health"`
}

// Backend is the desktop session surface the phone may call.
type Backend interface {
	Overview(ctx context.Context) (Overview, error)
	Pods(ctx context.Context) ([]Pod, error)
	Logs(ctx context.Context, namespace, name, container string) (io.ReadCloser, error)
	RestartOwnedPod(ctx context.Context, namespace, name string) error
	DeletePod(ctx context.Context, namespace, name string) error
	ArgoApplications(ctx context.Context) ([]ArgoApp, error)
}

// Handler serves the phone routes. Sync, apply, exec, and Helm are absent.
func Handler(b Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/overview", func(w http.ResponseWriter, r *http.Request) {
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
