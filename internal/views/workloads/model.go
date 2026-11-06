package workloads

import (
	"fmt"
	"slices"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	ColKind      = "kind"
	ColNamespace = "namespace"
	ColName      = "name"
	ColStatus    = "status"
	ColReady     = "ready"
	ColAge       = "age"
)

type Status int

const (
	StatusHealthy Status = iota
	StatusActive
	StatusSucceeded
	StatusScheduled
	StatusSuspended
	StatusFailed
	StatusUnknown
	statusCount
)

func (s Status) Label() string {
	switch s {
	case StatusHealthy:
		return "Healthy"
	case StatusActive:
		return "Active"
	case StatusSucceeded:
		return "Succeeded"
	case StatusScheduled:
		return "Scheduled"
	case StatusSuspended:
		return "Suspended"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func (s Status) Healthy() bool {
	return s == StatusHealthy || s == StatusSucceeded || s == StatusScheduled
}

func (s Status) Attention() bool {
	return s == StatusFailed || s == StatusUnknown
}

type Row struct {
	Key       string
	KindID    string
	Kind      string
	Namespace string
	Name      string
	Status    Status
	Ready     int32
	Desired   int32
	Created   time.Time
}

func (r Row) ReadyText() string {
	if r.Desired <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d/%d", r.Ready, r.Desired)
}

type KindSummary struct {
	ID      string
	Title   string
	Total   int
	Ready   int32
	Desired int32
	Counts  [statusCount]int
}

func (k KindSummary) Count(status Status) int {
	if status < 0 || status >= statusCount {
		return 0
	}
	return k.Counts[status]
}

type Page struct {
	Kinds     []KindSummary
	Rows      []Row
	Total     int
	Healthy   int
	Active    int
	Attention int
	Suspended int
	Ready     int32
	Desired   int32
}

func Build(objects []runtime.Object, query string, filter int, sortBy string, descending bool) Page {
	page := Page{Kinds: kindSummaries()}
	for _, object := range objects {
		row, ok := normalize(object)
		if !ok {
			continue
		}
		page.Total++
		page.Ready += row.Ready
		page.Desired += row.Desired
		switch {
		case row.Status.Healthy():
			page.Healthy++
		case row.Status == StatusActive:
			page.Active++
		case row.Status.Attention():
			page.Attention++
		case row.Status == StatusSuspended:
			page.Suspended++
		}
		for i := range page.Kinds {
			if page.Kinds[i].ID != row.KindID {
				continue
			}
			page.Kinds[i].Total++
			page.Kinds[i].Ready += row.Ready
			page.Kinds[i].Desired += row.Desired
			page.Kinds[i].Counts[row.Status]++
			break
		}
		if matches(row, query, filter) {
			page.Rows = append(page.Rows, row)
		}
	}
	if sortBy == "" {
		sortBy = ColKind
	}
	slices.SortStableFunc(page.Rows, func(a, b Row) int {
		order := compareRows(a, b, sortBy, descending)
		if order != 0 {
			return order
		}
		if order = kit.CompareText(a.Namespace, b.Namespace, false); order != 0 {
			return order
		}
		return kit.CompareText(a.Name, b.Name, false)
	})
	return page
}

func kindSummaries() []KindSummary {
	return []KindSummary{
		{ID: "pods", Title: "Pods"},
		{ID: "deployments", Title: "Deployments"},
		{ID: "replicasets", Title: "ReplicaSets"},
		{ID: "daemonsets", Title: "DaemonSets"},
		{ID: "statefulsets", Title: "StatefulSets"},
		{ID: "jobs", Title: "Jobs"},
		{ID: "cronjobs", Title: "CronJobs"},
	}
}

func matches(row Row, query string, filter int) bool {
	q := strings.TrimSpace(query)
	if q != "" && !kit.Match(q, row.Kind) && !kit.Match(q, row.Namespace) && !kit.Match(q, row.Name) && !kit.Match(q, row.Status.Label()) {
		return false
	}
	switch filter {
	case 1:
		return row.Status.Healthy()
	case 2:
		return row.Status == StatusActive
	case 3:
		return row.Status.Attention()
	case 4:
		return row.Status == StatusSuspended
	default:
		return true
	}
}

func compareRows(a, b Row, column string, descending bool) int {
	switch column {
	case ColNamespace:
		return kit.CompareText(a.Namespace, b.Namespace, descending)
	case ColName:
		return kit.CompareText(a.Name, b.Name, descending)
	case ColStatus:
		return kit.CompareText(a.Status.Label(), b.Status.Label(), descending)
	case ColReady:
		return kit.CompareFloat(float64(a.Ready), float64(b.Ready), descending)
	case ColAge:
		return kit.CompareTime(a.Created, b.Created, !descending)
	default:
		return kit.CompareText(a.Kind, b.Kind, descending)
	}
}

func normalize(object runtime.Object) (Row, bool) {
	switch item := object.(type) {
	case *corev1.Pod:
		ready := int32(0)
		status := StatusUnknown
		switch item.Status.Phase {
		case corev1.PodRunning:
			status = StatusActive
			if podReady(item) {
				ready, status = 1, StatusHealthy
			}
		case corev1.PodPending:
			status = StatusActive
		case corev1.PodSucceeded:
			ready, status = 1, StatusSucceeded
		case corev1.PodFailed:
			status = StatusFailed
		}
		return newRow("pods", "Pod", item.Namespace, item.Name, item.CreationTimestamp.Time, status, ready, 1), true
	case *appsv1.Deployment:
		desired := replicas(item.Spec.Replicas)
		status := replicaStatus(item.Status.ReadyReplicas, desired)
		for _, condition := range item.Status.Conditions {
			failedReplica := condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue
			timedOut := condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded"
			if failedReplica || timedOut {
				status = StatusFailed
			}
		}
		return newRow("deployments", "Deployment", item.Namespace, item.Name, item.CreationTimestamp.Time, status, item.Status.ReadyReplicas, desired), true
	case *appsv1.ReplicaSet:
		desired := replicas(item.Spec.Replicas)
		return newRow("replicasets", "ReplicaSet", item.Namespace, item.Name, item.CreationTimestamp.Time, replicaStatus(item.Status.ReadyReplicas, desired), item.Status.ReadyReplicas, desired), true
	case *appsv1.DaemonSet:
		status := replicaStatus(item.Status.NumberReady, item.Status.DesiredNumberScheduled)
		if item.Status.NumberMisscheduled > 0 {
			status = StatusFailed
		}
		return newRow("daemonsets", "DaemonSet", item.Namespace, item.Name, item.CreationTimestamp.Time, status, item.Status.NumberReady, item.Status.DesiredNumberScheduled), true
	case *appsv1.StatefulSet:
		desired := replicas(item.Spec.Replicas)
		return newRow("statefulsets", "StatefulSet", item.Namespace, item.Name, item.CreationTimestamp.Time, replicaStatus(item.Status.ReadyReplicas, desired), item.Status.ReadyReplicas, desired), true
	case *batchv1.Job:
		desired := int32(1)
		if item.Spec.Completions != nil && *item.Spec.Completions > 0 {
			desired = *item.Spec.Completions
		}
		status := StatusActive
		if item.Spec.Suspend != nil && *item.Spec.Suspend {
			status = StatusSuspended
		}
		for _, condition := range item.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			switch condition.Type {
			case batchv1.JobFailed, batchv1.JobFailureTarget:
				status = StatusFailed
			case batchv1.JobComplete, batchv1.JobSuccessCriteriaMet:
				status = StatusSucceeded
			case batchv1.JobSuspended:
				status = StatusSuspended
			}
		}
		return newRow("jobs", "Job", item.Namespace, item.Name, item.CreationTimestamp.Time, status, item.Status.Succeeded, desired), true
	case *batchv1.CronJob:
		status := StatusScheduled
		if item.Spec.Suspend != nil && *item.Spec.Suspend {
			status = StatusSuspended
		} else if len(item.Status.Active) > 0 {
			status = StatusActive
		}
		return newRow("cronjobs", "CronJob", item.Namespace, item.Name, item.CreationTimestamp.Time, status, int32(len(item.Status.Active)), 0), true
	default:
		return Row{}, false
	}
}

func newRow(kindID, kind, namespace, name string, created time.Time, status Status, ready, desired int32) Row {
	return Row{
		Key:       kindID + "/" + namespace + "/" + name,
		KindID:    kindID,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Status:    status,
		Ready:     ready,
		Desired:   desired,
		Created:   created,
	}
}

func replicas(value *int32) int32 {
	if value == nil {
		return 1
	}
	return max(*value, 0)
}

func replicaStatus(ready, desired int32) Status {
	if desired == 0 || ready >= desired {
		return StatusHealthy
	}
	return StatusActive
}

func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	if len(pod.Status.ContainerStatuses) == 0 {
		return pod.Status.Phase == corev1.PodRunning
	}
	return !slices.ContainsFunc(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool { return !status.Ready })
}

func CountLabel(filtered, total int) string {
	if filtered == total {
		return fmt.Sprintf("%d resources", total)
	}
	return fmt.Sprintf("%d of %d resources", filtered, total)
}

func StatusOrder() []Status {
	return []Status{StatusFailed, StatusActive, StatusSuspended, StatusHealthy, StatusScheduled, StatusSucceeded, StatusUnknown}
}
