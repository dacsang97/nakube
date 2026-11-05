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
		Lines:   lines,
		OnBack:  a.leaveOverlay,
	})
}

func (a *App) openLogs(namespace, name, container string) {
	if a.logCancel != nil {
		a.logCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.logCancel = cancel
	a.mode = "logs"
	a.logLines = nil
	a.logFollow = true
	a.logPod = name
	go func() {
		err := a.session.StreamLogs(ctx, namespace, name, container, true, 200, func(lines []string) {
			a.mu.Lock()
			for _, line := range lines {
				a.logLines = append(a.logLines, LogLine{Pod: name, Text: line})
			}
			a.mu.Unlock()
			a.invalidate()
		})
		if err != nil && ctx.Err() == nil {
			a.setErr(publicError(err))
		}
	}()
}

func (a *App) drawExec(c *ui.Context, t *ui.Theme) {
	term.Draw(c, t, term.Page{Err: a.execErr, Term: a.execTerm, OnBack: a.closeExec})
}

func (a *App) openExec(namespace, name, container string) {
	a.closeExec()
	a.mode = "exec"
	a.execErr = ""
	go func() {
		conn, err := a.session.Exec(context.Background(), namespace, name, container, []string{"/bin/sh"})
		if err != nil {
			a.mu.Lock()
			a.execErr = publicError(err)
			a.mu.Unlock()
			a.invalidate()
			return
		}
		term, err := terminal.New(terminal.Options{Conn: conn})
		if err != nil {
			_ = conn.Close()
			a.mu.Lock()
			a.execErr = publicError(err)
			a.mu.Unlock()
			a.invalidate()
			return
		}
		a.mu.Lock()
		if a.mode != "exec" {
			a.mu.Unlock()
			term.Close()
			_ = conn.Close()
			return
		}
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
	if a.mode == "exec" {
		a.mode = ""
	}
}

func (a *App) drawYAML(c *ui.Context, t *ui.Theme) {
	page := manifest.Page{OnBack: func() {
		a.mode = ""
		a.yaml = nil
	}, OnDryRun: a.dryRunYAML}
	if a.yaml != nil {
		page.Err = a.yaml.Err
		page.Kind = a.yaml.Identity.Kind
		page.State = a.yaml.State
		page.Diff = a.yaml.Diff
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
	a.mode = "yaml"
	a.yaml = &yamlScreen{Identity: id}
	reveal := a.secretReveal
	go func() {
		body, err := a.session.GetYAML(context.Background(), id)
		if err != nil {
			a.mu.Lock()
			if a.yaml != nil {
				a.yaml.Err = publicError(err)
			}
			a.mu.Unlock()
			a.invalidate()
			return
		}
		if id.Kind == "Secret" && !reveal {
			body = maskSecretYAML(body)
		}
		buf := editor.NewBuffer(body)
		buf.SetLanguage(editor.LanguageFor("resource.yaml"))
		a.mu.Lock()
		if a.yaml != nil && a.yaml.Identity == id {
			a.yaml.State = &editor.State{Buffer: buf}
			a.yaml.Err = ""
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
			a.yaml.Err = publicError(err)
			a.invalidate()
			return
		}
		a.yaml.Confirm = OpenYAML(preview, body)
		old, newBuf := editor.NewBuffer(preview.Live), editor.NewBuffer(preview.Predicted)
		lang := editor.LanguageFor("resource.yaml")
		old.SetLanguage(lang)
		newBuf.SetLanguage(lang)
		a.yaml.Diff = &editor.DiffState{Old: old, New: newBuf}
		a.yaml.Err = ""
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
		OnDryRun: func() {
			a.helmDry = true
			a.runHelm(true)
		},
		OnCancel: func() {
			a.helmDry = false
			a.helmDiff = nil
		},
		OnInstall: func() {
			a.runHelm(false)
			a.helmDry = false
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
	go func() {
		res, err := a.session.HelmInstall(context.Background(), req)
		a.mu.Lock()
		defer a.mu.Unlock()
		if err != nil {
			a.helmErr = publicError(err)
			a.invalidate()
			return
		}
		a.helmErr = ""
		if dry && res != nil {
			old, newBuf := editor.NewBuffer(""), editor.NewBuffer(res.Manifest)
			a.helmDiff = &editor.DiffState{Old: old, New: newBuf}
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
}

func (a *App) applyYAML(id cluster.Identity, body string) error {
	return a.session.Apply(context.Background(), id, body)
}

var _ io.Closer = (*cluster.ExecConn)(nil)
