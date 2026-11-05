package logs

import "github.com/egoist/mygo/ui"

// Line is one log row already filtered by the shell.
type Line struct {
	Pod   string
	Text  string
	Color ui.Color
}

// Page is the log stream. Follow and Filter are shell state.
type Page struct {
	Pod     string
	Filter  *string
	Follow  *bool
	Invalid bool
	Lines   []Line
	OnBack  func()
}

// Draw paints the log stream.
func Draw(c *ui.Context, t *ui.Theme, page Page) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Back").Clicked() && page.OnBack != nil {
			page.OnBack()
		}
		ui.Text(c, "Logs").FontSize(22).Bold()
		if page.Pod != "" {
			ui.Text(c, page.Pod).TextColor(t.TextMuted)
		}
		ui.TextInput(c, page.Filter).Placeholder("regex").Width(180)
		label := "Follow on"
		if page.Follow != nil && !*page.Follow {
			label = "Follow off"
		}
		if ui.Button(c, label).Clicked() && page.Follow != nil {
			*page.Follow = !*page.Follow
		}
	})
	if page.Invalid {
		ui.Text(c, "Invalid regex").TextColor(t.Danger)
		return
	}
	if len(page.Lines) == 0 {
		ui.Text(c, "No log lines").TextColor(t.TextMuted)
		return
	}
	follow := page.Follow != nil && *page.Follow
	ui.Table(c, &ui.ListState{FollowEnd: follow}, []ui.TableColumn{
		{Title: "Pod", Width: 140},
		{Title: "Line"},
	}, len(page.Lines), func(row, col int) {
		line := page.Lines[row]
		text := line.Pod
		if col == 1 {
			text = line.Text
		}
		ui.Text(c, text).TextColor(line.Color)
	}).Fill()
}
