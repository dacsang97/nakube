package desktop

import (
	"context"
	"net/http"
	"time"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ContextArgoApplications reads Argo CD from the requested kubeconfig context
// without changing the desktop's selected or persisted context.
func (p Phone) ContextArgoApplications(ctx context.Context, name string) ([]remote.ArgoApplication, error) {
	if name == "" {
		return nil, &remote.StatusError{Status: http.StatusBadRequest, Msg: "context is required"}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	session, err := p.App.session.ForContext(name)
	if err != nil {
		return nil, err
	}
	apps, err := session.ListArgoApps(ctx)
	if apierrors.IsNotFound(err) {
		return nil, &remote.StatusError{Status: http.StatusServiceUnavailable, Msg: "Argo CD is unavailable in this context"}
	}
	if err != nil {
		return nil, err
	}
	out := make([]remote.ArgoApplication, len(apps))
	for i, app := range apps {
		out[i] = remoteArgoApplication(app)
	}
	return out, nil
}

func (p Phone) ContextArgoApplicationSets(ctx context.Context, name string) ([]remote.ArgoApplicationSet, error) {
	session, ctx, cancel, err := p.argoSession(ctx, name)
	if err != nil {
		return nil, err
	}
	defer cancel()
	items, err := session.ListArgoApplicationSets(ctx)
	if err != nil {
		return nil, argoReadError(err)
	}
	out := make([]remote.ArgoApplicationSet, len(items))
	for i, item := range items {
		out[i] = remote.ArgoApplicationSet{
			Name: item.Name, Namespace: item.Namespace, Generators: item.Generators,
			Project: item.Project, Destination: item.Destination,
			Conditions: append([]string(nil), item.Conditions...), CreatedAt: item.Created,
		}
	}
	return out, nil
}

func (p Phone) ContextArgoProjects(ctx context.Context, name string) ([]remote.ArgoProject, error) {
	session, ctx, cancel, err := p.argoSession(ctx, name)
	if err != nil {
		return nil, err
	}
	defer cancel()
	items, err := session.ListArgoProjects(ctx)
	if err != nil {
		return nil, argoReadError(err)
	}
	out := make([]remote.ArgoProject, len(items))
	for i, item := range items {
		out[i] = remote.ArgoProject{
			Name: item.Name, Namespace: item.Namespace, Description: item.Description,
			SourceRepos:  append([]string(nil), item.SourceRepos...),
			Destinations: append([]string(nil), item.Destinations...),
			Roles:        item.Roles, CreatedAt: item.Created,
		}
	}
	return out, nil
}

func (p Phone) ContextArgoRepositories(ctx context.Context, name string) ([]remote.ArgoRepository, error) {
	session, ctx, cancel, err := p.argoSession(ctx, name)
	if err != nil {
		return nil, err
	}
	defer cancel()
	items, err := session.ListArgoRepositories(ctx)
	if err != nil {
		return nil, argoReadError(err)
	}
	out := make([]remote.ArgoRepository, len(items))
	for i, item := range items {
		out[i] = remote.ArgoRepository{Name: item.Name, Namespace: item.Namespace, Type: item.Type, URL: item.URL, Project: item.Project}
	}
	return out, nil
}

func (p Phone) ContextArgoClusters(ctx context.Context, name string) ([]remote.ArgoCluster, error) {
	session, ctx, cancel, err := p.argoSession(ctx, name)
	if err != nil {
		return nil, err
	}
	defer cancel()
	items, err := session.ListArgoClusters(ctx)
	if err != nil {
		return nil, argoReadError(err)
	}
	out := make([]remote.ArgoCluster, len(items))
	for i, item := range items {
		out[i] = remote.ArgoCluster{Name: item.Name, Namespace: item.Namespace, Server: item.Server}
	}
	return out, nil
}

func (p Phone) SyncArgoApplication(ctx context.Context, contextName, namespace, name string, request remote.ArgoSyncRequest) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	return session.SyncArgo(ctx, namespace, name, cluster.ArgoSyncOptions{
		Revision: request.Revision, Strategy: request.Strategy, DryRun: request.DryRun, Prune: request.Prune,
	})
}

func (p Phone) RefreshArgoApplication(ctx context.Context, contextName, namespace, name string, request remote.ArgoRefreshRequest) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	return session.RefreshArgo(ctx, namespace, name, request.Kind)
}

func (p Phone) RollbackArgoApplication(ctx context.Context, contextName, namespace, name string, request remote.ArgoRollbackRequest) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	return session.RollbackArgo(ctx, namespace, name, request.ID, request.Prune)
}

func (p Phone) DeleteArgoResource(ctx context.Context, contextName, resource, namespace, name string, request remote.ArgoDeleteRequest) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	if resource == "applications" {
		return session.DeleteArgo(ctx, namespace, name, request.Cascade)
	}
	kind := ""
	switch resource {
	case "applicationsets":
		kind = "ApplicationSet"
	case "appprojects":
		kind = "AppProject"
	default:
		return &remote.StatusError{Status: http.StatusNotFound, Msg: "Argo resource not found"}
	}
	return session.Delete(ctx, cluster.Identity{
		GVR:  schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: resource},
		Kind: kind, Namespace: namespace, Name: name,
	})
}

func (p Phone) SyncOutOfSyncArgoApplications(ctx context.Context, contextName string, request remote.ArgoFleetRequest) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	apps, err := session.ListArgoApps(ctx)
	if err != nil {
		return argoReadError(err)
	}
	var targets []syncTarget
	for _, app := range apps {
		if app.Sync == "OutOfSync" {
			targets = append(targets, syncTarget{
				Namespace: app.Namespace,
				Name:      app.Name,
				Revision:  app.TargetRevision,
			})
		}
	}
	if len(targets) == 0 {
		return &remote.StatusError{Status: http.StatusConflict, Msg: "nothing to sync"}
	}
	err = applyTargets(targets, func(target syncTarget) error {
		return session.SyncArgo(ctx, target.Namespace, target.Name, cluster.ArgoSyncOptions{
			Revision: target.Revision,
			Prune:    request.Prune,
		})
	})
	if err != nil {
		return &remote.StatusError{Status: http.StatusBadGateway, Msg: err.Error()}
	}
	return nil
}

func (p Phone) RefreshAllArgoApplications(ctx context.Context, contextName string) error {
	session, ctx, cancel, err := p.argoSession(ctx, contextName)
	if err != nil {
		return err
	}
	defer cancel()
	apps, err := session.ListArgoApps(ctx)
	if err != nil {
		return argoReadError(err)
	}
	if len(apps) == 0 {
		return &remote.StatusError{Status: http.StatusConflict, Msg: "nothing to refresh"}
	}
	targets := make([]syncTarget, len(apps))
	for i, app := range apps {
		targets[i] = syncTarget{Namespace: app.Namespace, Name: app.Name}
	}
	err = applyTargets(targets, func(target syncTarget) error {
		return session.RefreshArgo(ctx, target.Namespace, target.Name, "normal")
	})
	if err != nil {
		return &remote.StatusError{Status: http.StatusBadGateway, Msg: err.Error()}
	}
	return nil
}

func (p Phone) argoSession(ctx context.Context, name string) (*cluster.Session, context.Context, context.CancelFunc, error) {
	if name == "" {
		return nil, nil, nil, &remote.StatusError{Status: http.StatusBadRequest, Msg: "context is required"}
	}
	timed, cancel := context.WithTimeout(ctx, 20*time.Second)
	session, err := p.App.session.ForContext(name)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return session, timed, cancel, nil
}

func argoReadError(err error) error {
	if apierrors.IsNotFound(err) {
		return &remote.StatusError{Status: http.StatusServiceUnavailable, Msg: "Argo CD is unavailable in this context"}
	}
	return err
}

func remoteArgoApplication(app cluster.ArgoApp) remote.ArgoApplication {
	sources := make([]remote.ArgoSource, len(app.Sources))
	for i, source := range app.Sources {
		sources[i] = remote.ArgoSource{
			RepoURL: source.RepoURL, Path: source.Path, Chart: source.Chart,
			TargetRevision: source.TargetRevision, Ref: source.Ref,
		}
	}
	resources := make([]remote.ArgoResource, len(app.Resources))
	for i, resource := range app.Resources {
		resources[i] = remote.ArgoResource{
			Kind: resource.Kind, Namespace: resource.Namespace, Name: resource.Name,
			Sync: resource.Sync, Health: resource.Health, Hook: resource.Hook,
		}
	}
	history := make([]remote.ArgoHistory, len(app.History))
	for i, item := range app.History {
		history[i] = remote.ArgoHistory{ID: item.ID, Revision: item.Revision, By: item.By}
	}
	return remote.ArgoApplication{
		Name: app.Name, Namespace: app.Namespace, Project: app.Project,
		Sync: app.Sync, Health: app.Health, HealthMessage: app.HealthMessage,
		Revision: app.Revision, TargetRevision: app.TargetRevision,
		Destination: app.Destination, Policy: app.Policy, Source: app.Source,
		Sources: sources, Resources: resources, History: history,
		Conditions:     append([]string(nil), app.Conditions...),
		OperationPhase: app.OperationPhase, OperationMessage: app.OperationMessage,
		OperationBy: app.OperationBy, Images: append([]string(nil), app.Images...),
		SourceType: app.SourceType, CreatedAt: app.Created,
	}
}
