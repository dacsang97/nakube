package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
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

// Phone adapts the desktop session for the Tailscale listener.
type Phone struct {
	App *App
}

func (p Phone) Overview(context.Context) (remote.Overview, error) {
	a := p.App
	a.mu.Lock()
	shown := overview.From(a.nodes, a.pods, a.events, a.usage, a.usageErr, time.Now())
	current := a.session.Current()
	a.mu.Unlock()
	return remote.Overview{
		Context:  current,
		Nodes:    shown.NodesText(),
		Pods:     shown.PodsText(),
		Warnings: shown.WarningsText(),
		CPU:      shown.CPU,
		Memory:   shown.Memory,
	}, nil
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

// StartPhoneListener binds the Tailscale address and publishes a scannable
// endpoint to Settings. A missing CLI or a down tailnet leaves the desktop
// running and makes the setup problem visible there.
func (a *App) StartPhoneListener(ctx context.Context) {
	ip, err := tailscaleIPv4(ctx)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("Tailscale is unavailable. Sign in to Tailscale and restart NaKube"))
		return
	}
	dnsName, err := tailscaleDNSName(ctx)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("Tailscale MagicDNS is unavailable. Enable MagicDNS and restart NaKube"))
		return
	}
	qr, err := mobilePairingBitmap(dnsName)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("Na Kube could not create the mobile pairing code"))
		return
	}
	_, err = remote.ListenTailscale(ctx, func(context.Context) (string, error) {
		return ip, nil
	}, remote.Handler(Phone{App: a}))
	if err != nil {
		a.setMobileConnection("", nil, errors.New("Na Kube could not listen on the Tailscale address"))
		return
	}
	a.setMobileConnection(fmt.Sprintf("http://%s:%d", dnsName, remote.Port), qr, nil)
}

func (a *App) setMobileConnection(address string, qr *ui.Bitmap, err error) {
	a.mu.Lock()
	a.mobileAddress = address
	a.mobileQR = qr
	a.mobileErr = ""
	if err != nil {
		a.mobileErr = err.Error()
	}
	a.mu.Unlock()
	a.invalidate()
}

func mobilePairingURL(host string) string {
	query := url.Values{}
	query.Set("host", strings.TrimSpace(host))
	query.Set("port", fmt.Sprint(remote.Port))
	return "nakube://connect?" + query.Encode()
}

func mobilePairingBitmap(host string) (*ui.Bitmap, error) {
	code, err := qrcode.New(mobilePairingURL(host), qrcode.Medium)
	if err != nil {
		return nil, err
	}
	bitmap := ui.NewBitmap(code.Image(352))
	bitmap.Pixelated()
	return bitmap, nil
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

func tailscaleIPv4(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
