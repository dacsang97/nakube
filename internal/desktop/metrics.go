package desktop

import (
	"context"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"k8s.io/client-go/kubernetes"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/metrics"
)

const (
	metricsEvery = 60 * time.Second // a shown block fetches again after this
	metricsKeep  = 5 * time.Minute  // an entry nobody drew is dropped after this
	metricsWait  = 20 * time.Second // one fetch, discovery and queries
)

// metricsKey is one chart: a pod's tab over a range, in one context.
type metricsKey struct {
	Context   string
	Namespace string
	Pod       string
	Tab       string
	Range     string
}

// metricsEntry is a chart's last result. busy is a fetch in flight; it is
// never started twice.
type metricsEntry struct {
	view    metrics.View
	fetched time.Time
	busy    bool
	seen    time.Time
}

// metricsBlock is the reader's choice in the open Metrics block. It starts
// over on CPU and an hour whenever another resource is chosen.
type metricsBlock struct {
	target string
	tab    string
	rng    string
	hover  int
}

// metricsShown is the last chart drawn with data, kept so a new range of
// the same pod and tab can stay on it while it loads.
type metricsShown struct {
	key  metricsKey
	view metrics.View
}

// promFound is a context's Prometheus, or why there is none. A miss is
// tried again after metricsEvery, as the stack may be installed meanwhile.
type promFound struct {
	src      cluster.PromSource
	err      error
	at       time.Time
	override string
}

// metricsFetcher runs the queries for one key. Tests replace it.
type metricsFetcher func(ctx context.Context, key metricsKey) metrics.View

// drawPodMetrics draws the block for a pod and starts a fetch when its
// chart is missing or older than metricsEvery.
func (a *App) drawPodMetrics(c *ui.Context, t *ui.Theme, namespace, name string) {
	b := &a.metricsUI
	contextName := a.session.Current()
	if target := contextName + "\x00" + namespace + "/" + name; b.target != target {
		*b = metricsBlock{target: target, tab: metrics.DefaultTab, rng: metrics.DefaultRange, hover: -1}
	}
	key := metricsKey{Context: contextName, Namespace: namespace, Pod: name, Tab: b.tab, Range: b.rng}
	view := a.metricsView(key, time.Now())
	tab, rng := b.tab, b.rng
	metrics.Draw(c, t, view, &b.tab, &b.rng, &b.hover, metricsIcons())
	if b.tab != tab || b.rng != rng {
		b.hover = -1
		a.invalidate()
	}
}

// metricsView returns what to draw for key and fetches when due. While a
// new range loads, the last chart of the same tab stays, so the block does
// not flash empty.
func (a *App) metricsView(key metricsKey, now time.Time) metrics.View {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.metricsCache == nil {
		a.metricsCache = map[metricsKey]*metricsEntry{}
	}
	for k, e := range a.metricsCache {
		if !e.busy && now.Sub(e.seen) > metricsKeep {
			delete(a.metricsCache, k)
		}
	}
	e := a.metricsCache[key]
	if e == nil {
		e = &metricsEntry{}
		a.metricsCache[key] = e
	}
	e.seen = now
	if !e.busy && (e.fetched.IsZero() || now.Sub(e.fetched) >= metricsEvery) {
		e.busy = true
		go a.runMetricsFetch(key, e)
	}
	view := e.view
	view.Tab = key.Tab
	if e.fetched.IsZero() {
		view.Loading = true
		last := a.metricsShown
		last.key.Range = key.Range
		if last.key == key && last.view.Err == "" && len(last.view.Results) > 0 {
			view.Source, view.Results = last.view.Source, last.view.Results
		}
	} else {
		a.metricsShown = metricsShown{key: key, view: view}
	}
	return view
}

// runMetricsFetch writes into e only while e is still the key's entry: a
// fetch begun before a forget lands nowhere, so it cannot clear a newer
// fetch's busy mark or replace its chart.
func (a *App) runMetricsFetch(key metricsKey, e *metricsEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsWait)
	defer cancel()
	fetch := a.metricsFetch
	if fetch == nil {
		fetch = a.fetchPodMetrics
	}
	view := fetch(ctx, key)
	a.mu.Lock()
	if a.metricsCache[key] == e {
		e.view = view
		e.fetched = time.Now()
		e.busy = false
	}
	a.mu.Unlock()
	a.invalidate()
	// Draw again once the chart is due, which fetches only if it is still
	// shown.
	time.AfterFunc(metricsEvery, a.invalidate)
}

// fetchPodMetrics finds Prometheus and runs a tab's queries side by side.
// The first failure is the block's message; no number is invented for it.
func (a *App) fetchPodMetrics(ctx context.Context, key metricsKey) metrics.View {
	cs, err := a.session.Clientset()
	if err != nil {
		return metrics.View{Err: metrics.Message(err)}
	}
	if a.session.Current() != key.Context {
		return metrics.View{Err: "Context changed."}
	}
	src, err := a.promSource(ctx, cs, key.Context)
	if err != nil {
		return metrics.View{Err: metrics.Message(err)}
	}
	end := time.Now().Truncate(time.Minute)
	rng := metrics.RangeDur(key.Range)
	step := cluster.MetricStep(rng)
	start := end.Add(-rng)
	queries := cluster.PodQueries(key.Tab, key.Namespace, []string{key.Pod}, "", step)
	results := make([]metrics.Result, len(queries))
	errs := make([]error, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			points, err := cluster.QueryRange(ctx, cs.CoreV1().RESTClient(), src, q.Expr, start, end, step)
			results[i] = metrics.Result{Name: q.Name, Line: q.Line, Points: points}
			errs[i] = err
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err == nil {
			continue
		}
		if metrics.SourceGone(err) {
			// The service moved or lost its pods: find it again next time.
			a.mu.Lock()
			delete(a.promFound, key.Context)
			a.mu.Unlock()
		}
		return metrics.View{Source: src.String(), Err: metrics.Message(err)}
	}
	return metrics.View{Source: src.String(), Results: results}
}

// promSource is the context's Prometheus, found once and kept. A miss is
// found again after metricsEvery, and a changed override at once.
func (a *App) promSource(ctx context.Context, cs kubernetes.Interface, contextName string) (cluster.PromSource, error) {
	a.mu.Lock()
	override := a.promOverridesLocked()[contextName]
	found, ok := a.promFound[contextName]
	a.mu.Unlock()
	if ok && found.override == override && (found.err == nil || time.Since(found.at) < metricsEvery) {
		return found.src, found.err
	}
	src, err := cluster.DiscoverPrometheus(ctx, cs, override)
	a.mu.Lock()
	if a.promFound == nil {
		a.promFound = map[string]promFound{}
	}
	a.promFound[contextName] = promFound{src: src, err: err, at: time.Now(), override: override}
	a.mu.Unlock()
	return src, err
}

// forgetMetricsLocked drops every chart and found source, as on a context
// switch or a new override. A fetch in flight lands in no entry.
func (a *App) forgetMetricsLocked() {
	a.metricsCache = map[metricsKey]*metricsEntry{}
	a.promFound = map[string]promFound{}
	a.metricsShown = metricsShown{}
}

func metricsIcons() metrics.Icons {
	return metrics.Icons{
		cluster.MetricCPU:        IconNamed("cpu"),
		cluster.MetricMemory:     IconNamed("memory-stick"),
		cluster.MetricNetwork:    IconNamed("arrow-up-down"),
		cluster.MetricFilesystem: IconNamed("hard-drive"),
	}
}
