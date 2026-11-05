package desktop

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
)

type podRow struct {
	Namespace string
	Name      string
	Phase     string
	Ready     string
}

func (a *App) drawSelected(c *ui.Context, t *ui.Theme, selected string, pods []podRow) {
	if a.mode == "logs" {
		a.drawLogs(c, t)
		return
	}
	if a.mode == "exec" {
		a.drawExec(c, t)
		return
	}
	if a.mode == "yaml" {
		a.drawYAML(c, t)
		return
	}
	switch {
	case selected == "overview" || selected == "":
		a.cards(c, t)
	case selected == "workloads" || isWorkload(selected):
		a.drawWorkloads(c, t, selected, pods)
	case selected == "helm":
		a.drawHelm(c, t)
	case strings.HasPrefix(selected, "argo-"):
		a.drawArgo(c, t, selected)
	case strings.HasPrefix(selected, "istio-"):
		a.drawIstio(c, t, selected)
	case selected == "port-forwards":
		a.drawForwards(c, t)
	default:
		if kind, ok := LookupKind(selected); ok {
			a.drawGeneric(c, t, kind)
			return
		}
		ui.Text(c, labelFor(selected)).FontSize(22).Bold()
		ui.Text(c, "No objects yet").TextColor(t.TextMuted)
	}
}

func isWorkload(id string) bool {
	switch id {
	case "pods", "deployments", "replicasets", "daemonsets", "statefulsets", "jobs", "cronjobs":
		return true
	default:
		return false
	}
}

func (a *App) drawWorkloads(c *ui.Context, t *ui.Theme, selected string, pods []podRow) {
	ui.Text(c, labelFor(selected)).FontSize(22).Bold()
	if len(pods) == 0 {
		ui.Text(c, "No objects yet").TextColor(t.TextMuted)
		return
	}
	rows := pods
	if a.row < 0 || a.row >= len(rows) {
		a.row = 0
	}
	ui.Table(c, &ui.ListState{Selected: &a.row}, []ui.TableColumn{
		{Title: "Namespace", Width: 140},
		{Title: "Name", Width: 180},
		{Title: "Ready", Width: 80},
		{Title: "Phase", Width: 100},
	}, len(rows), func(row, col int) {
		item := rows[row]
		text := []string{item.Namespace, item.Name, item.Ready, item.Phase}[col]
		ui.Text(c, text)
	}).FillWidth().Height(280)
	item := rows[a.row]
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Logs").Clicked() {
			a.openLogs(item.Namespace, item.Name, "")
		}
		if ui.Button(c, "Exec").Clicked() {
			a.openExec(item.Namespace, item.Name, "")
		}
		if ui.Button(c, "YAML").Clicked() {
			a.openYAML(cluster.Identity{Kind: "Pod", Namespace: item.Namespace, Name: item.Name, GVR: schema.GroupVersionResource{Version: "v1", Resource: "pods"}})
		}
		if ui.Button(c, "Delete").Clicked() {
			a.OpenConfirm("Pod", item.Namespace, item.Name, "delete")
		}
		if selected != "pods" && ui.Button(c, "Restart").Clicked() {
			a.OpenConfirm(kindFromID(selected), item.Namespace, item.Name, "restart")
		}
	})
}

func (a *App) drawLogs(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() {
			a.mode = ""
		}
		ui.Text(c, "Logs").FontSize(22).Bold()
		ui.TextInput(c, &a.logFilter).Placeholder("regex").Width(180)
		label := "Follow on"
		if !a.logFollow {
			label = "Follow off"
		}
		if ui.Button(c, label).Clicked() {
			a.logFollow = !a.logFollow
		}
	})
	shown, err := FilterLines(a.logLines, a.logFilter)
	if err != nil {
		ui.Text(c, "Invalid regex").TextColor(t.Danger)
		return
	}
	if len(shown) == 0 {
		ui.Text(c, "No log lines").TextColor(t.TextMuted)
		return
	}
	ui.Table(c, &ui.ListState{FollowEnd: a.logFollow}, []ui.TableColumn{
		{Title: "Pod", Width: 140},
		{Title: "Line"},
	}, len(shown), func(row, col int) {
		line := shown[row]
		if col == 0 {
			ui.Text(c, line.Pod).TextColor(PodColor(line.Pod))
			return
		}
		ui.Text(c, line.Text).TextColor(PodColor(line.Pod))
	}).Fill()
}

func (a *App) openLogs(namespace, name, container string) {
	a.mode = "logs"
	a.logLines = nil
	a.logFollow = true
	go func() {
		_ = a.session.FanoutLogs(context.Background(), namespace, "app="+name, true, 200, func(lines []cluster.LogLine) {
			a.mu.Lock()
			for _, line := range lines {
				a.logLines = append(a.logLines, LogLine{Pod: line.Pod, Text: line.Text})
			}
			a.mu.Unlock()
			a.invalidate()
		})
	}()
	_ = container
}

func (a *App) drawExec(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() {
			a.closeExec()
		}
		ui.Text(c, "Exec").FontSize(22).Bold()
	})
	if a.execErr != "" {
		ui.Text(c, a.execErr).TextColor(t.Danger)
		return
	}
	if a.execTerm == nil {
		ui.Text(c, "Connecting").TextColor(t.TextMuted)
		return
	}
	terminal.View(c, a.execTerm).Fill()
}

func (a *App) openExec(namespace, name, container string) {
	a.mode = "exec"
	a.execErr = ""
	go func() {
		conn, err := a.session.Exec(context.Background(), namespace, name, container, []string{"/bin/sh"})
		if err != nil {
			a.mu.Lock()
			a.execErr = err.Error()
			a.mu.Unlock()
			a.invalidate()
			return
		}
		term, err := terminal.New(terminal.Options{Conn: conn})
		if err != nil {
			_ = conn.Close()
			a.mu.Lock()
			a.execErr = err.Error()
			a.mu.Unlock()
			a.invalidate()
			return
		}
		a.mu.Lock()
		a.execConn = conn
		a.execTerm = term
		a.mu.Unlock()
		a.invalidate()
	}()
}

func (a *App) closeExec() {
	if a.execConn != nil {
		_ = a.execConn.Close()
		a.execConn = nil
	}
	if a.execTerm != nil {
		a.execTerm.Close()
		a.execTerm = nil
	}
	a.mode = ""
}

func (a *App) drawYAML(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() {
			a.mode = ""
			a.yaml = nil
		}
		ui.Text(c, "YAML").FontSize(22).Bold()
		if ui.Button(c, "Dry-run").Clicked() && a.yaml != nil {
			a.dryRunYAML()
		}
	})
	if a.yaml == nil || a.yaml.State == nil {
		ui.Text(c, "Loading").TextColor(t.TextMuted)
		return
	}
	if a.yaml.Diff != nil {
		editor.DiffView(c, a.yaml.Diff).Fill()
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "Cancel").Clicked() {
				_ = ConfirmYAML(a.yaml.Confirm, false, a.applyYAML)
				a.yaml.Diff = nil
			}
			if ui.Button(c, "Confirm").Clicked() {
				body := a.yaml.State.Buffer.String()
				dialog := a.yaml.Confirm
				dialog.Body = body
				_ = ConfirmYAML(dialog, true, a.applyYAML)
				a.mode = ""
			}
		})
		return
	}
	editor.View(c, a.yaml.State).Fill()
}

func (a *App) openYAML(id cluster.Identity) {
	a.mode = "yaml"
	a.yaml = &yamlScreen{Identity: id}
	go func() {
		body, err := a.session.GetYAML(context.Background(), id)
		if err != nil {
			body = fmt.Sprintf("# %s\n", err.Error())
		}
		if id.Kind == "Secret" && !a.secretReveal {
			body = maskSecretYAML(body)
		}
		buf := editor.NewBuffer(body)
		buf.SetLanguage(editor.LanguageFor("resource.yaml"))
		a.mu.Lock()
		if a.yaml != nil {
			a.yaml.State = &editor.State{Buffer: buf}
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

func (a *App) dryRunYAML() {
	if a.yaml == nil || a.yaml.State == nil || a.yaml.State.Buffer == nil {
		return
	}
	body := a.yaml.State.Buffer.String()
	go func() {
		preview, err := a.session.DryRunApply(context.Background(), body)
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.yaml == nil {
			return
		}
		if err != nil {
			a.yaml.Err = err.Error()
			return
		}
		a.yaml.Confirm = OpenYAML(preview, body)
		old, newBuf := editor.NewBuffer(preview.Live), editor.NewBuffer(preview.Predicted)
		lang := editor.LanguageFor("resource.yaml")
		old.SetLanguage(lang)
		newBuf.SetLanguage(lang)
		a.yaml.Diff = &editor.DiffState{Old: old, New: newBuf}
		a.invalidate()
	}()
}

func (a *App) drawGeneric(c *ui.Context, t *ui.Theme, kind Kind) {
	ui.Text(c, kind.ID).FontSize(22).Bold()
	rows := a.genericRows
	if len(rows) == 0 {
		ui.Text(c, "No objects yet").TextColor(t.TextMuted)
	} else {
		if a.row < 0 || a.row >= len(rows) {
			a.row = 0
		}
		ui.Table(c, &ui.ListState{Selected: &a.row}, []ui.TableColumn{
			{Title: "Namespace", Width: 160},
			{Title: "Name", Width: 220},
			{Title: "Age", Width: 100},
		}, len(rows), func(row, col int) {
			item := rows[row]
			switch col {
			case 0:
				ui.Text(c, item.Namespace)
			case 1:
				ui.Text(c, item.Name)
			default:
				ui.Text(c, item.Age.Truncate(time.Second).String())
			}
		}).FillWidth().Height(320)
		item := rows[a.row]
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "YAML").Clicked() {
				a.openYAML(cluster.Identity{GVR: kind.GVR, Kind: kind.ID, Namespace: item.Namespace, Name: item.Name})
			}
			if ui.Button(c, "Delete").Clicked() {
				a.OpenConfirm(kind.ID, item.Namespace, item.Name, "delete")
			}
			if kind.ID == "secrets" && ui.Button(c, "Reveal").Clicked() {
				a.secretReveal = !a.secretReveal
			}
		})
	}
}

func (a *App) drawForwards(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "Port-forwards").FontSize(22).Bold()
	if len(a.forwards) == 0 {
		ui.Text(c, "No forwards in this process").TextColor(t.TextMuted)
		return
	}
	for _, fw := range a.forwards {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("%s/%s %d:%d", fw.Namespace, fw.Pod, fw.Local, fw.Remote))
			if ui.Button(c, "Stop").Clicked() {
				fw.Stop()
			}
		})
	}
}

func (a *App) drawHelm(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "Helm").FontSize(22).Bold()
	ui.TextInput(c, &a.helmRelease).Placeholder("release").Label("Release")
	ui.TextInput(c, &a.helmChart).Placeholder("chart").Label("Chart")
	ui.TextInput(c, &a.helmNamespace).Placeholder("namespace").Label("Namespace")
	if a.helmValues == nil {
		buf := editor.NewBuffer("")
		buf.SetLanguage(editor.LanguageFor("values.yaml"))
		a.helmValues = &editor.State{Buffer: buf}
	}
	editor.View(c, a.helmValues).Height(160)
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Dry-run").Clicked() {
			a.helmDry = true
			a.runHelm(true)
		}
		if ui.Button(c, "Cancel").Clicked() {
			a.helmDry = false
			a.helmDiff = nil
		}
		if ui.Button(c, "Install").Clicked() && a.helmDry {
			a.runHelm(false)
			a.helmDry = false
		}
	})
	if a.helmDiff != nil {
		editor.DiffView(c, a.helmDiff).Height(220)
	}
	if a.helmErr != "" {
		ui.Text(c, a.helmErr).TextColor(t.Danger)
	}
}

func (a *App) runHelm(dry bool) {
	req := cluster.HelmRequest{
		Release:   a.helmRelease,
		Chart:     a.helmChart,
		Namespace: a.helmNamespace,
		Values:    "",
		DryRun:    dry,
	}
	if a.helmValues != nil && a.helmValues.Buffer != nil {
		req.Values = a.helmValues.Buffer.String()
	}
	if !dry && !a.helmDry {
		return
	}
	go func() {
		res, err := a.session.HelmInstall(context.Background(), req)
		a.mu.Lock()
		defer a.mu.Unlock()
		if err != nil {
			a.helmErr = err.Error()
			a.invalidate()
			return
		}
		if dry && res != nil {
			old, newBuf := editor.NewBuffer(""), editor.NewBuffer(res.Manifest)
			a.helmDiff = &editor.DiffState{Old: old, New: newBuf}
		}
		a.invalidate()
	}()
}

func (a *App) drawArgo(c *ui.Context, t *ui.Theme, selected string) {
	ui.Text(c, labelFor(selected)).FontSize(22).Bold()
	if a.crdMissing {
		ui.Text(c, "CRD is not installed").TextColor(t.TextMuted)
		return
	}
	if len(a.argoApps) == 0 {
		ui.Text(c, "No objects yet").TextColor(t.TextMuted)
		return
	}
	for _, app := range a.argoApps {
		ui.Text(c, fmt.Sprintf("%s/%s %s %s", app.Namespace, app.Name, app.Sync, app.Health))
	}
}

func (a *App) drawIstio(c *ui.Context, t *ui.Theme, selected string) {
	ui.Text(c, labelFor(selected)).FontSize(22).Bold()
	if a.crdMissing {
		ui.Text(c, "CRD is not installed").TextColor(t.TextMuted)
		return
	}
	if len(a.istioRules) == 0 {
		ui.Text(c, "No objects yet").TextColor(t.TextMuted)
		return
	}
	for _, rule := range a.istioRules {
		ui.Text(c, rule.Match)
		for _, dest := range rule.Destinations {
			ui.Text(c, fmt.Sprintf("%s %s %d", dest.Host, dest.Subset, dest.Weight)).TextColor(t.TextMuted)
		}
	}
}

func podRows(pods map[string]*podSnap) []podRow {
	out := make([]podRow, 0, len(pods))
	for _, pod := range pods {
		out = append(out, podRow{Namespace: pod.Namespace, Name: pod.Name, Phase: pod.Phase, Ready: pod.Ready})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

type podSnap struct {
	Namespace string
	Name      string
	Phase     string
	Ready     string
}

func maskSecretYAML(body string) string {
	var b strings.Builder
	mask := false
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "data:" || trim == "stringData:" {
			mask = true
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if mask && strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
			key, _, _ := strings.Cut(line, ":")
			b.WriteString(key)
			b.WriteString(": \"***\"\n")
			continue
		}
		if mask && !strings.HasPrefix(line, " ") && trim != "" {
			mask = false
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func kindFromID(id string) string {
	switch id {
	case "deployments":
		return "Deployment"
	case "replicasets":
		return "ReplicaSet"
	case "daemonsets":
		return "DaemonSet"
	case "statefulsets":
		return "StatefulSet"
	case "jobs":
		return "Job"
	case "cronjobs":
		return "CronJob"
	default:
		return "Pod"
	}
}

type yamlScreen struct {
	Identity cluster.Identity
	State    *editor.State
	Diff     *editor.DiffState
	Confirm  YAMLConfirm
	Err      string
}

func (a *App) applyYAML(id cluster.Identity, body string) error {
	return a.session.Apply(context.Background(), id, body)
}

// Keep io import used by exec close through the conn interface.
var _ io.Closer = (*cluster.ExecConn)(nil)
