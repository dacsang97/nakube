package resource

import (
	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Row is one table row. Kind chooses the action buttons.
type Row struct {
	Kind  string
	Cells []string
}

// List is a generic resource table plus the actions for the selected row.
type List struct {
	Title    string
	Err      string
	Notice   string
	Empty    string
	Loading  bool
	Columns  []string
	Rows     []Row
	Selected *int
	ScaleTo  *string
	Local    *string
	Remote   *string
	Revision *string
	OnAction func(index int, action string)
}

// Draw paints the list. It does not load it.
func Draw(c *ui.Context, t *ui.Theme, list List) {
	ui.Text(c, list.Title).FontSize(22).Bold()
	switch {
	case list.Err != "":
		ui.Text(c, list.Err).TextColor(t.Danger)
	case list.Notice != "":
		ui.Text(c, list.Notice).TextColor(t.TextMuted)
	case list.Loading && len(list.Rows) == 0:
		tableSkeleton(c, t, list.Columns)
	case len(list.Rows) == 0:
		text := list.Empty
		if text == "" {
			text = "None"
		}
		ui.Text(c, text).TextColor(t.TextMuted)
	}
	if len(list.Rows) == 0 || list.Selected == nil {
		return
	}
	if *list.Selected < 0 || *list.Selected >= len(list.Rows) {
		*list.Selected = 0
	}
	row := list.Rows[*list.Selected]
	inputs(c, row.Kind, list)
	actions(c, t, *list.Selected, row.Kind, list.OnAction)
	ui.Table(c, &ui.ListState{Selected: list.Selected}, columns(list.Columns), len(list.Rows), func(i, col int) {
		ui.Text(c, list.Rows[i].Cells[col])
	}).Fill()
}

func inputs(c *ui.Context, kind string, list List) {
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		ui.TextInput(c, list.ScaleTo).Label("Replicas").Width(80)
	case "Pod", "Service":
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, list.Local).Label("Local").Width(80)
			ui.TextInput(c, list.Remote).Label("Remote").Width(80)
		})
	case "Application", "Release":
		ui.TextInput(c, list.Revision).Label("Revision").Width(80)
	}
}

func actions(c *ui.Context, t *ui.Theme, index int, kind string, on func(int, string)) {
	Buttons(c, t, kind, func(action string) {
		if on != nil {
			on(index, action)
		}
	})
}

// Buttons paints the actions for a kind. The caller owns the selected row.
// Delete is danger-colored so it is not a peer of the other actions.
func Buttons(c *ui.Context, t *ui.Theme, kind string, on func(string)) {
	names := Actions(kind)
	if len(names) == 0 {
		return
	}
	ui.Row(c).Gap(6).Shrink(0).Children(func() {
		for _, action := range names {
			b := ui.Button(c, actionLabel(action)).Shrink(0)
			if action == "delete" && t != nil {
				b.TextColor(t.Danger)
			}
			if b.Clicked() && on != nil {
				on(action)
			}
		}
	})
}

func Actions(kind string) []string {
	switch kind {
	case "Pod":
		return []string{"logs", "exec", "yaml", "forward", "delete"}
	case "Deployment", "StatefulSet", "ReplicaSet":
		return []string{"yaml", "scale", "restart", "delete"}
	case "DaemonSet":
		return []string{"yaml", "restart", "delete"}
	case "Service":
		return []string{"yaml", "forward", "delete"}
	case "Application":
		return []string{"sync", "rollback", "yaml", "delete"}
	case "Release":
		return []string{"rollback", "uninstall"}
	case "Forward":
		return []string{"stop"}
	case "":
		return nil
	default:
		return []string{"yaml", "delete"}
	}
}

func actionLabel(action string) string {
	switch action {
	case "logs":
		return "Logs"
	case "exec":
		return "Exec"
	case "yaml":
		return "YAML"
	case "forward":
		return "Forward"
	case "delete":
		return "Delete"
	case "scale":
		return "Scale"
	case "restart":
		return "Restart"
	case "sync":
		return "Sync"
	case "rollback":
		return "Rollback"
	case "uninstall":
		return "Uninstall"
	case "stop":
		return "Stop"
	default:
		return action
	}
}

func columns(titles []string) []ui.TableColumn {
	out := make([]ui.TableColumn, len(titles))
	for i, title := range titles {
		width := float32(140)
		switch title {
		case "Name", "Message", "Chart":
			width = 220
		case "Age", "Phase", "Sync", "Health", "Ready", "Local", "Remote", "Revision":
			width = 90
		}
		out[i] = ui.TableColumn{Title: title, Width: width}
	}
	return out
}

func tableSkeleton(c *ui.Context, t *ui.Theme, titles []string) {
	cols := titles
	if len(cols) == 0 {
		cols = []string{"", "", "", ""}
	}
	kit.Status(c, "Loading").Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			for i, title := range cols {
				headerCell(c, t, title, i)
			}
		})
		for range 6 {
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				for col := range cols {
					kit.Bar(c, t, barWidth(col), 12)
				}
			})
		}
	})
}

func headerCell(c *ui.Context, t *ui.Theme, title string, col int) {
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).TextColor(t.TextMuted).NoWrap()
			return
		}
		kit.Bar(c, t, barWidth(col), 12)
	})
}

func barWidth(col int) float32 {
	if col == 1 {
		return 140
	}
	return 72
}
