package pods

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Column positions, in the order columns returns them.
const (
	colName = iota
	colNamespace
	colContainers
	colCPU
	colMemory
	colRestarts
	colOwner
	colNode
	colQoS
	colAge
	colStatus
)

// Draw paints the pod list. onOpen follows a Controlled By or Node link to
// a screen and the key of the row to choose there. header sits beside the
// title, as the namespace selector.
func Draw(c *ui.Context, t *ui.Theme, page Page, query *string, state *ui.ListState, loading bool, now time.Time, onOpen func(screen, key string), header func(*ui.Context, *ui.Theme)) kit.Selection {
	empty := "None"
	if page.Total > 0 && len(page.Rows) == 0 {
		empty = "No matches"
	}
	return kit.DrawBoard(c, t, kit.Board{
		Title:         "Pods",
		Count:         kit.Count(len(page.Rows), page.Total),
		Empty:         empty,
		Loading:       loading,
		Query:         query,
		Hint:          "Search pods",
		HeaderControl: header,
		Columns:       columns(),
		Rows:          len(page.Rows),
		State:         state,
		Cell: func(c *ui.Context, row, col int) {
			cell(c, t, page.Rows[row], col, kit.Chosen(state, row, col), now, onOpen)
		},
	})
}

func columns() []kit.Column {
	return []kit.Column{
		{ID: ColName, Title: "Name", MinWidth: 180, Sortable: true},
		{ID: ColNamespace, Title: "Namespace", Width: 120, Sortable: true},
		{ID: ColContainers, Title: "Containers", Width: 96},
		{ID: ColCPU, Title: "CPU", Width: 72, Align: ui.End, Sortable: true},
		{ID: ColMemory, Title: "Memory", Width: 84, Align: ui.End, Sortable: true},
		{ID: ColRestarts, Title: "Restarts", Width: 96, Align: ui.End, Sortable: true},
		{ID: ColOwner, Title: "Controlled By", Width: 110, Sortable: true},
		{ID: ColNode, Title: "Node", Width: 160, Sortable: true},
		{ID: ColQoS, Title: "QoS", Width: 90, Sortable: true},
		{ID: ColAge, Title: "Age", Width: 56, Align: ui.End, Sortable: true},
		{ID: ColStatus, Title: "Status", Width: 120, Sortable: true},
	}
}

func cell(c *ui.Context, t *ui.Theme, row Row, col int, on bool, now time.Time, onOpen func(screen, key string)) {
	switch col {
	case colName:
		kit.Line(c, t, row.Name, kit.ToneNone, on)
	case colNamespace:
		kit.Line(c, t, row.Namespace, kit.ToneNone, on)
	case colContainers:
		containerDots(c, t, row.Dots)
	case colCPU:
		usage(c, t, row.Known, CPU(row.CPU), on)
	case colMemory:
		usage(c, t, row.Known, Memory(row.Mem), on)
	case colRestarts:
		tone := kit.ToneNone
		if row.Restarts > 0 {
			tone = kit.ToneWarn
		}
		kit.Line(c, t, strconv.Itoa(int(row.Restarts)), tone, on)
	case colOwner:
		link(c, t, row.Owner.Kind, row.Owner.Kind+" "+row.Owner.Name, row.Owner.Screen, row.Owner.Key, on, onOpen)
	case colNode:
		screen := ""
		if row.Node != "" {
			screen = "nodes"
		}
		link(c, t, row.Node, row.Node, screen, row.Node, on, onOpen)
	case colQoS:
		kit.Line(c, t, row.QoS, kit.ToneNone, on)
	case colAge:
		kit.Line(c, t, kit.Age(row.Created, now), kit.ToneMuted, on)
	case colStatus:
		kit.Line(c, t, row.Status, row.StatusTone, on)
	}
}

// usage is N/A when metrics-server has no reading, never a zero.
func usage(c *ui.Context, t *ui.Theme, known bool, text string, on bool) {
	if !known {
		kit.Line(c, t, "N/A", kit.ToneMuted, on)
		return
	}
	kit.Line(c, t, text, kit.ToneNone, on)
}

// containerDots draws a square per container. Init containers are hollow,
// so the app containers stand out.
func containerDots(c *ui.Context, t *ui.Theme, dots []Dot) {
	ui.Row(c).Grow(1).MinWidth(0).Wrap().Gap(3).AlignItems(ui.Center).Children(func() {
		for _, d := range dots {
			color := dotColor(t, d.State)
			box := ui.Box(c).Size(8, 8).Radius(1).Shrink(0).Tooltip(d.Text).Label(d.Text)
			if d.Init {
				box.Border(1.5, color)
			} else {
				box.Background(color)
			}
		}
	})
}

func dotColor(t *ui.Theme, state DotState) ui.Color {
	switch state {
	case DotReady:
		return t.Success
	case DotRunning, DotWaiting:
		return t.Warning
	case DotFailed:
		return t.Danger
	default:
		return t.TextMuted
	}
}

// link is text that opens screen at key. With no screen it is plain text.
func link(c *ui.Context, t *ui.Theme, text, tip, screen, key string, on bool, onOpen func(screen, key string)) {
	if screen == "" || onOpen == nil {
		kit.Line(c, t, text, kit.ToneNone, on)
		return
	}
	e := ui.ButtonBase(c).Role(ui.RoleButton).Grow(1).Shrink(1).MinWidth(40).MinHeight(40).
		Justify(ui.Start).Padding(1).Radius(4).Border(1, ui.Transparent).Tooltip(tip).Label("Open " + tip)
	color := t.Accent
	if on {
		color = t.AccentText
	} else if e.Hovered() {
		color = t.AccentHover
	}
	e.Children(func() {
		ui.Text(c, text).SingleLine().TextColor(color)
	})
	// The table owns Enter for row submission; the focused link must
	// claim it more narrowly. Register both keys on every frame.
	enter, space := e.Shortcut(0, ui.KeyEnter), e.Shortcut(0, ui.KeySpace)
	if e.Clicked() || enter || space {
		onOpen(screen, key)
	}
}
