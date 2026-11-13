package term

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/agent"
	kit "github.com/dacsang97/nakube/internal/ui"
)

// Page is the exec terminal. The shell owns the connection.
type Page struct {
	Err        string
	Term       *terminal.Terminal
	Restarting bool
	Agent      agent.Identity
	OnBack     func()
	OnRestart  func()
}

// Draw paints the exec screen.
func Draw(c *ui.Context, t *ui.Theme, page Page) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "Back").Clicked() && page.OnBack != nil {
			page.OnBack()
		}
		ui.Text(c, "Exec").FontSize(22).Bold()
		if page.Agent.ID != "" {
			ui.Row(c).Padding(3, 8).Radius(999).Gap(5).AlignItems(ui.Center).Background(t.SurfaceHover).
				Label("Coding agent: " + page.Agent.Name).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(t.Success)
				ui.Text(c, page.Agent.Name).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
			})
		}
		ui.Spacer(c)
		if page.Err != "" && page.OnRestart != nil {
			if ui.Button(c, "Restart shell").Clicked() {
				page.OnRestart()
			}
		}
	})
	switch {
	case page.Err != "":
		terminalNotice(c, t, "Disconnected", page.Err, t.Danger)
	case page.Term == nil && page.Restarting:
		terminalNotice(c, t, "Restarting shell…", "Reconnecting to the pod.", t.TextMuted)
	case page.Term == nil:
		termSkeleton(c, t)
	default:
		terminal.View(c, page.Term).Fill()
	}
}

func terminalNotice(c *ui.Context, t *ui.Theme, title, detail string, color ui.Color) {
	ui.Column(c).Grow(1).FillHeight().AlignItems(ui.Center).Justify(ui.Center).Gap(8).
		Radius(8).Background(t.Surface).Children(func() {
		ui.Text(c, title).FontSize(16).FontWeight(650).TextColor(color)
		ui.Text(c, detail).FontSize(13).TextColor(t.TextMuted)
	})
}

func termSkeleton(c *ui.Context, t *ui.Theme) {
	kit.Status(c, "Connecting").FillHeight().Radius(8).Background(t.Surface).Border(1, t.Border).Padding(12).Children(func() {
		kit.Bar(c, t, 180, 12)
		kit.Bar(c, t, 260, 12)
		kit.Bar(c, t, 120, 12)
	})
}
