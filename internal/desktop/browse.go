package desktop

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/resource"
)

// listRow is one table row. Actions use Identity, captured when the row was built.
type listRow struct {
	Cells     []string
	Namespace string
	Name      string
	Identity  cluster.Identity
	Forward   *cluster.Forward
}

type listSnap struct {
	columns []string
	rows    []listRow
	loading bool
	err     string
	notice  string
}

func (a *App) drawMain(c *ui.Context, t *ui.Theme, id string, snap listSnap) {
	if a.mode == "logs" {
		a.drawLogs(c, t)
		return
	}
	if a.mode == "exec" {
		a.drawExec(c, t)
		return
	}
	if a.mode == "yaml" {
		a.drawYAML(c, t)
		return
	}
	if id == "overview" || id == "" {
		return
	}
	a.drawList(c, t, id, snap)
	if id == "helm" {
		a.drawHelm(c, t)
	}
}

func (a *App) drawList(c *ui.Context, t *ui.Theme, selected string, snap listSnap) {
	rows := make([]resource.Row, len(snap.rows))
	for i, row := range snap.rows {
		rows[i] = resource.Row{Kind: row.Identity.Kind, Cells: row.Cells}
	}
	resource.Draw(c, t, resource.List{
		Title:    labelFor(selected),
		Err:      snap.err,
		Notice:   snap.notice,
		Empty:    emptyText(selected),
		Loading:  snap.loading,
		Columns:  snap.columns,
		Rows:     rows,
		Selected: &a.row,
		ScaleTo:  &a.scaleTo,
		Local:    &a.forwardLocal,
		Remote:   &a.forwardRemote,
		Revision: &a.rollbackRev,
		OnAction: func(index int, action string) {
			if index < 0 || index >= len(snap.rows) {
				return
			}
			a.runAction(snap.rows[index], action)
		},
	})
}

func emptyText(id string) string {
	switch id {
	case "port-forwards":
		return "None in this process. Start one from a Pod or Service."
	case "helm":
		return "No releases"
	default:
		return "None"
	}
}

func (a *App) runAction(row listRow, action string) {
	switch action {
	case "logs":
		a.openLogs(row.Namespace, row.Name, "")
	case "exec":
		a.openExec(row.Namespace, row.Name, "")
	case "yaml":
		a.openYAML(row.Identity)
	case "forward":
		a.startForward(row)
	case "stop":
		a.stopForward(row.Forward)
	default:
		a.ask(row, action)
	}
}

func (a *App) ask(row listRow, action string) {
	c := Confirm{
		Open:      true,
		Kind:      row.Identity.Kind,
		Namespace: row.Namespace,
		Name:      row.Name,
		Action:    action,
		GVR:       row.Identity.GVR,
	}
	if action == "scale" {
		n, err := strconv.Atoi(strings.TrimSpace(a.scaleTo))
		if err != nil || n < 0 {
			a.setErr("replicas must be a number")
			return
		}
		c.Replicas = int32(n)
	}
	if action == "rollback" {
		n, err := strconv.ParseInt(strings.TrimSpace(a.rollbackRev), 10, 64)
		if err != nil || n < 0 {
			a.setErr("revision must be a number")
			return
		}
		c.Revision = n
	}
	a.mu.Lock()
	a.confirm = &c
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) applyConfirm() {
	a.mu.Lock()
	if a.confirm == nil || !a.confirm.Open {
		a.mu.Unlock()
		return
	}
	c := *a.confirm
	a.confirm.Open = false
	selected := a.selected
	a.mu.Unlock()
	go func() {
		err := a.runConfirmed(c)
		if err != nil {
			a.setErr(publicError(err))
			return
		}
		a.setErr("")
		a.loadList(selected)
	}()
}

func (a *App) runConfirmed(c Confirm) error {
	ctx := context.Background()
	id := cluster.Identity{GVR: c.GVR, Kind: c.Kind, Namespace: c.Namespace, Name: c.Name}
	switch c.Action {
	case "delete":
		if c.Kind == "Application" {
			return a.session.DeleteArgo(ctx, c.Namespace, c.Name, "foreground")
		}
		return a.session.Delete(ctx, id)
	case "restart":
		return a.session.Restart(ctx, id)
	case "scale":
		return a.session.Scale(ctx, id, c.Replicas)
	case "sync":
		return a.session.SyncArgo(ctx, c.Namespace, c.Name, cluster.ArgoSyncOptions{})
	case "rollback":
		if c.Kind == "Release" {
			return a.session.HelmRollback(ctx, c.Namespace, c.Name, int(c.Revision))
		}
		return a.session.RollbackArgo(ctx, c.Namespace, c.Name, c.Revision, false)
	case "uninstall":
		return a.session.HelmUninstall(ctx, c.Namespace, c.Name)
	default:
		return fmt.Errorf("unknown action %s", c.Action)
	}
}

func (a *App) startForward(row listRow) {
	local, remote, err := a.forwardPorts()
	if err != nil {
		a.setErr(err.Error())
		return
	}
	go func() {
		pod := row.Name
		if row.Identity.Kind == "Service" {
			pod, err = a.session.ReadyPod(context.Background(), row.Namespace, row.Name)
			if err != nil {
				a.setErr(publicError(err))
				return
			}
		}
		fw, err := a.session.PortForward(context.Background(), row.Namespace, pod, local, remote)
		if err != nil {
			a.setErr(publicError(err))
			return
		}
		a.mu.Lock()
		a.forwards = append(a.forwards, fw)
		a.mu.Unlock()
		a.setErr("")
		a.invalidate()
	}()
}

func (a *App) forwardPorts() (uint16, uint16, error) {
	local, err := strconv.ParseUint(strings.TrimSpace(a.forwardLocal), 10, 16)
	if err != nil || local == 0 {
		return 0, 0, fmt.Errorf("local port must be a number")
	}
	remote, err := strconv.ParseUint(strings.TrimSpace(a.forwardRemote), 10, 16)
	if err != nil || remote == 0 {
		return 0, 0, fmt.Errorf("remote port must be a number")
	}
	return uint16(local), uint16(remote), nil
}

func (a *App) stopForward(fw *cluster.Forward) {
	if fw != nil {
		fw.Stop()
	}
	a.mu.Lock()
	next := make([]*cluster.Forward, 0, len(a.forwards))
	for _, item := range a.forwards {
		if item != fw {
			next = append(next, item)
		}
	}
	a.forwards = next
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) leaveOverlay() {
	if a.logCancel != nil {
		a.logCancel()
		a.logCancel = nil
	}
	if a.mode == "exec" {
		a.closeExec()
	}
	a.mode = ""
	a.yaml = nil
}

func (a *App) loadList(id string) {
	if isLive(id) {
		return
	}
	a.mu.Lock()
	a.listGen++
	gen := a.listGen
	a.listID = id
	a.listLoading = true
	a.listErr = ""
	a.listNotice = ""
	a.listRows = nil
	a.listCols = nil
	a.mu.Unlock()
	a.invalidate()
	go func() {
		cols, rows, notice, err := a.fetchList(context.Background(), id)
		a.mu.Lock()
		defer a.mu.Unlock()
		if gen != a.listGen || id != a.selected {
			return
		}
		a.listLoading = false
		a.listCols = cols
		a.listRows = rows
		a.listNotice = notice
		if err != nil {
			a.listErr = publicError(err)
		}
		a.invalidate()
	}()
}

func isLive(id string) bool {
	switch id {
	case "", "overview", "workloads", "pods", "nodes", "events", "port-forwards", "settings", "catalog":
		return true
	default:
		return false
	}
}

func (a *App) snapshotLocked(id string) listSnap {
	if isLive(id) {
		cols, rows := a.liveRowsLocked(id)
		return listSnap{columns: cols, rows: rows}
	}
	if a.listID != id {
		return listSnap{loading: true}
	}
	rows := append([]listRow(nil), a.listRows...)
	return listSnap{columns: append([]string(nil), a.listCols...), rows: rows, loading: a.listLoading, err: a.listErr, notice: a.listNotice}
}

func (a *App) liveRowsLocked(id string) ([]string, []listRow) {
	switch id {
	case "workloads":
		return workloadSummary(a.pods)
	case "pods":
		return podTable(a.pods)
	case "nodes":
		return nodeTable(a.nodes)
	case "events":
		return eventTable(a.events)
	case "port-forwards":
		return forwardTable(a.forwards)
	default:
		return nil, nil
	}
}

func (a *App) fetchList(ctx context.Context, id string) ([]string, []listRow, string, error) {
	switch {
	case id == "helm":
		rels, err := a.session.HelmList(ctx)
		if err != nil {
			return nil, nil, "", err
		}
		return helmRows(rels)
	case id == "argo-applications":
		apps, err := a.session.ListArgoApplications(ctx)
		if apierrors.IsNotFound(err) {
			return []string{"Namespace", "Name", "Sync", "Health"}, nil, "CRD is not installed", nil
		}
		if err != nil {
			return nil, nil, "", err
		}
		return argoRows(apps)
	case strings.HasPrefix(id, "argo-"):
		return a.genericRows(ctx, argoKind(id), "CRD is not installed")
	case strings.HasPrefix(id, "istio-"):
		return a.istioRows(ctx, id)
	case workloadKind(id).Kind != "":
		return a.genericRows(ctx, workloadKind(id), "")
	default:
		kind, ok := LookupKind(id)
		if !ok {
			return nil, nil, "", fmt.Errorf("unknown screen %s", id)
		}
		notice := ""
		rows, err := a.session.ListGeneric(ctx, kind.GVR, "")
		if err != nil {
			if apierrors.IsNotFound(err) {
				return []string{"Namespace", "Name", "Age"}, nil, "CRD is not installed", nil
			}
			return nil, nil, "", err
		}
		return genericColumns(), genericListRows(rows, kind.GVR, kindLabel(kind.ID)), notice, nil
	}
}

func (a *App) genericRows(ctx context.Context, kind namedKind, missing string) ([]string, []listRow, string, error) {
	rows, err := a.session.ListGeneric(ctx, kind.GVR, "")
	if err != nil {
		if missing != "" && apierrors.IsNotFound(err) {
			return genericColumns(), nil, missing, nil
		}
		return nil, nil, "", err
	}
	return genericColumns(), genericListRows(rows, kind.GVR, kind.Kind), "", nil
}

func (a *App) istioRows(ctx context.Context, id string) ([]string, []listRow, string, error) {
	var group string
	for _, kind := range cluster.IstioKinds {
		if kind.ID == id {
			group = kind.Group
		}
	}
	if group == "" {
		return nil, nil, "", fmt.Errorf("unknown istio screen %s", id)
	}
	resource := strings.TrimPrefix(id, "istio-")
	var last error
	for _, ver := range []string{"v1", "v1beta1"} {
		gvr := schema.GroupVersionResource{Group: group, Version: ver, Resource: resource}
		rows, err := a.session.ListGeneric(ctx, gvr, "")
		if err == nil {
			return genericColumns(), genericListRows(rows, gvr, istioKindName(id)), "", nil
		}
		last = err
		if !apierrors.IsNotFound(err) {
			return nil, nil, "", err
		}
	}
	if apierrors.IsNotFound(last) {
		return genericColumns(), nil, "CRD is not installed", nil
	}
	return nil, nil, "", last
}

type namedKind struct {
	Kind string
	GVR  schema.GroupVersionResource
}

func workloadKind(id string) namedKind {
	switch id {
	case "deployments":
		return namedKind{"Deployment", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}}
	case "replicasets":
		return namedKind{"ReplicaSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}}
	case "daemonsets":
		return namedKind{"DaemonSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}}
	case "statefulsets":
		return namedKind{"StatefulSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}}
	case "jobs":
		return namedKind{"Job", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}}
	case "cronjobs":
		return namedKind{"CronJob", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}}
	default:
		return namedKind{}
	}
}

func argoKind(id string) namedKind {
	resource := strings.TrimPrefix(id, "argo-")
	kind := "Application"
	switch id {
	case "argo-applicationsets":
		kind = "ApplicationSet"
	case "argo-appprojects":
		kind = "AppProject"
	}
	return namedKind{kind, schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: resource}}
}

func genericColumns() []string {
	return []string{"Namespace", "Name", "Age"}
}

func genericListRows(rows []cluster.GenericRow, gvr schema.GroupVersionResource, kind string) []listRow {
	out := make([]listRow, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		out = append(out, listRow{
			Cells:     []string{row.Namespace, row.Name, ageText(row.Created, now)},
			Namespace: row.Namespace,
			Name:      row.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: kind, Namespace: row.Namespace, Name: row.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func helmRows(rels []cluster.HelmRelease) ([]string, []listRow, string, error) {
	cols := []string{"Namespace", "Name", "Chart", "Revision", "Status"}
	out := make([]listRow, 0, len(rels))
	for _, rel := range rels {
		out = append(out, listRow{
			Cells:     []string{rel.Namespace, rel.Name, rel.Chart, strconv.Itoa(rel.Revision), rel.Status},
			Namespace: rel.Namespace,
			Name:      rel.Name,
			Identity:  cluster.Identity{Kind: "Release", Namespace: rel.Namespace, Name: rel.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return cols, out, "", nil
}

func argoRows(apps []cluster.ArgoApplication) ([]string, []listRow, string, error) {
	cols := []string{"Namespace", "Name", "Sync", "Health"}
	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	out := make([]listRow, 0, len(apps))
	for _, app := range apps {
		out = append(out, listRow{
			Cells:     []string{app.Namespace, app.Name, app.Sync, app.Health},
			Namespace: app.Namespace,
			Name:      app.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: "Application", Namespace: app.Namespace, Name: app.Name},
		})
	}
	return cols, out, "", nil
}

func workloadSummary(pods map[string]*corev1.Pod) ([]string, []listRow) {
	var running, pending, failed, other int
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			pending++
		case corev1.PodFailed:
			failed++
		default:
			other++
		}
	}
	return []string{"Phase", "Pods"}, []listRow{
		{Cells: []string{"Running", strconv.Itoa(running)}},
		{Cells: []string{"Pending", strconv.Itoa(pending)}},
		{Cells: []string{"Failed", strconv.Itoa(failed)}},
		{Cells: []string{"Other", strconv.Itoa(other)}},
	}
}

func podTable(pods map[string]*corev1.Pod) ([]string, []listRow) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	out := make([]listRow, 0, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		out = append(out, listRow{
			Cells:     []string{pod.Namespace, pod.Name, string(pod.Status.Phase), readyCount(pod)},
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return []string{"Namespace", "Name", "Phase", "Ready"}, out
}

func nodeTable(nodes map[string]*corev1.Node) ([]string, []listRow) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	out := make([]listRow, 0, len(nodes))
	now := time.Now()
	for _, node := range nodes {
		if node == nil {
			continue
		}
		out = append(out, listRow{
			Cells:    []string{node.Name, nodeReady(node), ageText(node.CreationTimestamp.Time, now)},
			Name:     node.Name,
			Identity: cluster.Identity{GVR: gvr, Kind: "Node", Name: node.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return []string{"Name", "Status", "Age"}, out
}

func eventTable(events map[string]*corev1.Event) ([]string, []listRow) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "events"}
	out := make([]listRow, 0, len(events))
	for _, ev := range events {
		if ev == nil {
			continue
		}
		out = append(out, listRow{
			Cells:     []string{ev.Namespace, ev.Name, ev.Reason, trimText(ev.Message, 80)},
			Namespace: ev.Namespace,
			Name:      ev.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: "Event", Namespace: ev.Namespace, Name: ev.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return []string{"Namespace", "Name", "Reason", "Message"}, out
}

func forwardTable(items []*cluster.Forward) ([]string, []listRow) {
	out := make([]listRow, 0, len(items))
	for _, fw := range items {
		if fw == nil {
			continue
		}
		out = append(out, listRow{
			Cells:     []string{fw.Namespace, fw.Pod, strconv.Itoa(int(fw.Local)), strconv.Itoa(int(fw.Remote))},
			Namespace: fw.Namespace,
			Name:      fw.Pod,
			Identity:  cluster.Identity{Kind: "Forward", Namespace: fw.Namespace, Name: fw.Pod},
			Forward:   fw,
		})
	}
	return []string{"Namespace", "Pod", "Local", "Remote"}, out
}

func readyCount(pod *corev1.Pod) string {
	var ready, total int
	for _, cs := range pod.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func nodeReady(node *corev1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func ageText(created, now time.Time) string {
	if created.IsZero() {
		return ""
	}
	d := now.Sub(created)
	if d < 0 {
		d = 0
	}
	return d.Truncate(time.Second).String()
}

func trimText(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func kindLabel(id string) string {
	switch id {
	case "nodes":
		return "Node"
	case "namespaces":
		return "Namespace"
	case "events":
		return "Event"
	case "leases":
		return "Lease"
	case "services":
		return "Service"
	case "ingresses":
		return "Ingress"
	case "endpointslices":
		return "EndpointSlice"
	case "networkpolicies":
		return "NetworkPolicy"
	case "ingressclasses":
		return "IngressClass"
	case "configmaps":
		return "ConfigMap"
	case "secrets":
		return "Secret"
	case "horizontalpodautoscalers":
		return "HorizontalPodAutoscaler"
	case "limitranges":
		return "LimitRange"
	case "resourcequotas":
		return "ResourceQuota"
	case "poddisruptionbudgets":
		return "PodDisruptionBudget"
	case "persistentvolumes":
		return "PersistentVolume"
	case "persistentvolumeclaims":
		return "PersistentVolumeClaim"
	case "volumeattachments":
		return "VolumeAttachment"
	case "storageclasses":
		return "StorageClass"
	case "csidrivers":
		return "CSIDriver"
	case "csinodes":
		return "CSINode"
	case "serviceaccounts":
		return "ServiceAccount"
	case "roles":
		return "Role"
	case "rolebindings":
		return "RoleBinding"
	case "clusterroles":
		return "ClusterRole"
	case "clusterrolebindings":
		return "ClusterRoleBinding"
	case "crds":
		return "CustomResourceDefinition"
	case "priorityclasses":
		return "PriorityClass"
	case "runtimeclasses":
		return "RuntimeClass"
	default:
		return id
	}
}

func istioKindName(id string) string {
	for _, kind := range cluster.IstioKinds {
		if kind.ID == id {
			return kind.Kind
		}
	}
	return id
}
