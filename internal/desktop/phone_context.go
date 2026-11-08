package desktop

import (
	"context"
	"time"

	"github.com/dacsang97/nakube/internal/remote"
	"github.com/dacsang97/nakube/internal/views/overview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p Phone) Contexts(ctx context.Context) ([]remote.Context, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	infos, _, err := p.App.session.ContextInfos()
	if err != nil {
		return nil, err
	}
	current := p.App.session.Current()
	rows := make([]remote.Context, 0, len(infos))
	for _, info := range infos {
		rows = append(rows, remote.Context{Name: info.Name, Cluster: info.Cluster, Current: info.Name == current})
	}
	return rows, nil
}

// ContextOverview reads the requested cluster without changing desktop selection.
func (p Phone) ContextOverview(ctx context.Context, name string) (remote.Overview, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	session, err := p.App.session.ForContext(name)
	if err != nil {
		return remote.Overview{}, err
	}
	cs, err := session.Clientset()
	if err != nil {
		return remote.Overview{}, err
	}
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return remote.Overview{}, err
	}
	pods, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return remote.Overview{}, err
	}
	events, err := cs.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return remote.Overview{}, err
	}
	usage, usageErr := session.Usage(ctx)
	nodeMap := make(map[string]*corev1.Node, len(nodes.Items))
	for i := range nodes.Items {
		nodeMap[nodes.Items[i].Name] = &nodes.Items[i]
	}
	podMap := make(map[string]*corev1.Pod, len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		podMap[pod.Namespace+"/"+pod.Name] = pod
	}
	eventMap := make(map[string]*corev1.Event, len(events.Items))
	for i := range events.Items {
		event := &events.Items[i]
		eventMap[event.Namespace+"/"+event.Name] = event
	}
	shown := overview.From(nodeMap, podMap, eventMap, usage, usageErr, time.Now())
	return phoneOverview(name, shown, usage), nil
}
