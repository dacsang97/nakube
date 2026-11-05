package term

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Page is the exec terminal. The shell owns the connection.
type Page struct {
	Err    string
	Term   *terminal.Terminal
	OnBack func()
}

// Draw paints the exec screen.
func Draw(c *ui.Context, t *ui.Theme, page Page) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() && page.OnBack != nil {
			page.OnBack()
		}
		ui.Text(c, "Exec").FontSize(22).Bold()
	})
	if page.Err != "" {
		ui.Text(c, page.Err).TextColor(t.Danger)
		return
	}
	if page.Term == nil {
		ui.Text(c, "Connecting").TextColor(t.TextMuted)
		return
	}
	terminal.View(c, page.Term).Fill()
}
