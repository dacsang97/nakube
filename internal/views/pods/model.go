// Package pods is the Pods screen: a sortable, searchable list with the
// columns Lens shows.
package pods

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	ColName       = "name"
	ColNamespace  = "namespace"
	ColContainers = "containers"
	ColCPU        = "cpu"
	ColMemory     = "memory"
	ColRestarts   = "restarts"
	ColOwner      = "owner"
	ColNode       = "node"
	ColQoS        = "qos"
	ColAge        = "age"
	ColStatus     = "status"
)

// DotState is a container's square in the Containers column.
type DotState int

const (
	DotWaiting DotState = iota
	DotRunning          // running, not ready
	DotReady
	DotDone // exited 0
	DotFailed
)

// Dot is one container. Text is its tooltip.
type Dot struct {
	Name  string
	State DotState
	Init  bool
	Text  string
}

// Owner is the pod's controller. Screen and Key are where its link goes;
// an empty Screen is text with no link.
type Owner struct {
	Kind   string
	Name   string
	Screen string
	Key    string
}

// Row is one pod. CPU is cores and Mem bytes; Known false is N/A, never 0.
type Row struct {
	Key        string
	Name       string
	Namespace  string
	Dots       []Dot
	CPU        float64
	Mem        float64
	Known      bool
	Restarts   int32
	Owner      Owner
	Node       string
	QoS        string
	Created    time.Time
	Status     string
	StatusTone kit.Tone
	Identity   cluster.Identity

	search string
}

// Page is the visible pod list. Total counts pods before search.
type Page struct {
	Rows  []Row
	Total int
}

// Build lists pods. usage is keyed "namespace/name"; a pod absent from it
// has no reading. An empty column sorts by name.
func Build(items []*corev1.Pod, usage map[string]cluster.PodUsage, query, column string, desc bool) Page {
	rows := make([]Row, 0, len(items))
	total := 0
	for _, pod := range items {
		if pod == nil {
			continue
		}
		total++
		row := rowFrom(pod, usage)
		if !kit.Match(query, row.search) {
			continue
		}
		rows = append(rows, row)
	}
	sortRows(rows, column, desc)
	return Page{Rows: rows, Total: total}
}

func rowFrom(pod *corev1.Pod, usage map[string]cluster.PodUsage) Row {
	key := pod.Namespace + "/" + pod.Name
	status, tone := Status(pod)
	row := Row{
		Key:        key,
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Dots:       dots(pod),
		Restarts:   restarts(pod),
		Owner:      owner(pod),
		Node:       pod.Spec.NodeName,
		QoS:        string(pod.Status.QOSClass),
		Created:    pod.CreationTimestamp.Time,
		Status:     status,
		StatusTone: tone,
		Identity: cluster.Identity{
			GVR:       schema.GroupVersionResource{Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
	}
	if u, ok := usage[key]; ok {
		row.Known = true
		row.CPU = float64(u.CPUMilli) / 1000
		row.Mem = float64(u.MemoryBytes)
	}
	row.search = strings.Join([]string{row.Name, row.Namespace, row.Node, row.Owner.Kind, row.Owner.Name, row.Status, row.QoS}, " ")
	return row
}

// Status reads as kubectl's STATUS column: Terminating, an init step, a
// container's waiting or exit reason, the pod's reason, else its phase.
func Status(pod *corev1.Pod) (string, kit.Tone) {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	initializing := false
	for i, c := range pod.Status.InitContainerStatuses {
		switch {
		case c.State.Terminated != nil && c.State.Terminated.ExitCode == 0:
			continue
		case sidecar(pod, c.Name) && c.Started != nil && *c.Started:
			// A started sidecar keeps running; it does not hold the pod in Init.
			continue
		case c.State.Terminated != nil:
			reason = "Init:" + exitReason(c.State.Terminated)
		case c.State.Waiting != nil && c.State.Waiting.Reason != "" && c.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + c.State.Waiting.Reason
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
		}
		initializing = true
		break
	}
	if !initializing {
		running := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			c := pod.Status.ContainerStatuses[i]
			switch {
			case c.State.Waiting != nil && c.State.Waiting.Reason != "":
				reason = c.State.Waiting.Reason
			case c.State.Terminated != nil:
				reason = exitReason(c.State.Terminated)
			case c.Ready && c.State.Running != nil:
				running = true
			}
		}
		if reason == "Completed" && running {
			reason = "Running"
			if !podReady(pod) {
				reason = "NotReady"
			}
		}
	}
	if pod.DeletionTimestamp != nil {
		if pod.Status.Reason == "NodeLost" {
			reason = "Unknown"
		} else {
			reason = "Terminating"
		}
	}
	return reason, statusTone(reason)
}

func exitReason(t *corev1.ContainerStateTerminated) string {
	switch {
	case t.Reason != "":
		return t.Reason
	case t.Signal != 0:
		return "Signal:" + strconv.Itoa(int(t.Signal))
	default:
		return "ExitCode:" + strconv.Itoa(int(t.ExitCode))
	}
}

func podReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func statusTone(reason string) kit.Tone {
	switch {
	case reason == "Running":
		return kit.ToneOK
	case reason == "Completed" || reason == "Succeeded":
		return kit.ToneMuted
	case reason == "Pending" || reason == "ContainerCreating" || reason == "PodInitializing" ||
		reason == "Terminating" || reason == "NotReady" || strings.HasPrefix(reason, "Init:") && !failing(reason):
		return kit.ToneWarn
	default:
		return kit.ToneBad
	}
}

// failing is a waiting reason that will not clear on its own.
func failing(reason string) bool {
	return strings.Contains(reason, "BackOff") || strings.Contains(reason, "Err") ||
		strings.Contains(reason, "Error") || strings.Contains(reason, "Invalid")
}

// dots lists init containers, then app containers, in spec order. A
// container with no status yet is waiting.
func dots(pod *corev1.Pod) []Dot {
	out := make([]Dot, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	add := func(specs []corev1.Container, statuses []corev1.ContainerStatus, init bool) {
		byName := make(map[string]corev1.ContainerStatus, len(statuses))
		for _, s := range statuses {
			byName[s.Name] = s
		}
		for _, spec := range specs {
			s, ok := byName[spec.Name]
			state, text := DotWaiting, "waiting"
			if ok {
				state, text = dotState(s)
			}
			label := spec.Name
			if init {
				label += " (init)"
			}
			out = append(out, Dot{Name: spec.Name, State: state, Init: init, Text: label + ": " + text})
		}
	}
	add(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, true)
	add(pod.Spec.Containers, pod.Status.ContainerStatuses, false)
	return out
}

func dotState(s corev1.ContainerStatus) (DotState, string) {
	switch {
	case s.State.Running != nil && s.Ready:
		return DotReady, "running, ready"
	case s.State.Running != nil:
		return DotRunning, "running, not ready"
	case s.State.Terminated != nil && s.State.Terminated.ExitCode == 0:
		return DotDone, "terminated, " + exitReason(s.State.Terminated)
	case s.State.Terminated != nil:
		return DotFailed, "terminated, " + exitReason(s.State.Terminated)
	case s.State.Waiting != nil && failing(s.State.Waiting.Reason):
		return DotFailed, "waiting, " + s.State.Waiting.Reason
	case s.State.Waiting != nil && s.State.Waiting.Reason != "":
		return DotWaiting, "waiting, " + s.State.Waiting.Reason
	default:
		return DotWaiting, "waiting"
	}
}

// sidecar is an init container that runs beside the app, restartPolicy
// Always. It is not an init step.
func sidecar(pod *corev1.Pod, name string) bool {
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
		}
	}
	return false
}

// restarts counts app containers and sidecars, as kubectl's RESTARTS.
func restarts(pod *corev1.Pod) int32 {
	var n int32
	for _, s := range pod.Status.InitContainerStatuses {
		if sidecar(pod, s.Name) {
			n += s.RestartCount
		}
	}
	for _, s := range pod.Status.ContainerStatuses {
		n += s.RestartCount
	}
	return n
}

// workloadIDs are the Workloads screen's kind ids, the first part of its keys.
var workloadIDs = map[string]string{
	"ReplicaSet":  "replicasets",
	"Deployment":  "deployments",
	"StatefulSet": "statefulsets",
	"DaemonSet":   "daemonsets",
	"Job":         "jobs",
	"CronJob":     "cronjobs",
}

// owner is the controller reference, else the first owner. A kind with no
// screen of its own, as a custom resource, has no link.
func owner(pod *corev1.Pod) Owner {
	refs := pod.OwnerReferences
	if len(refs) == 0 {
		return Owner{}
	}
	ref := refs[0]
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			ref = r
			break
		}
	}
	o := Owner{Kind: ref.Kind, Name: ref.Name}
	if id, ok := workloadIDs[ref.Kind]; ok && builtIn(ref.APIVersion) {
		o.Screen, o.Key = "workloads", id+"/"+pod.Namespace+"/"+ref.Name
	}
	if ref.Kind == "Node" && builtIn(ref.APIVersion) {
		o.Screen, o.Key = "nodes", ref.Name
	}
	return o
}

// builtIn is true for the core, apps and batch groups, so a custom
// resource that reuses a kind name gets no link.
func builtIn(apiVersion string) bool {
	group, _, found := strings.Cut(apiVersion, "/")
	if !found {
		group = ""
	}
	return group == "" || group == "apps" || group == "batch"
}

func sortRows(rows []Row, column string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		c := 0
		switch column {
		case ColNamespace:
			c = kit.CompareText(a.Namespace, b.Namespace, desc)
		case ColCPU:
			c = kit.CompareKnown(a.Known, b.Known, a.CPU, b.CPU, desc)
		case ColMemory:
			c = kit.CompareKnown(a.Known, b.Known, a.Mem, b.Mem, desc)
		case ColRestarts:
			c = kit.CompareFloat(float64(a.Restarts), float64(b.Restarts), desc)
		case ColOwner:
			c = kit.CompareText(a.Owner.Kind+" "+a.Owner.Name, b.Owner.Kind+" "+b.Owner.Name, desc)
		case ColNode:
			c = kit.CompareText(a.Node, b.Node, desc)
		case ColQoS:
			c = kit.CompareText(a.QoS, b.QoS, desc)
		case ColAge:
			// The youngest pod is the newest, so age runs against time.
			c = kit.CompareTime(a.Created, b.Created, !desc)
		case ColStatus:
			c = kit.CompareText(a.Status, b.Status, desc)
		default:
			c = kit.CompareText(a.Name, b.Name, desc && column == ColName)
		}
		if c != 0 {
			return c < 0
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
}

// CPU is cores with three decimals, as Lens writes them.
func CPU(cores float64) string {
	return strconv.FormatFloat(cores, 'f', 3, 64)
}

// Memory is a binary size with one decimal: 81.1MiB, 1.5GiB.
func Memory(bytes float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for bytes >= 1024 && i < len(units)-1 {
		bytes /= 1024
		i++
	}
	text := strconv.FormatFloat(bytes, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	return text + units[i]
}
