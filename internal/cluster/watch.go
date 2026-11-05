package cluster

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// WatchedKinds are the informers Watch starts. Secrets are not in this set.
var WatchedKinds = []string{
	"Pod",
	"Node",
	"Event",
	"Deployment",
	"ReplicaSet",
	"DaemonSet",
	"StatefulSet",
	"Job",
	"CronJob",
}

// WatchEvent is one informer notification. Object is a deep copy the caller
// owns; the informer may reuse its own object after the callback returns.
type WatchEvent struct {
	Type   string
	Kind   string
	Object runtime.Object
}

// WatchHandler receives owned copies. The caller coalesces them. This
// package does not redraw a window.
type WatchHandler func(WatchEvent)

// Watch starts informers for WatchedKinds. A second call stops the previous
// watches. Use also stops them.
func (s *Session) Watch(ctx context.Context, h WatchHandler) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cs, err := s.clientset()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.stopWatchLocked()
	wctx, cancel := context.WithCancel(ctx)
	s.watchCancel = cancel
	gen := s.watchGen
	s.mu.Unlock()

	factory := informers.NewSharedInformerFactoryWithOptions(cs, 0, informers.WithTransform(stripManagedFields))
	s.bindInformers(factory, gen, h)
	factory.Start(wctx.Done())

	s.mu.Lock()
	if s.watchGen == gen {
		s.factory = factory
	}
	s.mu.Unlock()
	return nil
}

// StopWatch cancels the current informers. Late callbacks are dropped.
func (s *Session) StopWatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopWatchLocked()
}

func (s *Session) bindInformers(factory informers.SharedInformerFactory, gen uint64, h WatchHandler) {
	core := factory.Core().V1()
	apps := factory.Apps().V1()
	batch := factory.Batch().V1()
	bind(core.Pods().Informer(), "Pod", gen, s, h)
	bind(core.Nodes().Informer(), "Node", gen, s, h)
	bind(core.Events().Informer(), "Event", gen, s, h)
	bind(apps.Deployments().Informer(), "Deployment", gen, s, h)
	bind(apps.ReplicaSets().Informer(), "ReplicaSet", gen, s, h)
	bind(apps.DaemonSets().Informer(), "DaemonSet", gen, s, h)
	bind(apps.StatefulSets().Informer(), "StatefulSet", gen, s, h)
	bind(batch.Jobs().Informer(), "Job", gen, s, h)
	bind(batch.CronJobs().Informer(), "CronJob", gen, s, h)
}

func bind(inf cache.SharedIndexInformer, kind string, gen uint64, s *Session, h WatchHandler) {
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { s.emit(gen, h, "add", kind, obj) },
		UpdateFunc: func(_, obj any) { s.emit(gen, h, "update", kind, obj) },
		DeleteFunc: func(obj any) { s.emit(gen, h, "delete", kind, obj) },
	})
}

func (s *Session) emit(gen uint64, h WatchHandler, typ, kind string, obj any) {
	copied := copyObject(obj)
	s.mu.Lock()
	current := s.watchGen == gen
	s.mu.Unlock()
	if !current || copied == nil || h == nil {
		return
	}
	h(WatchEvent{Type: typ, Kind: kind, Object: copied})
}

func copyObject(obj any) runtime.Object {
	if obj == nil {
		return nil
	}
	if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = deleted.Obj
	}
	ro, ok := obj.(runtime.Object)
	if !ok || ro == nil {
		return nil
	}
	return ro.DeepCopyObject()
}

func stripManagedFields(obj any) (any, error) {
	if acc, ok := obj.(metav1.Object); ok && acc != nil {
		acc.SetManagedFields(nil)
	}
	return obj, nil
}
