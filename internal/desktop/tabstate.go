package desktop

import (
	"context"
	"io"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui/editor"

	"github.com/dacsang97/nakube/internal/cluster"
)

// tabBody is what one tab keeps while another is in front: the row, the
// overlay, and the list that screen fetched. The live watch stays on App.
// Drawing reads the active projection on App; a switch copies this in and out.
type tabBody struct {
	loaded bool

	row int

	mode       string
	logLines   []LogLine
	logFilter  string
	logFollow  bool
	logPod     string
	logGen     int
	logLoading bool
	logCancel  context.CancelFunc
	execTerm   *terminal.Terminal
	execConn   io.Closer
	execErr    string
	yaml       *yamlScreen

	scaleTo       string
	forwardLocal  string
	forwardRemote string
	rollbackRev   string

	helmRelease   string
	helmChart     string
	helmNamespace string
	helmValues    *editor.State
	helmDiff      *editor.DiffState
	helmBusy      bool
	helmDry       bool
	helmErr       string

	syncRev     string
	cascadeMode string
	pruneSync   bool

	listGen     int
	listID      string
	listLoading bool
	listErr     string
	listNotice  string
	listCols    []string
	listRows    []listRow

	argoApps          []cluster.ArgoApp
	argoSets          []cluster.ArgoApplicationSet
	argoProjects      []cluster.ArgoProject
	argoRepos         []cluster.ArgoRepository
	argoClusters      []cluster.ArgoCluster
	argoNotice        string
	argoErr           string
	argoRepoErr       string
	argoClusterErr    string
	argoAppsReady     bool
	argoSetsReady     bool
	argoProjectsReady bool
	argoReposReady    bool
	argoClustersReady bool
}

func newTabBody() *tabBody {
	return &tabBody{
		scaleTo:       "1",
		forwardLocal:  "8080",
		forwardRemote: "80",
		rollbackRev:   "1",
		helmNamespace: "default",
	}
}

func (b *tabBody) clearList() {
	b.listRows = nil
	b.listCols = nil
	b.listErr = ""
	b.listNotice = ""
	b.listLoading = true
	b.argoApps = nil
	b.argoSets = nil
	b.argoProjects = nil
	b.argoRepos = nil
	b.argoClusters = nil
	b.argoNotice = ""
	b.argoErr = ""
	b.argoRepoErr = ""
	b.argoClusterErr = ""
	b.argoAppsReady = false
	b.argoSetsReady = false
	b.argoProjectsReady = false
	b.argoReposReady = false
	b.argoClustersReady = false
}

func (a *App) ensureTabs() {
	if a.tabs != nil && a.active != "" && a.bodies != nil {
		return
	}
	if a.tabs == nil {
		a.tabs = newTabStrip()
	}
	if a.bodies == nil {
		a.bodies = map[string]*tabBody{}
	}
	if a.active == "" {
		a.active = a.tabs.Selected()
	}
	if a.bodies[a.active] == nil {
		a.bodies[a.active] = newTabBody()
	}
}

func (a *App) body(id string) *tabBody {
	a.ensureTabs()
	b := a.bodies[id]
	if b == nil {
		b = newTabBody()
		a.bodies[id] = b
	}
	return b
}

func (a *App) stash() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stashLocked()
}

func (a *App) stashLocked() {
	if a.active == "" {
		return
	}
	if a.bodies == nil {
		a.bodies = map[string]*tabBody{}
	}
	b := a.bodies[a.active]
	if b == nil {
		b = newTabBody()
		a.bodies[a.active] = b
	}
	saveInto(a, b)
}

func (a *App) restore() {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.bodies[a.active]
	if b == nil {
		return
	}
	loadFrom(a, b)
	if screen := a.tabs.Screen(a.active); screen != "" {
		a.selected = screen
	}
}

func (a *App) capture() *tabBody {
	b := newTabBody()
	saveInto(a, b)
	b.loaded = a.listID != "" || len(a.listRows) > 0 || len(a.argoApps) > 0
	return b
}

func saveInto(a *App, b *tabBody) {
	b.row = a.row
	b.mode = a.mode
	b.logLines = a.logLines
	b.logFilter = a.logFilter
	b.logFollow = a.logFollow
	b.logPod = a.logPod
	b.logGen = a.logGen
	b.logLoading = a.logLoading
	b.logCancel = a.logCancel
	b.execTerm = a.execTerm
	b.execConn = a.execConn
	b.execErr = a.execErr
	b.yaml = a.yaml
	b.scaleTo = a.scaleTo
	b.forwardLocal = a.forwardLocal
	b.forwardRemote = a.forwardRemote
	b.rollbackRev = a.rollbackRev
	b.helmRelease = a.helmRelease
	b.helmChart = a.helmChart
	b.helmNamespace = a.helmNamespace
	b.helmValues = a.helmValues
	b.helmDiff = a.helmDiff
	b.helmBusy = a.helmBusy
	b.helmDry = a.helmDry
	b.helmErr = a.helmErr
	b.syncRev = a.syncRev
	b.cascadeMode = a.cascadeMode
	b.pruneSync = a.pruneSync
	b.listGen = a.listGen
	b.listID = a.listID
	b.listLoading = a.listLoading
	b.listErr = a.listErr
	b.listNotice = a.listNotice
	b.listCols = a.listCols
	b.listRows = a.listRows
	b.argoApps = a.argoApps
	b.argoSets = a.argoSets
	b.argoProjects = a.argoProjects
	b.argoRepos = a.argoRepos
	b.argoClusters = a.argoClusters
	b.argoNotice = a.argoNotice
	b.argoErr = a.argoErr
	b.argoRepoErr = a.argoRepoErr
	b.argoClusterErr = a.argoClusterErr
	b.argoAppsReady = a.argoAppsReady
	b.argoSetsReady = a.argoSetsReady
	b.argoProjectsReady = a.argoProjectsReady
	b.argoReposReady = a.argoReposReady
	b.argoClustersReady = a.argoClustersReady
	b.loaded = a.listID != "" && !a.listLoading
}

func loadFrom(a *App, b *tabBody) {
	a.row = b.row
	a.mode = b.mode
	a.logLines = b.logLines
	a.logFilter = b.logFilter
	a.logFollow = b.logFollow
	a.logPod = b.logPod
	a.logGen = b.logGen
	a.logLoading = b.logLoading
	a.logCancel = b.logCancel
	a.execTerm = b.execTerm
	a.execConn = b.execConn
	a.execErr = b.execErr
	a.yaml = b.yaml
	a.scaleTo = b.scaleTo
	a.forwardLocal = b.forwardLocal
	a.forwardRemote = b.forwardRemote
	a.rollbackRev = b.rollbackRev
	a.helmRelease = b.helmRelease
	a.helmChart = b.helmChart
	a.helmNamespace = b.helmNamespace
	a.helmValues = b.helmValues
	a.helmDiff = b.helmDiff
	a.helmBusy = b.helmBusy
	a.helmDry = b.helmDry
	a.helmErr = b.helmErr
	a.syncRev = b.syncRev
	a.cascadeMode = b.cascadeMode
	a.pruneSync = b.pruneSync
	a.listGen = b.listGen
	a.listID = b.listID
	a.listLoading = b.listLoading
	a.listErr = b.listErr
	a.listNotice = b.listNotice
	a.listCols = b.listCols
	a.listRows = b.listRows
	a.argoApps = b.argoApps
	a.argoSets = b.argoSets
	a.argoProjects = b.argoProjects
	a.argoRepos = b.argoRepos
	a.argoClusters = b.argoClusters
	a.argoNotice = b.argoNotice
	a.argoErr = b.argoErr
	a.argoRepoErr = b.argoRepoErr
	a.argoClusterErr = b.argoClusterErr
	a.argoAppsReady = b.argoAppsReady
	a.argoSetsReady = b.argoSetsReady
	a.argoProjectsReady = b.argoProjectsReady
	a.argoReposReady = b.argoReposReady
	a.argoClustersReady = b.argoClustersReady
}

func (a *App) projectListLocked(b *tabBody) {
	a.listGen = b.listGen
	a.listID = b.listID
	a.listLoading = b.listLoading
	a.listErr = b.listErr
	a.listNotice = b.listNotice
	a.listCols = b.listCols
	a.listRows = b.listRows
	a.argoApps = b.argoApps
	a.argoSets = b.argoSets
	a.argoProjects = b.argoProjects
	a.argoRepos = b.argoRepos
	a.argoClusters = b.argoClusters
	a.argoNotice = b.argoNotice
	a.argoErr = b.argoErr
	a.argoRepoErr = b.argoRepoErr
	a.argoClusterErr = b.argoClusterErr
	a.argoAppsReady = b.argoAppsReady
	a.argoSetsReady = b.argoSetsReady
	a.argoProjectsReady = b.argoProjectsReady
	a.argoReposReady = b.argoReposReady
	a.argoClustersReady = b.argoClustersReady
}

func releaseBody(b *tabBody) {
	if b == nil {
		return
	}
	if b.logCancel != nil {
		b.logCancel()
		b.logCancel = nil
	}
	if b.execTerm != nil {
		b.execTerm.Close()
		b.execTerm = nil
	}
	if b.execConn != nil {
		_ = b.execConn.Close()
		b.execConn = nil
	}
}
