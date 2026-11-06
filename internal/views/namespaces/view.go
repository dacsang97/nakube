package namespaces

import (
	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	colStatus = iota
	colName
	colLabels
	colAge
)

// Draw paints the namespace list. It does not load it.
func Draw(c *ui.Context, t *ui.Theme, page Page, query *string, filter *int, state *ui.ListState, loading bool, actions func(*ui.Context)) {
	empty := "None"
	if page.Total > 0 && len(page.Rows) == 0 {
		empty = "No matches"
	}
	kit.DrawBoard(c, t, kit.Board{
		Title:   "Namespaces",
		Count:   kit.Count(len(page.Rows), page.Total),
		Empty:   empty,
		Loading: loading,
		Query:   query,
		Hint:    "Search namespaces",
		Filters: Filters(),
		Filter:  filter,
		Columns: columns(),
		Rows:    len(page.Rows),
		State:   state,
		Cell: func(c *ui.Context, row, col int) {
			cell(c, t, page.Rows[row], col, kit.Chosen(state, row, col))
		},
		Detail: func(c *ui.Context) {
			if state == nil || state.Selected == nil {
				return
			}
			i := *state.Selected
			if i < 0 || i >= len(page.Rows) {
				return
			}
			kit.DetailView(c, t, page.Rows[i].Detail, actions)
		},
	})
}

func columns() []kit.Column {
	return []kit.Column{
		{ID: ColStatus, Title: "Status", Width: 110, Sortable: true},
		{ID: ColName, Title: "Name", MinWidth: 140, Sortable: true},
		{ID: ColLabels, Title: "Labels", MinWidth: 180, Sortable: true},
		{ID: ColAge, Title: "Age", Width: 56, Align: ui.End, Sortable: true},
	}
}

func cell(c *ui.Context, t *ui.Theme, row Row, col int, on bool) {
	switch col {
	case colStatus:
		kit.Line(c, t, row.Status, row.Tone, on)
	case colName:
		kit.Line(c, t, row.Name, kit.ToneNone, on)
	case colLabels:
		kit.Chips(c, t, labelChips(row.Labels), on)
	case colAge:
		kit.Line(c, t, row.Age, kit.ToneMuted, on)
	}
}
