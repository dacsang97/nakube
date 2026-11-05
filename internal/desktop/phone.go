package desktop

import (
	"context"
	"io"
	"os/exec"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
)

// Phone adapts the desktop session for the Tailscale listener.
type Phone struct {
	App *App
}

func (p Phone) Overview(context.Context) (remote.Overview, error) {
	a := p.App
	a.mu.Lock()
	shown := overviewFrom(a.nodes, a.pods, a.events, a.usage, a.usageErr)
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

// StartPhoneListener binds the Tailscale address when the CLI is up.
// A missing CLI or a down tailnet leaves the desktop running.
func (a *App) StartPhoneListener(ctx context.Context) {
	_, _ = remote.ListenTailscale(ctx, tailscaleIPv4, remote.Handler(Phone{App: a}))
}

func tailscaleIPv4(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
