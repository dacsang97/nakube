package desktop

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPhoneWorkloadResourcesReturnDesignedRows(t *testing.T) {
	template := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "web:v1"}}}}
	objects := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"},
			Spec:       template.Spec,
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionTrue,
			}}},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "deployment", Namespace: "ns"},
			Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2)), Template: template},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "replicaset", Namespace: "ns"},
			Spec:       appsv1.ReplicaSetSpec{Replicas: new(int32(2)), Template: template},
			Status:     appsv1.ReplicaSetStatus{ReadyReplicas: 2},
		},
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "daemonset", Namespace: "ns"},
			Spec:       appsv1.DaemonSetSpec{Template: template},
			Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 2},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "statefulset", Namespace: "ns"},
			Spec:       appsv1.StatefulSetSpec{Replicas: new(int32(2)), Template: template},
			Status:     appsv1.StatefulSetStatus{ReadyReplicas: 2},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns"},
			Spec:       batchv1.JobSpec{Completions: new(int32(1)), Template: template},
			Status: batchv1.JobStatus{Succeeded: 1, Conditions: []batchv1.JobCondition{{
				Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
			}}},
		},
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Name: "cronjob", Namespace: "ns"},
			Spec: batchv1.CronJobSpec{
				Schedule:    "*/5 * * * *",
				JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: template}},
			},
		},
	}
	client := fake.NewClientset(objects...)
	for _, resource := range phoneWorkloadResources {
		t.Run(resource, func(t *testing.T) {
			rows, err := listPhoneWorkloads(context.Background(), client, resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].Resource != resource || rows[0].Namespace != "ns" ||
				rows[0].Status == "" || rows[0].Containers == nil || rows[0].Facts == nil {
				t.Fatalf("%s rows=%+v", resource, rows)
			}
		})
	}
}

func TestPhoneWorkloadResourceAllowlist(t *testing.T) {
	for _, resource := range phoneWorkloadResources {
		if !validPhoneWorkloadResource(resource) {
			t.Fatalf("locked workload resource %q is missing", resource)
		}
	}
	if validPhoneWorkloadResource("secrets") {
		t.Fatal("unexpected resource accepted")
	}
}
