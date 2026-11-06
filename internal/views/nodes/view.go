package nodes

import (
	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	colStatus = iota
	colName
	colRoles
	colCPU
	colMemory
	colVersion
	colConditions
	colTaints
	colAge
)

// Draw paints the node list. It does not load it.
func Draw(c *ui.Context, t *ui.Theme, page Page, query *string, filter *int, state *ui.ListState, loading bool) kit.Selection {
	empty := "None"
	if page.Total > 0 && len(page.Rows) == 0 {
		empty = "No matches"
	}
	return kit.DrawBoard(c, t, kit.Board{
		Title:   "Nodes",
		Count:   kit.Count(len(page.Rows), page.Total),
		Empty:   empty,
		Loading: loading,
		Query:   query,
		Hint:    "Search nodes",
		Filters: Filters(),
		Filter:  filter,
		Columns: columns(),
		Rows:    len(page.Rows),
		State:   state,
		Cell: func(c *ui.Context, row, col int) {
			cell(c, t, page.Rows[row], col, kit.Chosen(state, row, col))
		},
	})
}

func columns() []kit.Column {
	return []kit.Column{
		{ID: ColStatus, Title: "Status", Width: 88, Sortable: true},
		{ID: ColName, Title: "Name", MinWidth: 140, Sortable: true},
		{ID: ColRoles, Title: "Roles", Width: 120, Sortable: true},
		{ID: ColCPU, Title: "CPU", Width: 96, Sortable: true},
		{ID: ColMemory, Title: "Memory", Width: 96, Sortable: true},
		{ID: ColVersion, Title: "Version", Width: 88, Sortable: true},
		{ID: ColConditions, Title: "Conditions", Width: 160, Sortable: true},
		{ID: ColTaints, Title: "Taints", Width: 64, Align: ui.End, Sortable: true},
		{ID: ColAge, Title: "Age", Width: 56, Align: ui.End, Sortable: true},
	}
}

func cell(c *ui.Context, t *ui.Theme, row Row, col int, on bool) {
	switch col {
	case colStatus:
		kit.Line(c, t, row.Status, row.StatusTone, on)
	case colName:
		kit.Line(c, t, row.Name, kit.ToneNone, on)
	case colRoles:
		kit.Line(c, t, row.Roles, kit.ToneNone, on)
	case colCPU:
		kit.UsageBar(c, t, row.CPU, row.CPUMax, row.CPUKnown, row.CPUTip, on)
	case colMemory:
		kit.UsageBar(c, t, row.Mem, row.MemMax, row.MemKnown, row.MemTip, on)
	case colVersion:
		kit.Line(c, t, row.Version, kit.ToneNone, on)
	case colConditions:
		kit.Chips(c, t, row.Conditions, on)
	case colTaints:
		kit.Line(c, t, row.Taints, kit.ToneMuted, on)
	case colAge:
		kit.Line(c, t, row.Age, kit.ToneMuted, on)
	}
}
