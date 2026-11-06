package desktop

import (
	"context"
	"io"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/helm"
	"github.com/dacsang97/nakube/internal/views/logs"
	"github.com/dacsang97/nakube/internal/views/manifest"
	"github.com/dacsang97/nakube/internal/views/term"
)

func (a *App) drawLogs(c *ui.Context, t *ui.Theme) {
	shown, err := FilterLines(a.logLines, a.logFilter)
	lines := make([]logs.Line, len(shown))
	for i, line := range shown {
		lines[i] = logs.Line{Pod: line.Pod, Text: line.Text, Color: PodColor(line.Pod)}
	}
	logs.Draw(c, t, logs.Page{
		Pod:     a.logPod,
		Filter:  &a.logFilter,
		Follow:  &a.logFollow,
		Invalid: err != nil,
		Loading: a.logLoading,
		Lines:   lines,
		OnBack:  a.backFromOverlay,
	})
}

func (a *App) openLogs(namespace, name, container string) {
	a.openDetail("logs", "Pod", namespace, name, "Logs: "+name)
	a.ensureTabs()
	owner := a.active
	if a.logCancel != nil {
		a.logCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	b := a.bodies[owner]
	if b == nil {
		b = newTabBody()
		a.bodies[owner] = b
	}
	b.logCancel = cancel
	b.logGen++
	gen := b.logGen
	b.mode = "logs"
	b.logLines = nil
	b.logLoading = true
	b.logFollow = true
	b.logPod = name
	if a.active == owner {
		a.logCancel = cancel
		a.logGen = gen
		a.mode = "logs"
		a.logLines = nil
		a.logLoading = true
		a.logFollow = true
		a.logPod = name
	}
	a.mu.Unlock()
	go func() {
		err := a.session.StreamLogs(ctx, namespace, name, container, true, 200, func(lines []string) {
			a.mu.Lock()
			b := a.bodies[owner]
			if b == nil || b.logGen != gen {
				a.mu.Unlock()
				return
			}
			b.logLoading = false
			for _, line := range lines {
				b.logLines = append(b.logLines, LogLine{Pod: name, Text: line})
			}
			if a.active == owner {
				a.logLines = b.logLines
				a.logLoading = false
			}
			a.mu.Unlock()
			a.invalidate()
		})
		a.mu.Lock()
		if b := a.bodies[owner]; b != nil && b.logGen == gen {
			b.logLoading = false
			if a.active == owner {
				a.logLoading = false
			}
		}
		a.mu.Unlock()
		a.invalidate()
		if err != nil && ctx.Err() == nil {
			a.setErr(publicError(err))
		}
	}()
}

func (a *App) drawExec(c *ui.Context, t *ui.Theme) {
	term.Draw(c, t, term.Page{Err: a.execErr, Term: a.execTerm, OnBack: a.backFromExec})
}

func (a *App) openExec(namespace, name, container string) {
	a.openDetail("exec", "Pod", namespace, name, "Shell: "+name)
	a.closeExec()
	a.ensureTabs()
	owner := a.active
	a.mu.Lock()
	b := a.bodies[owner]
	if b == nil {
		b = newTabBody()
		a.bodies[owner] = b
	}
	b.mode = "exec"
	b.execErr = ""
	if a.active == owner {
		a.mode = "exec"
		a.execErr = ""
	}
	a.mu.Unlock()
	go func() {
		conn, err := a.session.Exec(context.Background(), namespace, name, container, []string{"/bin/sh"})
		if err != nil {
			a.noteExecErr(owner, publicError(err))
			return
		}
		term, err := terminal.New(terminal.Options{Conn: conn})
		if err != nil {
			_ = conn.Close()
			a.noteExecErr(owner, publicError(err))
			return
		}
		a.mu.Lock()
		b := a.bodies[owner]
		if b == nil || b.mode != "exec" {
			a.mu.Unlock()
			term.Close()
			_ = conn.Close()
			return
		}
		b.execConn = conn
		b.execTerm = term
		if a.active == owner {
			a.execConn = conn
			a.execTerm = term
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

func (a *App) noteExecErr(owner, msg string) {
	a.mu.Lock()
	if b := a.bodies[owner]; b != nil {
		b.execErr = msg
	}
	if a.active == owner {
		a.execErr = msg
	}
	a.mu.Unlock()
	a.invalidate()
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
	if a.mode == "exec" {
		a.mode = ""
	}
	a.stash()
}

func (a *App) drawYAML(c *ui.Context, t *ui.Theme) {
	page := manifest.Page{OnBack: a.backFromYAML, OnDryRun: a.dryRunYAML}
	if a.yaml != nil {
		page.Err = a.yaml.Err
		page.Kind = a.yaml.Identity.Kind
		page.State = a.yaml.State
		page.Diff = a.yaml.Diff
		page.Pending = a.yaml.Pending
		page.OnReveal = func() {
			a.secretReveal = !a.secretReveal
			a.openYAML(a.yaml.Identity)
		}
		page.OnCancel = func() {
			_ = ConfirmYAML(a.yaml.Confirm, false, a.applyYAML)
			a.yaml.Diff = nil
		}
		page.OnConfirm = func() {
			if a.yaml.State == nil || a.yaml.State.Buffer == nil {
				return
			}
			dialog := a.yaml.Confirm
			dialog.Body = a.yaml.State.Buffer.String()
			_ = ConfirmYAML(dialog, true, a.applyYAML)
			a.mode = ""
		}
	}
	manifest.Draw(c, t, page)
}

func (a *App) openYAML(id cluster.Identity) {
	title := id.Name + ".yaml"
	if id.Name == "" {
		title = "YAML"
	}
	a.openDetail("yaml", id.Kind, id.Namespace, id.Name, title)
	a.ensureTabs()
	owner := a.active
	screen := &yamlScreen{Identity: id}
	a.mode = "yaml"
	a.yaml = screen
	a.stash()
	reveal := a.secretReveal
	go func() {
		body, err := a.session.GetYAML(context.Background(), id)
		a.mu.Lock()
		defer a.mu.Unlock()
		b := a.bodies[owner]
		if b == nil || b.yaml != screen {
			return
		}
		if err != nil {
			screen.Err = publicError(err)
			a.invalidate()
			return
		}
		if id.Kind == "Secret" && !reveal {
			body = maskSecretYAML(body)
		}
		buf := editor.NewBuffer(body)
		buf.SetLanguage(editor.LanguageFor("resource.yaml"))
		screen.State = &editor.State{Buffer: buf}
		screen.Err = ""
		a.invalidate()
	}()
}

func (a *App) dryRunYAML() {
	if a.yaml == nil || a.yaml.State == nil || a.yaml.State.Buffer == nil {
		return
	}
	owner := a.active
	screen := a.yaml
	body := screen.State.Buffer.String()
	screen.Pending = true
	screen.Diff = nil
	a.invalidate()
	go func() {
		preview, err := a.session.DryRunApply(context.Background(), body)
		a.mu.Lock()
		defer a.mu.Unlock()
		b := a.bodies[owner]
		if b == nil || b.yaml != screen {
			return
		}
		screen.Pending = false
		if err != nil {
			screen.Err = publicError(err)
			a.invalidate()
			return
		}
		screen.Confirm = OpenYAML(preview, body)
		old, newBuf := editor.NewBuffer(preview.Live), editor.NewBuffer(preview.Predicted)
		lang := editor.LanguageFor("resource.yaml")
		old.SetLanguage(lang)
		newBuf.SetLanguage(lang)
		screen.Diff = &editor.DiffState{Old: old, New: newBuf}
		screen.Err = ""
		a.invalidate()
	}()
}

func (a *App) drawHelm(c *ui.Context, t *ui.Theme) {
	if a.helmValues == nil {
		buf := editor.NewBuffer("")
		buf.SetLanguage(editor.LanguageFor("values.yaml"))
		a.helmValues = &editor.State{Buffer: buf}
	}
	helm.Draw(c, t, helm.Form{
		Release:   &a.helmRelease,
		Chart:     &a.helmChart,
		Namespace: &a.helmNamespace,
		Values:    a.helmValues,
		Diff:      a.helmDiff,
		Err:       a.helmErr,
		Dry:       a.helmDry,
		Loading:   a.helmBusy,
		OnDryRun: func() {
			a.helmDry = true
			a.runHelm(true)
		},
		OnCancel: func() {
			a.helmDry = false
			a.helmDiff = nil
			a.helmBusy = false
			a.stash()
		},
		OnInstall: func() {
			a.runHelm(false)
			a.helmDry = false
			a.stash()
		},
	})
}

func (a *App) runHelm(dry bool) {
	req := cluster.HelmRequest{
		Release:   a.helmRelease,
		Chart:     a.helmChart,
		Namespace: a.helmNamespace,
		DryRun:    dry,
	}
	if a.helmValues != nil && a.helmValues.Buffer != nil {
		req.Values = a.helmValues.Buffer.String()
	}
	if !dry && !a.helmDry {
		return
	}
	a.ensureTabs()
	owner := a.active
	a.helmBusy = true
	a.stash()
	a.invalidate()
	go func() {
		res, err := a.session.HelmInstall(context.Background(), req)
		a.mu.Lock()
		defer a.mu.Unlock()
		b := a.bodies[owner]
		if b == nil {
			return
		}
		b.helmBusy = false
		b.helmErr = ""
		if err != nil {
			b.helmErr = publicError(err)
		} else if dry && res != nil {
			old, newBuf := editor.NewBuffer(""), editor.NewBuffer(res.Manifest)
			b.helmDiff = &editor.DiffState{Old: old, New: newBuf}
		}
		if a.active == owner {
			a.helmBusy = b.helmBusy
			a.helmErr = b.helmErr
			a.helmDiff = b.helmDiff
		}
		a.invalidate()
	}()
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

type yamlScreen struct {
	Identity cluster.Identity
	State    *editor.State
	Diff     *editor.DiffState
	Confirm  YAMLConfirm
	Err      string
	Pending  bool
}

func (a *App) applyYAML(id cluster.Identity, body string) error {
	return a.session.Apply(context.Background(), id, body)
}

var _ io.Closer = (*cluster.ExecConn)(nil)
