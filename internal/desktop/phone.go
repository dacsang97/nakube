package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	qrcode "github.com/skip2/go-qrcode"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
	"github.com/dacsang97/nakube/internal/views/overview"
)

// Phone adapts the desktop session for authenticated private-network access.
type Phone struct {
	App *App
}

func (p Phone) Overview(context.Context) (remote.Overview, error) {
	a := p.App
	a.mu.Lock()
	shown := overview.From(a.nodes, a.pods, a.events, a.usage, a.usageErr, time.Now())
	current := a.session.Current()
	body := phoneOverview(current, shown, a.usage)
	a.mu.Unlock()
	return body, nil
}

func (p Phone) Pods(ctx context.Context) ([]remote.Pod, error) {
	cs, err := p.App.session.Clientset()
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]remote.Pod, 0, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		names := make([]string, 0, len(pod.Spec.Containers))
		for _, c := range pod.Spec.Containers {
			names = append(names, c.Name)
		}
		out = append(out, remote.Pod{
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			Phase:      string(pod.Status.Phase),
			Containers: names,
			Owned:      ownedPod(pod),
		})
	}
	return out, nil
}

func (p Phone) Logs(ctx context.Context, namespace, name, container string) (io.ReadCloser, error) {
	cs, err := p.App.session.Clientset()
	if err != nil {
		return nil, err
	}
	if container == "" {
		pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(pod.Spec.Containers))
		for _, c := range pod.Spec.Containers {
			names = append(names, c.Name)
		}
		chosen, _, err := remote.ChooseContainer(names)
		if err != nil {
			return nil, err
		}
		container = chosen
	}
	pr, pw := io.Pipe()
	go func() {
		err := p.App.session.StreamLogs(ctx, namespace, name, container, true, 200, func(lines []string) {
			_, _ = pw.Write([]byte(strings.Join(lines, "\n") + "\n"))
		})
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

func (p Phone) RestartOwnedPod(ctx context.Context, namespace, name string) error {
	cs, err := p.App.session.Clientset()
	if err != nil {
		return err
	}
	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if !ownedPod(pod) {
		return remote.RestartRefused
	}
	return p.DeletePod(ctx, namespace, name)
}

func (p Phone) DeletePod(ctx context.Context, namespace, name string) error {
	return p.App.session.Delete(ctx, cluster.Identity{
		GVR:       schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		Kind:      "Pod",
		Namespace: namespace,
		Name:      name,
	})
}

func (p Phone) ArgoApplications(ctx context.Context) ([]remote.ArgoApp, error) {
	apps, err := p.App.session.ListArgoApplications(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]remote.ArgoApp, 0, len(apps))
	for _, app := range apps {
		out = append(out, remote.ArgoApp{
			Name:      app.Name,
			Namespace: app.Namespace,
			Sync:      app.Sync,
			Health:    app.Health,
		})
	}
	return out, nil
}

func ownedPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

// StartPhoneListener publishes authenticated LAN and Tailscale endpoints.
// Either network is sufficient; unavailable Tailscale does not disable LAN.
func (a *App) StartPhoneListener(ctx context.Context) {
	endpoints := localLANEndpoints()
	if host, err := tailscaleDNSName(ctx); err == nil {
		endpoints = append(endpoints, remote.Endpoint{Network: "tailscale", Host: host, Port: remote.Port})
	}
	if len(endpoints) == 0 {
		a.setMobileConnection(nil, "", nil, errors.New("no private LAN or Tailscale address is available"))
		return
	}
	token, err := mobilePairingToken()
	if err != nil {
		a.setMobileConnection(nil, "", nil, errors.New("could not load the mobile pairing credential"))
		return
	}
	qr, err := mobilePairingBitmap(endpoints, token)
	if err != nil {
		a.setMobileConnection(nil, "", nil, errors.New("could not create the mobile pairing code"))
		return
	}
	handler := remote.Handler(token, Phone{App: a})
	if _, err = remote.Listen(ctx, handler); err != nil {
		a.setMobileConnection(nil, "", nil, errors.New("could not listen for mobile connections"))
		return
	}
	primary := endpointAddress(endpoints[0])
	a.setMobileConnection(endpoints, primary, qr, nil)
}

func (a *App) setMobileConnection(endpoints []remote.Endpoint, address string, qr *ui.Bitmap, err error) {
	a.mu.Lock()
	a.mobileAddress = address
	a.mobileEndpoints = append([]remote.Endpoint(nil), endpoints...)
	a.mobileQR = qr
	a.mobileErr = ""
	if err != nil {
		a.mobileErr = err.Error()
	}
	a.mu.Unlock()
	a.invalidate()
}

func endpointAddress(endpoint remote.Endpoint) string {
	return "http://" + net.JoinHostPort(endpoint.Host, fmt.Sprint(endpoint.Port))
}

func mobilePairingURL(endpoints []remote.Endpoint, token string) string {
	query := url.Values{}
	query.Set("v", "2")
	query.Set("port", fmt.Sprint(remote.Port))
	query.Set("token", token)
	for _, endpoint := range endpoints {
		query.Add(endpoint.Network, strings.TrimSpace(endpoint.Host))
	}
	return "nakube://connect?" + query.Encode()
}

func mobilePairingBitmap(endpoints []remote.Endpoint, token string) (*ui.Bitmap, error) {
	code, err := qrcode.New(mobilePairingURL(endpoints, token), qrcode.Medium)
	if err != nil {
		return nil, err
	}
	bitmap := ui.NewBitmap(code.Image(352))
	bitmap.Pixelated()
	return bitmap, nil
}

func localLANEndpoints() []remote.Endpoint {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var endpoints []remote.Endpoint
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addrs {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || !ip.IsPrivate() || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			endpoints = append(endpoints, remote.Endpoint{Network: "lan", Host: ip.String(), Port: remote.Port})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Host < endpoints[j].Host })
	return endpoints
}

func mobilePairingToken() (string, error) {
	state, err := cluster.StatePath()
	if err != nil {
		return "", err
	}
	return loadOrCreatePairingToken(filepath.Join(filepath.Dir(state), "mobile-pairing-token"))
}

func loadOrCreatePairingToken(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(body))
		if len(token) < 32 {
			return "", errors.New("stored mobile pairing token is invalid")
		}
		return token, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	token, err := remote.NewPairingToken()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadOrCreatePairingToken(path)
	}
	if err != nil {
		return "", err
	}
	if _, err = file.WriteString(token + "\n"); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return token, nil
}

func tailscaleDNSName(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return "", err
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
	if name == "" {
		return "", errors.New("tailscale status has no MagicDNS name")
	}
	return name, nil
}
