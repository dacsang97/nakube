package manifest

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Page is the YAML editor. Dry-run and apply stay with the shell.
type Page struct {
	Err       string
	Kind      string
	State     *editor.State
	Diff      *editor.DiffState
	OnBack    func()
	OnDryRun  func()
	OnReveal  func()
	OnCancel  func()
	OnConfirm func()
}

// Draw paints the editor or the dry-run diff.
func Draw(c *ui.Context, t *ui.Theme, page Page) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() && page.OnBack != nil {
			page.OnBack()
		}
		ui.Text(c, "YAML").FontSize(22).Bold()
		if ui.Button(c, "Dry-run").Clicked() && page.OnDryRun != nil {
			page.OnDryRun()
		}
		if page.Kind == "Secret" && ui.Button(c, "Reveal").Clicked() && page.OnReveal != nil {
			page.OnReveal()
		}
	})
	if page.Err != "" {
		ui.Text(c, page.Err).TextColor(t.Danger)
	}
	if page.State == nil {
		ui.Text(c, "Loading").TextColor(t.TextMuted)
		return
	}
	if page.Diff != nil {
		editor.DiffView(c, page.Diff).Fill()
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "Cancel").Clicked() && page.OnCancel != nil {
				page.OnCancel()
			}
			if ui.Button(c, "Confirm").Clicked() && page.OnConfirm != nil {
				page.OnConfirm()
			}
		})
		return
	}
	editor.View(c, page.State).Fill()
}
