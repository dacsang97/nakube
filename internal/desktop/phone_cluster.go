package desktop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
)

func (p Phone) ContextNodes(ctx context.Context, name string) ([]remote.Node, error) {
	session, cs, err := p.contextClientset(name)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	usage, usageErr := session.Usage(ctx)
	if usageErr != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		usage = cluster.Usage{}
	}
	samples := make(map[string]cluster.NodeUsage, len(usage.Nodes))
	if usage.Available {
		for _, sample := range usage.Nodes {
			samples[sample.Name] = sample
		}
	}
	out := make([]remote.Node, 0, len(list.Items))
	for i := range list.Items {
		node := &list.Items[i]
		cpu := remote.ResourceReading{Allocatable: node.Status.Allocatable.Cpu().MilliValue()}
		memory := remote.ResourceReading{Allocatable: node.Status.Allocatable.Memory().Value()}
		if sample, ok := samples[node.Name]; ok {
			cpu.Used = new(sample.CPUMilli)
			memory.Used = new(sample.MemoryBytes)
		}
		out = append(out, remote.Node{
			Name: node.Name, Ready: cluster.NodeReady(node), Roles: nodeRoles(node.Labels),
			Version: node.Status.NodeInfo.KubeletVersion, Conditions: nodeConditions(node),
			Taints: nodeTaints(node.Spec.Taints), CPU: cpu, Memory: memory,
			CreatedAt: node.CreationTimestamp.Time,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (p Phone) ContextNamespaces(ctx context.Context, name string) ([]remote.Namespace, error) {
	_, cs, err := p.contextClientset(name)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]remote.Namespace, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		phase := string(item.Status.Phase)
		if phase == "" {
			phase = string(corev1.NamespaceActive)
		}
		finalizers := make([]string, 0, len(item.Spec.Finalizers))
		for _, finalizer := range item.Spec.Finalizers {
			finalizers = append(finalizers, string(finalizer))
		}
		labels := make(map[string]string, len(item.Labels))
		for key, value := range item.Labels {
			labels[key] = value
		}
		out = append(out, remote.Namespace{
			Name: item.Name, Phase: phase, Labels: labels, Finalizers: finalizers,
			CreatedAt: item.CreationTimestamp.Time,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (p Phone) ContextEvents(ctx context.Context, name string) ([]remote.Event, error) {
	_, cs, err := p.contextClientset(name)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]remote.Event, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		out = append(out, remote.Event{
			Namespace: item.Namespace, Name: item.Name, Type: item.Type, Reason: item.Reason,
			Object: eventObject(item.InvolvedObject), Message: strings.Join(strings.Fields(item.Message), " "),
			Source: eventSource(item), Count: item.Count, FirstSeen: eventFirstSeen(item),
			LastSeen: eventLastSeen(item),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out, nil
}

func (p Phone) ContextLeases(ctx context.Context, name string) ([]remote.Lease, error) {
	_, cs, err := p.contextClientset(name)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoordinationV1().Leases("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]remote.Lease, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		var renewTime *time.Time
		if item.Spec.RenewTime != nil {
			value := item.Spec.RenewTime.Time
			renewTime = &value
		}
		out = append(out, remote.Lease{
			Namespace: item.Namespace, Name: item.Name, Holder: stringPointerValue(item.Spec.HolderIdentity),
			RenewTime: renewTime, DurationSeconds: item.Spec.LeaseDurationSeconds,
			Transitions: item.Spec.LeaseTransitions, CreatedAt: item.CreationTimestamp.Time,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name
	})
	return out, nil
}

func (p Phone) PortForwards(ctx context.Context) ([]remote.PortForward, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.App.mu.Lock()
	defer p.App.mu.Unlock()
	out := make([]remote.PortForward, 0, len(p.App.forwards))
	for _, forward := range p.App.forwards {
		if forward != nil {
			out = append(out, remote.PortForward{
				Namespace: forward.Namespace, Pod: forward.Pod, Local: forward.Local, Remote: forward.Remote,
			})
		}
	}
	return out, nil
}

func (p Phone) contextClientset(name string) (*cluster.Session, kubernetes.Interface, error) {
	if name == "" {
		return nil, nil, fmt.Errorf("context is required")
	}
	session, err := p.App.session.ForContext(name)
	if err != nil {
		return nil, nil, err
	}
	cs, err := session.Clientset()
	if err != nil {
		return nil, nil, err
	}
	return session, cs, nil
}

func nodeRoles(labels map[string]string) []string {
	const prefix = "node-role.kubernetes.io/"
	roles := make([]string, 0)
	for key, value := range labels {
		if strings.HasPrefix(key, prefix) {
			role := strings.TrimPrefix(key, prefix)
			if role == "" {
				role = value
			}
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	sort.Strings(roles)
	return roles
}

func nodeConditions(node *corev1.Node) []string {
	out := make([]string, 0, len(node.Status.Conditions))
	for _, condition := range node.Status.Conditions {
		out = append(out, string(condition.Type)+": "+string(condition.Status))
	}
	sort.Strings(out)
	return out
}

func nodeTaints(taints []corev1.Taint) []string {
	out := make([]string, 0, len(taints))
	for _, taint := range taints {
		value := taint.Key
		if taint.Value != "" {
			value += "=" + taint.Value
		}
		if taint.Effect != "" {
			value += ":" + string(taint.Effect)
		}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func eventObject(ref corev1.ObjectReference) string {
	if ref.Kind != "" && ref.Name != "" {
		return ref.Kind + "/" + ref.Name
	}
	if ref.Name != "" {
		return ref.Name
	}
	return ref.Kind
}

func eventSource(item *corev1.Event) string {
	component := item.Source.Component
	if component == "" {
		component = item.ReportingController
	}
	if component == "" || item.Source.Host == "" {
		return component
	}
	return component + " " + item.Source.Host
}

func eventFirstSeen(item *corev1.Event) time.Time {
	if !item.FirstTimestamp.IsZero() {
		return item.FirstTimestamp.Time
	}
	return item.EventTime.Time
}

func eventLastSeen(item *corev1.Event) time.Time {
	if !item.LastTimestamp.IsZero() {
		return item.LastTimestamp.Time
	}
	if item.Series != nil && !item.Series.LastObservedTime.IsZero() {
		return item.Series.LastObservedTime.Time
	}
	if !item.EventTime.IsZero() {
		return item.EventTime.Time
	}
	return item.FirstTimestamp.Time
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
