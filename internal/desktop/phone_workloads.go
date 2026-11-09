package desktop

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/dacsang97/nakube/internal/remote"
)

var phoneWorkloadResources = [...]string{
	"pods", "deployments", "replicasets", "daemonsets", "statefulsets", "jobs", "cronjobs",
}

func (p Phone) ContextWorkloads(ctx context.Context, name, resource string) ([]remote.Workload, error) {
	resources := phoneWorkloadResources[:]
	if resource != "" {
		if !validPhoneWorkloadResource(resource) {
			return nil, &remote.StatusError{Status: 404, Msg: "workload resource not found"}
		}
		resources = []string{resource}
	}
	_, cs, err := p.contextClientset(name)
	if err != nil {
		return nil, err
	}
	out := make([]remote.Workload, 0)
	for _, current := range resources {
		rows, err := listPhoneWorkloads(ctx, cs, current)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Resource != out[j].Resource {
			return out[i].Resource < out[j].Resource
		}
		return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name
	})
	return out, nil
}

func (p Phone) ContextLogs(ctx context.Context, contextName, namespace, name, container string) (io.ReadCloser, error) {
	session, cs, err := p.contextClientset(contextName)
	if err != nil {
		return nil, err
	}
	if container == "" {
		pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(pod.Spec.Containers))
		for _, item := range pod.Spec.Containers {
			names = append(names, item.Name)
		}
		chosen, _, err := remote.ChooseContainer(names)
		if err != nil {
			return nil, err
		}
		container = chosen
	}
	reader, writer := io.Pipe()
	go func() {
		err := session.StreamLogs(ctx, namespace, name, container, true, 200, func(lines []string) {
			if len(lines) == 0 {
				return
			}
			_, _ = writer.Write([]byte(strings.Join(lines, "\n") + "\n"))
		})
		_ = writer.CloseWithError(err)
	}()
	return reader, nil
}

func (p Phone) RestartOwnedPod(ctx context.Context, contextName, namespace, name string) error {
	_, cs, err := p.contextClientset(contextName)
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
	return cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (p Phone) DeletePod(ctx context.Context, contextName, namespace, name string) error {
	_, cs, err := p.contextClientset(contextName)
	if err != nil {
		return err
	}
	return cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func validPhoneWorkloadResource(resource string) bool {
	for _, candidate := range phoneWorkloadResources {
		if resource == candidate {
			return true
		}
	}
	return false
}

func listPhoneWorkloads(ctx context.Context, cs kubernetes.Interface, resource string) ([]remote.Workload, error) {
	switch resource {
	case "pods":
		list, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phonePod), nil
	case "deployments":
		list, err := cs.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneDeployment), nil
	case "replicasets":
		list, err := cs.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneReplicaSet), nil
	case "daemonsets":
		list, err := cs.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneDaemonSet), nil
	case "statefulsets":
		list, err := cs.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneStatefulSet), nil
	case "jobs":
		list, err := cs.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneJob), nil
	case "cronjobs":
		list, err := cs.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return mapPhoneWorkloads(list.Items, phoneCronJob), nil
	default:
		return nil, fmt.Errorf("unsupported workload resource %q", resource)
	}
}

func mapPhoneWorkloads[T any](items []T, project func(*T) remote.Workload) []remote.Workload {
	out := make([]remote.Workload, len(items))
	for i := range items {
		out[i] = project(&items[i])
	}
	return out
}

func phonePod(item *corev1.Pod) remote.Workload {
	ready := int32(0)
	status := "Unknown"
	switch item.Status.Phase {
	case corev1.PodRunning:
		status = "Active"
		if podReady(item) {
			ready, status = 1, "Healthy"
		}
	case corev1.PodPending:
		status = "Active"
	case corev1.PodSucceeded:
		ready, status = 1, "Succeeded"
	case corev1.PodFailed:
		status = "Failed"
	}
	containers := phonePodContainers(item)
	restarts := int32(0)
	for _, container := range containers {
		restarts += container.Restarts
	}
	return remote.Workload{
		Resource: "pods", Kind: "Pod", Namespace: item.Namespace, Name: item.Name,
		Status: status, Ready: ready, Desired: 1, CreatedAt: item.CreationTimestamp.Time,
		Containers: containers, Owned: ownedPod(item),
		Facts: []remote.WorkloadFact{
			{Label: "Phase", Value: string(item.Status.Phase)},
			{Label: "Node", Value: emptyFallback(item.Spec.NodeName, "Unscheduled")},
			{Label: "Pod IP", Value: emptyFallback(item.Status.PodIP, "Unavailable")},
			{Label: "QoS", Value: emptyFallback(string(item.Status.QOSClass), "Unknown")},
			{Label: "Restarts", Value: strconv.FormatInt(int64(restarts), 10)},
		},
	}
}

func phoneDeployment(item *appsv1.Deployment) remote.Workload {
	desired := replicas(item.Spec.Replicas)
	status := replicaStatus(item.Status.ReadyReplicas, desired)
	for _, condition := range item.Status.Conditions {
		failed := condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue
		timedOut := condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded"
		if failed || timedOut {
			status = "Failed"
		}
	}
	return remote.Workload{
		Resource: "deployments", Kind: "Deployment", Namespace: item.Namespace, Name: item.Name,
		Status: status, Ready: item.Status.ReadyReplicas, Desired: desired, CreatedAt: item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Updated", Value: strconv.FormatInt(int64(item.Status.UpdatedReplicas), 10)},
			{Label: "Available", Value: strconv.FormatInt(int64(item.Status.AvailableReplicas), 10)},
			{Label: "Strategy", Value: string(item.Spec.Strategy.Type)},
		},
	}
}

func phoneReplicaSet(item *appsv1.ReplicaSet) remote.Workload {
	desired := replicas(item.Spec.Replicas)
	return remote.Workload{
		Resource: "replicasets", Kind: "ReplicaSet", Namespace: item.Namespace, Name: item.Name,
		Status: replicaStatus(item.Status.ReadyReplicas, desired), Ready: item.Status.ReadyReplicas,
		Desired: desired, CreatedAt: item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Available", Value: strconv.FormatInt(int64(item.Status.AvailableReplicas), 10)},
			{Label: "Fully labeled", Value: strconv.FormatInt(int64(item.Status.FullyLabeledReplicas), 10)},
		},
	}
}

func phoneDaemonSet(item *appsv1.DaemonSet) remote.Workload {
	status := replicaStatus(item.Status.NumberReady, item.Status.DesiredNumberScheduled)
	if item.Status.NumberMisscheduled > 0 {
		status = "Failed"
	}
	return remote.Workload{
		Resource: "daemonsets", Kind: "DaemonSet", Namespace: item.Namespace, Name: item.Name,
		Status: status, Ready: item.Status.NumberReady, Desired: item.Status.DesiredNumberScheduled,
		CreatedAt:  item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Current", Value: strconv.FormatInt(int64(item.Status.CurrentNumberScheduled), 10)},
			{Label: "Available", Value: strconv.FormatInt(int64(item.Status.NumberAvailable), 10)},
			{Label: "Misscheduled", Value: strconv.FormatInt(int64(item.Status.NumberMisscheduled), 10)},
		},
	}
}

func phoneStatefulSet(item *appsv1.StatefulSet) remote.Workload {
	desired := replicas(item.Spec.Replicas)
	return remote.Workload{
		Resource: "statefulsets", Kind: "StatefulSet", Namespace: item.Namespace, Name: item.Name,
		Status: replicaStatus(item.Status.ReadyReplicas, desired), Ready: item.Status.ReadyReplicas,
		Desired: desired, CreatedAt: item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Current", Value: strconv.FormatInt(int64(item.Status.CurrentReplicas), 10)},
			{Label: "Updated", Value: strconv.FormatInt(int64(item.Status.UpdatedReplicas), 10)},
			{Label: "Service", Value: emptyFallback(item.Spec.ServiceName, "None")},
		},
	}
}

func phoneJob(item *batchv1.Job) remote.Workload {
	desired := int32(1)
	if item.Spec.Completions != nil && *item.Spec.Completions > 0 {
		desired = *item.Spec.Completions
	}
	status := "Active"
	if item.Spec.Suspend != nil && *item.Spec.Suspend {
		status = "Suspended"
	}
	for _, condition := range item.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobFailed, batchv1.JobFailureTarget:
			status = "Failed"
		case batchv1.JobComplete, batchv1.JobSuccessCriteriaMet:
			status = "Succeeded"
		case batchv1.JobSuspended:
			status = "Suspended"
		}
	}
	return remote.Workload{
		Resource: "jobs", Kind: "Job", Namespace: item.Namespace, Name: item.Name,
		Status: status, Ready: item.Status.Succeeded, Desired: desired, CreatedAt: item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Active", Value: strconv.FormatInt(int64(item.Status.Active), 10)},
			{Label: "Succeeded", Value: strconv.FormatInt(int64(item.Status.Succeeded), 10)},
			{Label: "Failed", Value: strconv.FormatInt(int64(item.Status.Failed), 10)},
			{Label: "Parallelism", Value: strconv.FormatInt(int64(pointerInt32(item.Spec.Parallelism, 1)), 10)},
		},
	}
}

func phoneCronJob(item *batchv1.CronJob) remote.Workload {
	status := "Scheduled"
	if item.Spec.Suspend != nil && *item.Spec.Suspend {
		status = "Suspended"
	} else if len(item.Status.Active) > 0 {
		status = "Active"
	}
	lastSchedule := "Never"
	if item.Status.LastScheduleTime != nil {
		lastSchedule = item.Status.LastScheduleTime.Time.Format("2006-01-02 15:04:05Z07:00")
	}
	return remote.Workload{
		Resource: "cronjobs", Kind: "CronJob", Namespace: item.Namespace, Name: item.Name,
		Status: status, Ready: int32(len(item.Status.Active)), Desired: 0, CreatedAt: item.CreationTimestamp.Time,
		Containers: phoneTemplateContainers(item.Spec.JobTemplate.Spec.Template.Spec.Containers),
		Facts: []remote.WorkloadFact{
			{Label: "Schedule", Value: item.Spec.Schedule},
			{Label: "Time zone", Value: emptyFallback(stringPointerValue(item.Spec.TimeZone), "Controller default")},
			{Label: "Active jobs", Value: strconv.Itoa(len(item.Status.Active))},
			{Label: "Last scheduled", Value: lastSchedule},
		},
	}
}

func phonePodContainers(item *corev1.Pod) []remote.WorkloadContainer {
	statuses := make(map[string]corev1.ContainerStatus, len(item.Status.ContainerStatuses))
	for _, status := range item.Status.ContainerStatuses {
		statuses[status.Name] = status
	}
	out := make([]remote.WorkloadContainer, 0, len(item.Spec.Containers))
	for _, container := range item.Spec.Containers {
		status := statuses[container.Name]
		out = append(out, remote.WorkloadContainer{
			Name: container.Name, Image: container.Image, Ready: status.Ready, Restarts: status.RestartCount,
		})
	}
	return out
}

func phoneTemplateContainers(items []corev1.Container) []remote.WorkloadContainer {
	out := make([]remote.WorkloadContainer, 0, len(items))
	for _, item := range items {
		out = append(out, remote.WorkloadContainer{Name: item.Name, Image: item.Image})
	}
	return out
}

func ownedPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
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
	for _, status := range pod.Status.ContainerStatuses {
		if !status.Ready {
			return false
		}
	}
	return true
}

func replicas(value *int32) int32 {
	if value == nil {
		return 1
	}
	return max(*value, 0)
}

func replicaStatus(ready, desired int32) string {
	if desired == 0 || ready >= desired {
		return "Healthy"
	}
	return "Active"
}

func pointerInt32(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	return *value
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
