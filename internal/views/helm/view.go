package helm

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Form is the install panel under the release list.
type Form struct {
	Release   *string
	Chart     *string
	Namespace *string
	Values    *editor.State
	Diff      *editor.DiffState
	Err       string
	Dry       bool
	Loading   bool
	OnDryRun  func()
	OnCancel  func()
	OnInstall func()
}

// Draw paints the install form. It does not call Helm.
func Draw(c *ui.Context, t *ui.Theme, form Form) {
	ui.Text(c, "Install").FontSize(16).Bold()
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, form.Release).Placeholder("release").Label("Release").Width(140)
		ui.TextInput(c, form.Chart).Placeholder("chart").Label("Chart").Width(180)
		ui.TextInput(c, form.Namespace).Placeholder("namespace").Label("Namespace").Width(120)
	})
	if form.Values != nil {
		editor.View(c, form.Values).Height(120)
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Dry-run").Clicked() && form.OnDryRun != nil {
			form.OnDryRun()
		}
		if ui.Button(c, "Cancel").Clicked() && form.OnCancel != nil {
			form.OnCancel()
		}
		if ui.Button(c, "Install").Clicked() && form.Dry && form.OnInstall != nil {
			form.OnInstall()
		}
	})
	if form.Loading && form.Diff == nil {
		kit.Status(c, "Loading").Children(func() {
			kit.Block(c, t, 160, 8)
		})
	}
	if form.Diff != nil {
		editor.DiffView(c, form.Diff).Height(160)
	}
	if form.Err != "" {
		ui.Text(c, form.Err).TextColor(t.Danger)
	}
}
