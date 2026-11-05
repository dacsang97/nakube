package manifest

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	kit "github.com/dacsang97/nakube/internal/ui"
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
	Pending   bool
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
		if page.Err == "" {
			editorSkeleton(c, t)
		}
		return
	}
	if page.Pending && page.Diff == nil {
		editor.View(c, page.State).Height(160)
		editorSkeleton(c, t)
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

func editorSkeleton(c *ui.Context, t *ui.Theme) {
	widths := []float32{280, 160, 320, 220, 120, 260, 180, 300}
	kit.Status(c, "Loading").Children(func() {
		for _, width := range widths {
			kit.Bar(c, t, width, 10)
		}
	})
}
