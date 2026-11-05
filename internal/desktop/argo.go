package desktop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/argo"
)

func isArgoScreen(id string) bool {
	switch id {
	case "argo", "argo-applications", "argo-applicationsets", "argo-appprojects", "argo-repositories", "argo-clusters":
		return true
	default:
		return false
	}
}

func (a *App) drawArgo(c *ui.Context, t *ui.Theme, id string) {
	a.mu.Lock()
	page := a.argoPageLocked(id)
	a.mu.Unlock()
	argo.Draw(c, t, page)
}

func (a *App) argoPageLocked(id string) argo.Page {
	title := "Argo"
	if id != "argo" {
		title = labelFor(id)
	}
	page := argo.Page{
		ID:              id,
		Title:           title,
		Context:         a.current,
		Notice:          a.argoNotice,
		Err:             a.argoErr,
		Loading:         a.listID != id || a.listLoading,
		AppsLoading:     id == "argo" && (a.listID != id || a.listLoading) && !a.argoAppsReady,
		SetsLoading:     id == "argo" && (a.listID != id || a.listLoading) && !a.argoSetsReady,
		ProjectsLoading: id == "argo" && (a.listID != id || a.listLoading) && !a.argoProjectsReady,
		ReposLoading:    id == "argo" && (a.listID != id || a.listLoading) && !a.argoReposReady,
		ClustersLoading: id == "argo" && (a.listID != id || a.listLoading) && !a.argoClustersReady,
		Now:             time.Now(),
		Selected:        &a.row,
		Revision:        &a.rollbackRev,
		SyncRev:         &a.syncRev,
		Cascade:         &a.cascadeMode,
		Prune:           &a.pruneSync,
		OnOpen:          a.openPage,
	}
	if id != "argo" && page.Loading {
		return page
	}
	if id == "argo-repositories" && a.argoRepoErr != "" {
		page.Err = a.argoRepoErr
		return page
	}
	if id == "argo-clusters" && a.argoClusterErr != "" {
		page.Err = a.argoClusterErr
		return page
	}
	page.Apps = mapArgoApps(a.argoApps)
	page.Sets = mapArgoSets(a.argoSets)
	page.Projects = mapArgoProjects(a.argoProjects)
	page.Repos = mapArgoRepos(a.argoRepos)
	page.Clusters = mapArgoClusters(a.argoClusters)
	page.Activity = argoActivity(a.events)
	apps := append([]cluster.ArgoApp(nil), a.argoApps...)
	sets := append([]cluster.ArgoApplicationSet(nil), a.argoSets...)
	projects := append([]cluster.ArgoProject(nil), a.argoProjects...)
	page.OnAction = func(index int, action string) {
		a.runArgoAction(id, index, action, apps, sets, projects)
	}
	page.OnFleet = func(action string) {
		a.askFleet(action, apps)
	}
	return page
}

func (a *App) askFleet(action string, apps []cluster.ArgoApp) {
	a.mu.Lock()
	prune := a.pruneSync
	a.mu.Unlock()
	var targets []syncTarget
	switch action {
	case "sync-drift":
		for _, app := range apps {
			if app.Sync != "OutOfSync" {
				continue
			}
			targets = append(targets, syncTarget{Namespace: app.Namespace, Name: app.Name, Revision: app.TargetRevision})
		}
	case "refresh-all":
		for _, app := range apps {
			targets = append(targets, syncTarget{Namespace: app.Namespace, Name: app.Name})
		}
	default:
		return
	}
	if len(targets) == 0 {
		if action == "refresh-all" {
			a.setErr("nothing to refresh")
			return
		}
		a.setErr("nothing to sync")
		return
	}
	a.mu.Lock()
	a.confirm = &Confirm{
		Open:    true,
		Kind:    "Application",
		Name:    fmt.Sprintf("%d applications", len(targets)),
		Action:  action,
		Prune:   prune,
		Targets: targets,
	}
	a.mu.Unlock()
	a.invalidate()
}

func applySyncTargets(ctx context.Context, a *App, c Confirm) error {
	return applyTargets(c.Targets, func(target syncTarget) error {
		return a.session.SyncArgo(ctx, target.Namespace, target.Name, cluster.ArgoSyncOptions{Revision: target.Revision, Prune: c.Prune})
	})
}

func applyRefreshTargets(ctx context.Context, a *App, c Confirm) error {
	return applyTargets(c.Targets, func(target syncTarget) error {
		return a.session.RefreshArgo(ctx, target.Namespace, target.Name, "normal")
	})
}

func applyTargets(targets []syncTarget, fn func(syncTarget) error) error {
	var b strings.Builder
	for _, target := range targets {
		if err := fn(target); err != nil {
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s/%s: %v", target.Namespace, target.Name, err)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	return fmt.Errorf("%s", b.String())
}

func (a *App) loadArgo(gen int, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var (
		apps       []cluster.ArgoApp
		sets       []cluster.ArgoApplicationSet
		projects   []cluster.ArgoProject
		repos      []cluster.ArgoRepository
		clusters   []cluster.ArgoCluster
		notice     string
		repoErr    string
		clusterErr string
		err        error
	)
	switch id {
	case "argo":
		a.loadArgoDashboard(ctx, gen, id)
		return
	case "argo-applications":
		apps, notice, err = a.fetchArgoApps(ctx)
	case "argo-applicationsets":
		sets, notice, err = a.fetchArgoSetsNotice(ctx)
	case "argo-appprojects":
		projects, notice, err = a.fetchArgoProjectsNotice(ctx)
	case "argo-repositories":
		repos, repoErr = a.fetchArgoRepos(ctx)
	case "argo-clusters":
		clusters, clusterErr = a.fetchArgoClusters(ctx)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.listGen || id != a.selected {
		return
	}
	a.listLoading = false
	a.argoApps = apps
	a.argoSets = sets
	a.argoProjects = projects
	a.argoRepos = repos
	a.argoClusters = clusters
	a.argoNotice = notice
	a.argoRepoErr = repoErr
	a.argoClusterErr = clusterErr
	a.argoErr = ""
	if err != nil {
		a.argoErr = publicError(err)
	}
	a.invalidate()
}

func (a *App) loadArgoDashboard(ctx context.Context, gen int, id string) {
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		apps, notice, err := a.fetchArgoApps(ctx)
		a.storeArgo(gen, id, func() {
			a.argoApps = apps
			a.argoAppsReady = true
			a.argoNotice = notice
			if err != nil {
				a.argoErr = publicError(err)
			}
		})
	}()
	go func() {
		defer wg.Done()
		sets, err := a.fetchArgoSets(ctx, false)
		a.storeArgo(gen, id, func() {
			a.argoSets = sets
			a.argoSetsReady = true
			if err != nil && a.argoErr == "" {
				a.argoErr = publicError(err)
			}
		})
	}()
	go func() {
		defer wg.Done()
		projects, err := a.fetchArgoProjects(ctx, false)
		a.storeArgo(gen, id, func() {
			a.argoProjects = projects
			a.argoProjectsReady = true
			if err != nil && a.argoErr == "" {
				a.argoErr = publicError(err)
			}
		})
	}()
	go func() {
		defer wg.Done()
		repos, repoErr := a.fetchArgoRepos(ctx)
		a.storeArgo(gen, id, func() {
			a.argoRepos = repos
			a.argoReposReady = true
			a.argoRepoErr = repoErr
		})
	}()
	go func() {
		defer wg.Done()
		clusters, clusterErr := a.fetchArgoClusters(ctx)
		a.storeArgo(gen, id, func() {
			a.argoClusters = clusters
			a.argoClustersReady = true
			a.argoClusterErr = clusterErr
		})
	}()
	wg.Wait()
	a.storeArgo(gen, id, func() { a.listLoading = false })
}

func (a *App) storeArgo(gen int, id string, fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.listGen || id != a.selected {
		return
	}
	fn()
	a.invalidate()
}

func (a *App) fetchArgoApps(ctx context.Context) ([]cluster.ArgoApp, string, error) {
	apps, err := a.session.ListArgoApps(ctx)
	if apierrors.IsNotFound(err) {
		return nil, "CRD is not installed", nil
	}
	return apps, "", err
}

func (a *App) fetchArgoSets(ctx context.Context, noticeOnMissing bool) ([]cluster.ArgoApplicationSet, error) {
	sets, err := a.session.ListArgoApplicationSets(ctx)
	if noticeOnMissing || !apierrors.IsNotFound(err) {
		return sets, err
	}
	return nil, nil
}

func (a *App) fetchArgoSetsNotice(ctx context.Context) ([]cluster.ArgoApplicationSet, string, error) {
	sets, err := a.session.ListArgoApplicationSets(ctx)
	if apierrors.IsNotFound(err) {
		return nil, "CRD is not installed", nil
	}
	return sets, "", err
}

func (a *App) fetchArgoProjects(ctx context.Context, noticeOnMissing bool) ([]cluster.ArgoProject, error) {
	projects, err := a.session.ListArgoProjects(ctx)
	if noticeOnMissing || !apierrors.IsNotFound(err) {
		return projects, err
	}
	return nil, nil
}

func (a *App) fetchArgoProjectsNotice(ctx context.Context) ([]cluster.ArgoProject, string, error) {
	projects, err := a.session.ListArgoProjects(ctx)
	if apierrors.IsNotFound(err) {
		return nil, "CRD is not installed", nil
	}
	return projects, "", err
}

func (a *App) fetchArgoRepos(ctx context.Context) ([]cluster.ArgoRepository, string) {
	repos, err := a.session.ListArgoRepositories(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return repos, ""
}

func (a *App) fetchArgoClusters(ctx context.Context) ([]cluster.ArgoCluster, string) {
	clusters, err := a.session.ListArgoClusters(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return clusters, ""
}

func (a *App) runArgoAction(id string, index int, action string, apps []cluster.ArgoApp, sets []cluster.ArgoApplicationSet, projects []cluster.ArgoProject) {
	var ident cluster.Identity
	var target string
	switch id {
	case "argo-applications":
		if index < 0 || index >= len(apps) {
			return
		}
		app := apps[index]
		ident = argoIdentity("applications", "Application", app.Namespace, app.Name)
		target = app.TargetRevision
	case "argo-applicationsets":
		if index < 0 || index >= len(sets) {
			return
		}
		item := sets[index]
		ident = argoIdentity("applicationsets", "ApplicationSet", item.Namespace, item.Name)
	case "argo-appprojects":
		if index < 0 || index >= len(projects) {
			return
		}
		item := projects[index]
		ident = argoIdentity("appprojects", "AppProject", item.Namespace, item.Name)
	default:
		return
	}
	if action == "yaml" {
		a.openYAML(ident)
		return
	}
	a.ask(listRow{Namespace: ident.Namespace, Name: ident.Name, Identity: ident}, action)
	a.mu.Lock()
	if a.confirm != nil && a.confirm.Open {
		a.confirm.Prune = a.pruneSync
		a.confirm.Cascade = a.cascadeMode
		if action == "sync" {
			rev := strings.TrimSpace(a.syncRev)
			if rev == "" {
				rev = target
			}
			a.confirm.SyncRevision = rev
		}
	}
	a.mu.Unlock()
}

func argoIdentity(resource, kind, namespace, name string) cluster.Identity {
	return cluster.Identity{
		GVR:       schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: resource},
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	}
}

func mapArgoApps(in []cluster.ArgoApp) []argo.App {
	out := make([]argo.App, len(in))
	for i, app := range in {
		resources := make([]argo.Resource, len(app.Resources))
		for j, row := range app.Resources {
			resources[j] = argo.Resource{Kind: row.Kind, Namespace: row.Namespace, Name: row.Name, Sync: row.Sync, Health: row.Health, Hook: row.Hook}
		}
		history := make([]argo.History, len(app.History))
		for j, row := range app.History {
			history[j] = argo.History{ID: row.ID, Revision: row.Revision, By: row.By}
		}
		out[i] = argo.App{
			Namespace:        app.Namespace,
			Name:             app.Name,
			Project:          app.Project,
			Sync:             app.Sync,
			Health:           app.Health,
			HealthMessage:    app.HealthMessage,
			Revision:         app.Revision,
			TargetRevision:   app.TargetRevision,
			Destination:      app.Destination,
			Policy:           app.Policy,
			Source:           app.Source,
			Resources:        resources,
			History:          history,
			Conditions:       append([]string(nil), app.Conditions...),
			OperationPhase:   app.OperationPhase,
			OperationMessage: app.OperationMessage,
			OperationBy:      app.OperationBy,
			Images:           append([]string(nil), app.Images...),
			SourceType:       app.SourceType,
		}
	}
	return out
}

func mapArgoSets(in []cluster.ArgoApplicationSet) []argo.Set {
	out := make([]argo.Set, len(in))
	for i, item := range in {
		out[i] = argo.Set{
			Namespace:   item.Namespace,
			Name:        item.Name,
			Generators:  item.Generators,
			Project:     item.Project,
			Destination: item.Destination,
			Conditions:  append([]string(nil), item.Conditions...),
		}
	}
	return out
}

func mapArgoProjects(in []cluster.ArgoProject) []argo.Project {
	out := make([]argo.Project, len(in))
	for i, item := range in {
		out[i] = argo.Project{
			Namespace:    item.Namespace,
			Name:         item.Name,
			Description:  item.Description,
			SourceRepos:  append([]string(nil), item.SourceRepos...),
			Destinations: append([]string(nil), item.Destinations...),
			Roles:        item.Roles,
		}
	}
	return out
}

func mapArgoRepos(in []cluster.ArgoRepository) []argo.Repo {
	out := make([]argo.Repo, len(in))
	for i, item := range in {
		out[i] = argo.Repo{Namespace: item.Namespace, Name: item.Name, Type: item.Type, URL: item.URL, Project: item.Project}
	}
	return out
}

func mapArgoClusters(in []cluster.ArgoCluster) []argo.Cluster {
	out := make([]argo.Cluster, len(in))
	for i, item := range in {
		out[i] = argo.Cluster{Namespace: item.Namespace, Name: item.Name, Server: item.Server}
	}
	return out
}

func argoActivity(events map[string]*corev1.Event) []argo.Activity {
	out := make([]argo.Activity, 0)
	for _, ev := range events {
		if ev == nil || !isArgoEvent(ev) {
			continue
		}
		when := ev.LastTimestamp.Time
		if when.IsZero() {
			when = ev.EventTime.Time
		}
		if when.IsZero() {
			when = ev.CreationTimestamp.Time
		}
		object := ev.InvolvedObject.Kind
		if ev.Namespace != "" || ev.InvolvedObject.Name != "" {
			object += " " + ev.Namespace + "/" + ev.InvolvedObject.Name
		}
		out = append(out, argo.Activity{
			Reason:   ev.Reason,
			Object:   strings.TrimSpace(object),
			Body:     ev.Message,
			Category: argo.Category(ev.Type, ev.Reason),
			When:     when,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].When.After(out[j].When) })
	return out
}

func isArgoEvent(ev *corev1.Event) bool {
	switch ev.InvolvedObject.Kind {
	case "Application", "ApplicationSet", "AppProject":
		return true
	default:
		return strings.Contains(ev.InvolvedObject.APIVersion, "argoproj.io")
	}
}
