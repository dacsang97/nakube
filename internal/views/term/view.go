package term

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
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
		termSkeleton(c, t)
		return
	}
	terminal.View(c, page.Term).Fill()
}

func termSkeleton(c *ui.Context, t *ui.Theme) {
	kit.Status(c, "Connecting").FillHeight().Radius(8).Background(t.Surface).Border(1, t.Border).Padding(12).Children(func() {
		kit.Bar(c, t, 180, 12)
		kit.Bar(c, t, 260, 12)
		kit.Bar(c, t, 120, 12)
	})
}
