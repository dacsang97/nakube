package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/metrics"
	podview "github.com/dacsang97/nakube/internal/views/pods"
)

func podsApp(fetch podUsageFetcher) *App {
	a := New(cluster.NewSession())
	a.redraw = func() {}
	a.podUsageFetch = fetch
	a.metricsFetch = func(context.Context, metricsKey) metrics.View { return metrics.View{} }
	a.pods = map[string]*corev1.Pod{
		"shop/web": {
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "web", CreationTimestamp: metav1.Now()},
			Spec:       corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Name: "app"}}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		"ops/job": {
			ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "job"},
			Spec:       corev1.PodSpec{NodeName: "node-b"},
		},
	}
	a.watchSettled = true
	a.showScreen("pods", true)
	return a
}

func noUsage(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
	return nil, false, nil
}

func TestPodsScreenIsInspect(t *testing.T) {
	if !inspectScreen("pods") {
		t.Fatal("pods still draws the generic table")
	}
}

func TestDrawPodsShowsUsageAndFilter(t *testing.T) {
	a := podsApp(func(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
		return map[string]cluster.PodUsage{"shop/web": {CPUMilli: 302, MemoryBytes: 1 << 30}}, true, nil
	})
	tt := ui.NewTester(func(c *ui.Context) { a.drawPods(c, c.Theme()) }, 1600, 500)
	deadline := time.Now().Add(2 * time.Second)
	for !tt.HasText("0.302") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("0.302") || !tt.HasText("1GiB") || !tt.HasText("N/A") {
		t.Fatalf("usage not drawn %v", tt.Texts())
	}
	a.nsFilter = "ops"
	tt.Frame()
	if tt.HasText("web") || !tt.HasText("job") {
		t.Fatalf("namespace filter ignored %v", tt.Texts())
	}
}

func TestPodDrawerShowsEnvironmentAndDirectAdd(t *testing.T) {
	a := podsApp(noUsage)
	controller := true
	pod := a.pods["shop/web"]
	pod.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web-abc", Controller: &controller}}
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP}}
	pod.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "PLAIN", Value: "visible"},
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}}},
		{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "credentials"}, Key: "token",
		}}},
	}
	pod.Spec.InitContainers = []corev1.Container{{
		Name: "setup",
		Env:  []corev1.EnvVar{{Name: "INIT_VAR", Value: "old"}},
	}}
	pod.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
		Prefix: "CFG_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}},
	}}
	a.workloadObjects = map[string]runtime.Object{
		"ReplicaSet/shop/web-abc": &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
			Namespace: "shop", Name: "web-abc",
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "web", Controller: &controller}},
		}},
	}
	a.listKey, a.detailPanelOpen = "shop/web", true
	type envCall struct {
		context   string
		target    cluster.Identity
		container string
		init      bool
		name      string
		value     string
	}
	calls := make(chan envCall, 1)
	a.podEnvMutate = func(_ context.Context, contextName string, target cluster.Identity, container string, init bool, name string, value *string) error {
		call := envCall{context: contextName, target: target, container: container, init: init, name: name}
		if value != nil {
			call.value = *value
		}
		calls <- call
		return nil
	}
	a.forwardLocal, a.forwardRemote = "8080", "80"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.drawPods(c, c.Theme()) })
			a.drawDetailPanel(c, c.Theme())
		})
	}, 1600, 1300)
	for _, want := range []string{
		"Metrics", "Pod parameters", "Port forwards", "Environment", "Ports",
		"App", "Init", "4 sources", "Logs", "Shell", "Start",
		"PLAIN", "visible", "fieldRef(v1:metadata.name)",
		"secretKeyRef(credentials/token)", "envFrom configMap(settings, prefix=CFG_)",
		"Edits Deployment/web · rolls Pods",
	} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if a.forwardLocal != "8080" || a.forwardRemote != "80" {
		t.Fatalf("port values changed local=%q remote=%q", a.forwardLocal, a.forwardRemote)
	}
	for _, absent := range []string{"Operational summary", "Pod facts", "Actions"} {
		if tt.HasText(absent) {
			t.Fatalf("obsolete section %q in %v", absent, tt.Texts())
		}
	}
	for _, action := range []string{"Logs", "Shell", "Start"} {
		if n := countText(tt, action); n != 1 {
			t.Fatalf("%s rendered %d times: %v", action, n, tt.Texts())
		}
	}
	if n := countText(tt, "Exec"); n != 0 {
		t.Fatalf("Exec rendered %d times: %v", n, tt.Texts())
	}
	for _, action := range []string{"Copy name", "Logs", "Shell", "Edit YAML", "Delete", "Close details"} {
		if _, ok := tt.Find(action); !ok {
			t.Fatalf("missing direct Pod header action %q", action)
		}
	}
	if _, ok := tt.Find("Pod actions"); ok {
		t.Fatal("Pod actions overflow menu is still present")
	}
	panel, panelOK := tt.Find("Pod details panel")
	card, cardOK := tt.Find("app container environment, 4 sources")
	metrics, metricsOK := tt.Find("Metrics")
	parameters, parametersOK := tt.Find("Pod parameters")
	portCard, portCardOK := tt.Find("Pod port forwards")
	podFacts, podFactsOK := tt.Find("Pod parameter facts")
	environment, environmentOK := tt.Find("Environment")
	if !panelOK || panel.W != 360 || !cardOK || !metricsOK || !parametersOK || !podFactsOK || !portCardOK || !environmentOK ||
		card.X < panel.X || card.X+card.W > panel.X+panel.W ||
		!(metrics.Y < parameters.Y && parameters.Y < podFacts.Y && podFacts.Y < portCard.Y &&
			portCard.Y < environment.Y && environment.Y < card.Y) {
		t.Fatalf("360px hierarchy/layout panel=%+v metrics=%+v parameters=%+v facts=%+v ports=%+v environment=%+v card=%+v",
			panel, metrics, parameters, podFacts, portCard, environment, card)
	}
	if _, ok := tt.Find("Edit POD_NAME in app"); ok {
		t.Fatal("valueFrom variable exposed plain-value editing")
	}
	tt.SetDark(true)
	tt.SetPreferences(ui.Preferences{TextScale: 1.25})
	tt.Frame()
	panel, panelOK = tt.Find("Pod details panel")
	card, cardOK = tt.Find("app container environment, 4 sources")
	metrics, metricsOK = tt.Find("Metrics")
	parameters, parametersOK = tt.Find("Pod parameters")
	environment, environmentOK = tt.Find("Environment")
	if !panelOK || !cardOK || !metricsOK || !parametersOK || !environmentOK ||
		card.X < panel.X || card.X+card.W > panel.X+panel.W ||
		!(metrics.Y < parameters.Y && parameters.Y < environment.Y) {
		t.Fatalf("dark scaled layout panel=%+v metrics=%+v parameters=%+v environment=%+v card=%+v",
			panel, metrics, parameters, environment, card)
	}
	tt.SetPreferences(ui.Preferences{TextScale: 1})
	if err := tt.Click("Start app port forward"); err != nil {
		t.Fatal(err)
	}
	a.forwardLocal = "invalid"
	tt.Frame()
	if err := tt.Click("Start forwarding app"); err != nil {
		t.Fatal(err)
	}
	if a.errText != "local port must be between 1 and 65535" {
		t.Fatalf("forward submit did not dispatch: %q", a.errText)
	}
	if !tt.HasText("Local port") || tt.HasText("Retry") || tt.HasText("Failed") {
		t.Fatalf("validation error did not keep the editable form: %v", tt.Texts())
	}
	a.errText = ""
	a.forwardLocal = "8080"
	if err := tt.Click("Edit INIT_VAR in setup"); err != nil {
		t.Fatal(err)
	}
	if a.podEnvContainer != "setup" || !a.podEnvInit || a.podEnvEditing != "INIT_VAR" {
		t.Fatalf("init edit bound container=%q init=%v variable=%q", a.podEnvContainer, a.podEnvInit, a.podEnvEditing)
	}
	if _, ok := tt.Find("Save variable"); !ok {
		t.Fatalf("init edit form did not activate: %v", tt.Texts())
	}
	a.podEnvErr = "admission denied"
	tt.Frame()
	if n := countText(tt, "Environment change failed: admission denied"); n != 1 {
		t.Fatalf("mutation error rendered %d times: %v", n, tt.Texts())
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Add environment variable to app"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Environment variable name"); err != nil {
		t.Fatal(err)
	}
	tt.Type("DIRECT")
	if err := tt.Click("Environment variable value"); err != nil {
		t.Fatal(err)
	}
	tt.Type("plain-value")
	if a.podEnvName != "DIRECT" || a.podEnvValue != "plain-value" {
		t.Fatalf("direct-add inputs name=%q value=%q", a.podEnvName, a.podEnvValue)
	}
	if err := tt.Click("Add variable"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-calls:
		if got.context != a.current || got.target.Kind != "Deployment" || got.target.Namespace != "shop" || got.target.Name != "web" ||
			got.container != "app" || got.init || got.name != "DIRECT" || got.value != "plain-value" {
			t.Fatalf("direct add mutator call = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("direct add did not call mutator")
	}
}

func TestPodHeaderDirectActionsDispatch(t *testing.T) {
	a := New(cluster.NewSession())
	edited, deleted := 0, 0
	a.detailPanel = &shellDetailPanel{
		title:       "Pod: web",
		label:       "Pod details panel",
		copyText:    "web",
		editLabel:   "Edit YAML",
		removeLabel: "Delete",
		onEdit:      func() { edited++ },
		onRemove:    func() { deleted++ },
		draw:        func(*ui.Context, *ui.Theme) {},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawDetailPanel(c, c.Theme())
	}, 960, 400)

	for _, action := range []string{"Copy name", "Edit YAML", "Delete", "Close details"} {
		if _, ok := tt.Find(action); !ok {
			t.Fatalf("missing direct header action %q", action)
		}
	}
	if _, ok := tt.Find("Pod actions"); ok {
		t.Fatal("Pod actions overflow menu is still present")
	}
	if err := tt.Click("Copy name"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "web" {
		t.Fatalf("Pod copy wrote %q", got)
	}
	if err := tt.Click("Edit YAML"); err != nil {
		t.Fatal(err)
	}
	if edited != 1 || deleted != 0 {
		t.Fatalf("Edit YAML dispatch edited=%d deleted=%d", edited, deleted)
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	if edited != 1 || deleted != 1 {
		t.Fatalf("Delete dispatch edited=%d deleted=%d", edited, deleted)
	}
}

func TestPodEnvironmentSelectionGuardsInFlightCompletion(t *testing.T) {
	a := podsApp(noUsage)
	a.current = "dev"
	a.podEnvPod = "shop/first"
	started := make(chan struct{})
	release := make(chan struct{})
	calls := make(chan string, 2)
	a.podEnvMutate = func(_ context.Context, contextName string, _ cluster.Identity, _ string, _ bool, name string, _ *string) error {
		if contextName != "dev" {
			t.Errorf("captured context = %q", contextName)
		}
		calls <- name
		if name == "FIRST" {
			close(started)
			<-release
		}
		return nil
	}
	value := "one"
	a.changePodEnvironment(cluster.Identity{Kind: "Deployment", Namespace: "shop", Name: "web"}, "app", false, "FIRST", &value)
	<-started
	a.bindPodEnvironment("shop/second")
	a.podEnvName, a.podEnvValue = "SECOND_DRAFT", "two"
	if !a.podEnvBusy {
		t.Fatal("selection change cleared in-flight busy state")
	}
	a.changePodEnvironment(cluster.Identity{Kind: "Deployment", Namespace: "shop", Name: "other"}, "app", false, "SECOND", &value)
	if got := <-calls; got != "FIRST" {
		t.Fatalf("first mutation = %q", got)
	}
	select {
	case got := <-calls:
		t.Fatalf("overlapping mutation started: %q", got)
	default:
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		a.mu.Lock()
		busy := a.podEnvBusy
		name, draft := a.podEnvName, a.podEnvValue
		a.mu.Unlock()
		if !busy {
			if name != "SECOND_DRAFT" || draft != "two" {
				t.Fatalf("stale completion changed current draft name=%q value=%q", name, draft)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("in-flight busy state did not clear on completion")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestPodEnvironmentContextResetRejectsStaleCompletion(t *testing.T) {
	a := podsApp(noUsage)
	a.current = "dev"
	a.podEnvPod = "shop/web"
	started := make(chan struct{})
	release := make(chan struct{})
	a.podEnvMutate = func(_ context.Context, contextName string, _ cluster.Identity, _ string, _ bool, _ string, _ *string) error {
		if contextName != "dev" {
			t.Errorf("captured context = %q", contextName)
		}
		close(started)
		<-release
		return nil
	}
	value := "one"
	a.changePodEnvironment(cluster.Identity{Kind: "Deployment", Namespace: "shop", Name: "web"}, "app", false, "FIRST", &value)
	<-started
	a.mu.Lock()
	a.resetPodEnvironmentStateLocked()
	a.current = "prod"
	a.podEnvPod, a.podEnvContext = "shop/web", "prod"
	a.podEnvName, a.podEnvValue = "PROD_DRAFT", "two"
	busyAfterReset := a.podEnvBusy
	a.mu.Unlock()
	if !busyAfterReset {
		t.Fatal("context reset cleared in-flight busy state")
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		a.mu.Lock()
		busy := a.podEnvBusy
		name, draft := a.podEnvName, a.podEnvValue
		a.mu.Unlock()
		if !busy {
			if name != "PROD_DRAFT" || draft != "two" {
				t.Fatalf("old-context completion changed new-context draft name=%q value=%q", name, draft)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("old-context busy state did not clear on completion")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStandalonePodEnvironmentIsTruthfullyReadOnly(t *testing.T) {
	a := podsApp(noUsage)
	a.pods["shop/web"].Spec.Containers[0].Env = []corev1.EnvVar{{Name: "PLAIN", Value: "visible"}}
	a.listKey, a.detailPanelOpen = "shop/web", true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.drawPods(c, c.Theme()) })
			a.drawDetailPanel(c, c.Theme())
		})
	}, 1600, 900)
	if !tt.HasText("Live Pod environment is immutable") {
		t.Fatalf("missing immutable state: %v", tt.Texts())
	}
	if _, ok := tt.Find("Add environment variable to app"); ok {
		t.Fatal("standalone Pod exposed environment mutation")
	}
}

func TestPodLinkOpensNodeWithRowChosen(t *testing.T) {
	a := podsApp(noUsage)
	a.listQuery = "web"
	tt := ui.NewTester(func(c *ui.Context) {
		if a.tabs.Screen(a.active) == "pods" {
			a.drawPods(c, c.Theme())
		}
	}, 1600, 500)
	if err := tt.Click("Open node-a"); err != nil {
		t.Fatal(err)
	}
	if got := a.tabs.Screen(a.active); got != "nodes" {
		t.Fatalf("active screen %q", got)
	}
	if a.listKey != "node-a" || a.row != -1 || a.listQuery != "" || !a.listReveal || a.detailPanel != nil {
		t.Fatalf("nodes tab key %q row %d query %q reveal %v", a.listKey, a.row, a.listQuery, a.listReveal)
	}
	pods, _ := a.tabs.byScreen("pods")
	if b := a.bodies[pods]; b == nil || b.listKey == "node-a" || b.listQuery != "web" {
		t.Fatalf("pods tab state was overwritten: %+v", b)
	}
}

func TestOpenWorkloadDropsHidingNamespace(t *testing.T) {
	a := podsApp(noUsage)
	a.showScreen("workloads", false)
	a.nsFilter = "ops"
	a.showScreen("pods", true)
	a.openListAt("workloads", "replicasets/shop/web-1")
	if a.tabs.Screen(a.active) != "workloads" || a.nsFilter != "" || a.listKey != "replicasets/shop/web-1" {
		t.Fatalf("screen %q ns %q key %q", a.tabs.Screen(a.active), a.nsFilter, a.listKey)
	}
}

func TestBindListRevealsLinkedRow(t *testing.T) {
	a := podsApp(noUsage)
	a.listKey, a.row, a.listReveal, a.detailPanelOpen = "b", -1, true, false
	a.bindList([]string{"a", "b"})
	if a.row != 1 || a.listReveal || !a.detailPanelOpen {
		t.Fatalf("row %d reveal %v panel %v", a.row, a.listReveal, a.detailPanelOpen)
	}
}

func TestPodUsageFetchesOnceAndDropsStale(t *testing.T) {
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32
	a := podsApp(func(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
		calls.Add(1)
		<-release
		defer close(done)
		return map[string]cluster.PodUsage{"shop/web": {CPUMilli: 1}}, true, nil
	})
	a.mu.Lock()
	a.ensurePodUsageLocked(time.Now(), nil)
	a.ensurePodUsageLocked(time.Now(), nil)
	a.forgetPodUsageLocked()
	a.mu.Unlock()
	close(release)
	<-done
	// The write-back follows under the lock the fetch takes next.
	time.Sleep(10 * time.Millisecond)
	a.mu.Lock()
	defer a.mu.Unlock()
	if calls.Load() != 1 {
		t.Fatalf("fetched %d times", calls.Load())
	}
	if a.podUsage != nil {
		t.Fatal("a read from before the switch landed")
	}
}

func TestReadyDots(t *testing.T) {
	if got := readyDots(nil); got != "0/0" {
		t.Fatalf("got %s", got)
	}
}

func TestPodHeaderTargetsContainersAndFitsLongTitle(t *testing.T) {
	a := New(cluster.NewSession())
	type call struct {
		action    string
		container string
	}
	var calls []call
	const podName = "web-api-with-a-very-long-generated-suffix-7b9d8f6c4d-x2k9q"
	const title = "Pod: " + podName
	containers := []string{"application-container-with-a-long-name", "sidecar"}
	a.detailPanel = &shellDetailPanel{
		title:       title,
		label:       "Pod details panel",
		copyText:    podName,
		editLabel:   "Edit YAML",
		removeLabel: "Delete",
		onEdit:      func() {},
		onRemove:    func() {},
		drawHeaderActions: func(c *ui.Context, theme *ui.Theme) {
			a.drawPodHeaderAction(c, theme, "Logs", "list", containers, func(container string) {
				calls = append(calls, call{"logs", container})
			})
			a.drawPodHeaderAction(c, theme, "Shell", "terminal", containers, func(container string) {
				calls = append(calls, call{"shell", container})
			})
		},
		draw: func(c *ui.Context, theme *ui.Theme) {
			a.drawPodPortForwards(c, theme, &corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: containers[0]}, {Name: containers[1]}},
			}}, listRow{Namespace: "shop", Name: "web"})
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawDetailPanel(c, c.Theme())
	}, 960, 700)
	tt.SetPreferences(ui.Preferences{TextScale: 1.25})
	tt.Frame()

	header, headerOK := tt.Find("Detail panel header")
	titleRect, titleOK := tt.Find(title)
	copyRect, copyOK := tt.Find("Copy name")
	if !headerOK || !titleOK || !copyOK || titleRect.X < header.X || titleRect.X+titleRect.W > copyRect.X {
		t.Fatalf("long title crowds adjacent copy control: header=%+v title=%+v copy=%+v", header, titleRect, copyRect)
	}
	previousRight := titleRect.X + titleRect.W
	for _, label := range []string{"Copy name", "Logs", "Shell", "Edit YAML", "Delete", "Close details"} {
		control, ok := tt.Find(label)
		if !headerOK || !ok || control.X < header.X || control.X+control.W > header.X+header.W ||
			control.Y < header.Y || control.Y+control.H > header.Y+header.H || control.X < previousRight {
			t.Fatalf("%s crowded or out of order in 360px header: header=%+v control=%+v found=%v", label, header, control, ok)
		}
		previousRight = control.X + control.W
	}
	if _, ok := tt.Find("Pod actions"); ok {
		t.Fatal("Pod actions overflow menu is still present")
	}
	if n := countText(tt, "Logs"); n != 1 {
		t.Fatalf("Logs appears outside the header: %v", tt.Texts())
	}
	if n := countText(tt, "Shell"); n != 1 {
		t.Fatalf("Shell appears outside the header: %v", tt.Texts())
	}
	if !tt.HasText("Port forwards") {
		t.Fatalf("port-forward body missing: %v", tt.Texts())
	}
	if err := tt.Click("Logs"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Logs: application-container-with-a-long-name") || !tt.HasText("Logs: sidecar") {
		t.Fatalf("logs menu = %v", tt.Texts())
	}
	if err := tt.Click("Logs: sidecar"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Shell"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Shell: application-container-with-a-long-name") || !tt.HasText("Shell: sidecar") {
		t.Fatalf("shell menu = %v", tt.Texts())
	}
	if err := tt.Click("Shell: application-container-with-a-long-name"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != (call{"logs", "sidecar"}) ||
		calls[1] != (call{"shell", "application-container-with-a-long-name"}) {
		t.Fatalf("header calls = %+v", calls)
	}
}

func TestPodHeaderSingleContainerDispatchesDirectly(t *testing.T) {
	a := New(cluster.NewSession())
	var logs, shell string
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawPodHeaderAction(c, c.Theme(), "Logs", "list", []string{"app"}, func(container string) {
			logs = container
		})
		a.drawPodHeaderAction(c, c.Theme(), "Shell", "terminal", []string{"app"}, func(container string) {
			shell = container
		})
	}, 360, 100)
	if err := tt.Click("Logs"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Shell"); err != nil {
		t.Fatal(err)
	}
	if logs != "app" || shell != "app" {
		t.Fatalf("single-container targets logs=%q shell=%q", logs, shell)
	}
}

func TestPodPortForwardStartingFailureRetryAndActiveOpenAndStop(t *testing.T) {
	a := New(cluster.NewSession())
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name:  "app",
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
	}}}}
	row := listRow{Namespace: "shop", Name: "web"}
	started := make(chan struct{})
	release := make(chan struct{})
	a.portForwardRun = func(context.Context, string, string, string, uint16, uint16) (*cluster.Forward, error) {
		close(started)
		<-release
		return nil, errors.New("permission denied")
	}
	opened := make(chan string, 1)
	a.openExternal = func(url string) error {
		opened <- url
		return nil
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawPodPortForwards(c, c.Theme(), pod, row)
	}, 360, 700)

	if err := tt.Click("Start app port forward"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Local port") || !tt.HasText("Remote 8080") {
		t.Fatalf("start form = %v", tt.Texts())
	}
	if a.forwardLocal != "8080" || a.forwardRemote != "8080" {
		t.Fatalf("smart defaults local=%q remote=%q", a.forwardLocal, a.forwardRemote)
	}
	if err := tt.Click("Start forwarding app"); err != nil {
		t.Fatal(err)
	}
	<-started
	tt.Frame()
	if !tt.HasText("Starting") || !tt.HasText("127.0.0.1:8080") {
		t.Fatalf("starting state = %v", tt.Texts())
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for !tt.HasText("Failed") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("Failed") || !tt.HasText("permission denied") || !tt.HasText("Retry") {
		t.Fatalf("failed state = %v", tt.Texts())
	}
	retryStarted := make(chan struct{})
	retryRelease := make(chan struct{})
	a.portForwardRun = func(context.Context, string, string, string, uint16, uint16) (*cluster.Forward, error) {
		close(retryStarted)
		<-retryRelease
		return nil, errors.New("permission denied again")
	}
	if err := tt.Click("Retry app port forward"); err != nil {
		t.Fatal(err)
	}
	<-retryStarted
	tt.Frame()
	if !tt.HasText("Starting") {
		t.Fatalf("retry did not start directly: %v", tt.Texts())
	}
	close(retryRelease)
	deadline = time.Now().Add(time.Second)
	for !tt.HasText("permission denied again") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		tt.Frame()
	}

	key := a.podForwardKey(row, 8080)
	fw := &cluster.Forward{Context: a.session.Current(), Namespace: "shop", Pod: "web", Local: 8080, Remote: 8080}
	a.mu.Lock()
	delete(a.forwardFailed, key)
	a.forwards = []*cluster.Forward{fw}
	a.forwardKeys[fw] = key
	a.mu.Unlock()
	tt.Frame()
	if !tt.HasText("http://127.0.0.1:8080") || !tt.HasText("Active") || !tt.HasText("Stop") {
		t.Fatalf("active state = %v", tt.Texts())
	}
	activeRow, activeRowOK := tt.Find("app active destination")
	const openLabel = "Open http://127.0.0.1:8080 in browser"
	openAction, openOK := tt.Find(openLabel)
	stopAction, stopOK := tt.Find("Stop app port forward")
	if !activeRowOK || !openOK || !stopOK || openAction.Y != stopAction.Y ||
		openAction.X+openAction.W > stopAction.X || openAction.X < activeRow.X ||
		stopAction.X+stopAction.W > activeRow.X+activeRow.W {
		t.Fatalf("active actions misaligned row=%+v open=%+v stop=%+v", activeRow, openAction, stopAction)
	}
	if err := tt.Click(openLabel); err != nil {
		t.Fatal(err)
	}
	select {
	case url := <-opened:
		if url != "http://127.0.0.1:8080" {
			t.Fatalf("opened URL %q", url)
		}
	case <-time.After(time.Second):
		t.Fatal("Open did not dispatch")
	}
	a.openExternal = func(string) error { return errors.New("browser unavailable") }
	if err := tt.Click(openLabel); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		openErr := a.errText
		a.mu.Unlock()
		if openErr == "open port forward: browser unavailable" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	a.mu.Lock()
	openErr := a.errText
	a.mu.Unlock()
	if openErr != "open port forward: browser unavailable" {
		t.Fatalf("open error = %q", openErr)
	}
	if err := tt.Click("Stop app port forward"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Stopping") {
		t.Fatalf("stopping state = %v", tt.Texts())
	}
}

func TestPodWithoutDeclaredPortsOffersCustomForm(t *testing.T) {
	a := New(cluster.NewSession())
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawPodPortForwards(c, c.Theme(), pod, listRow{Namespace: "shop", Name: "web"})
	}, 360, 500)
	if !tt.HasText("Custom port") {
		t.Fatalf("custom affordance missing: %v", tt.Texts())
	}
	if err := tt.Click("Start Custom port port forward"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Local port") || !tt.HasText("Remote port") {
		t.Fatalf("custom form = %v", tt.Texts())
	}
}

func TestPodPortsDedupeTCPAndDisableUnsupportedProtocols(t *testing.T) {
	a := New(cluster.NewSession())
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "app", Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{Name: "dns", ContainerPort: 53, Protocol: corev1.ProtocolUDP},
		}},
		{Name: "sidecar", Ports: []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{Name: "stream", ContainerPort: 9000, Protocol: corev1.ProtocolSCTP},
		}},
	}}}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawPodPortForwards(c, c.Theme(), pod, listRow{Namespace: "shop", Name: "web"})
	}, 360, 700)

	if !tt.HasText("app, sidecar") || !tt.HasText("http, metrics · 8080/TCP") {
		t.Fatalf("deduped TCP row missing: %v", tt.Texts())
	}
	if n := countText(tt, "Start"); n != 1 {
		t.Fatalf("deduped TCP port rendered %d Start actions: %v", n, tt.Texts())
	}
	if n := countText(tt, "Unsupported"); n != 2 {
		t.Fatalf("unsupported protocol badges = %d: %v", n, tt.Texts())
	}
	if _, ok := tt.Find("Start app port forward"); ok {
		t.Fatal("UDP row exposed Start")
	}
	if _, ok := tt.Find("Start sidecar port forward"); ok {
		t.Fatal("SCTP row exposed Start")
	}
}

func TestPodFactsPrecedePortForwardsAt360Width(t *testing.T) {
	a := New(cluster.NewSession())
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawPodParameters(c, c.Theme(), podview.Row{Status: "Running"}, listRow{Namespace: "shop", Name: "web"}, pod)
	}, 360, 720)
	portCard, portOK := tt.Find("Pod port forwards")
	podFacts, factsOK := tt.Find("Pod parameter facts")
	if !portOK || !factsOK || portCard.X < 0 || portCard.X+portCard.W > 360 || podFacts.Y >= portCard.Y {
		t.Fatalf("facts/ports order at 360px facts=%+v ports=%+v texts=%v", podFacts, portCard, tt.Texts())
	}
}
