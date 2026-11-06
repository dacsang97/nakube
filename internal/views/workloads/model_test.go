package workloads

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestBuildNormalizesWorkloadHealth(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	two := int32(2)
	objects := []runtime.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod", CreationTimestamp: metav1.NewTime(now.Add(-time.Hour))}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"}, Spec: appsv1.DeploymentSpec{Replicas: &two}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "ops"}, Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 2, NumberMisscheduled: 1}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ops"}, Status: batchv1.JobStatus{Succeeded: 1, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "ops"}},
	}

	page := Build(objects, "", 0, ColKind, false)
	if page.Total != 5 || page.Healthy != 3 || page.Active != 1 || page.Attention != 1 {
		t.Fatalf("summary = total %d healthy %d active %d attention %d", page.Total, page.Healthy, page.Active, page.Attention)
	}
	if page.Ready != 5 || page.Desired != 7 {
		t.Fatalf("readiness = %d/%d", page.Ready, page.Desired)
	}
	if len(page.Rows) != 5 || page.Rows[0].Kind != "CronJob" || page.Rows[4].Kind != "Pod" {
		t.Fatalf("rows = %+v", page.Rows)
	}
}

func TestBuildFiltersSearchAndAttention(t *testing.T) {
	objects := []runtime.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "broken", Namespace: "prod"}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
	}
	page := Build(objects, "BROK", 3, ColName, false)
	if len(page.Rows) != 1 || page.Rows[0].Name != "broken" || page.Total != 2 {
		t.Fatalf("filtered page = %+v", page)
	}
}

func TestDeploymentFailureConditionWinsOverReplicaCount(t *testing.T) {
	one := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"},
		Spec:       appsv1.DeploymentSpec{Replicas: &one},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
			Conditions:    []appsv1.DeploymentCondition{{Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue}},
		},
	}
	page := Build([]runtime.Object{deployment}, "", 0, "", false)
	if len(page.Rows) != 1 || page.Rows[0].Status != StatusFailed || page.Attention != 1 {
		t.Fatalf("page = %+v", page)
	}
}
