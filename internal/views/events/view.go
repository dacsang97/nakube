package events

import (
	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	colType = iota
	colReason
	colObject
	colMessage
	colNamespace
	colSource
	colCount
	colAge
)

// Draw paints the event list. It does not load it.
func Draw(c *ui.Context, t *ui.Theme, page Page, query *string, state *ui.ListState, loading bool, actions func(*ui.Context)) {
	empty := "None"
	if page.Total > 0 && len(page.Rows) == 0 {
		empty = "No matches"
	}
	kit.DrawBoard(c, t, kit.Board{
		Title:   "Events",
		Count:   kit.Count(len(page.Rows), page.Total),
		Empty:   empty,
		Loading: loading,
		Query:   query,
		Hint:    "Search events",
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
		{ID: ColType, Title: "Type", Width: 88, Sortable: true},
		{ID: ColReason, Title: "Reason", Width: 120, Sortable: true},
		{ID: ColObject, Title: "Object", Width: 160, Sortable: true},
		{ID: ColMessage, Title: "Message", MinWidth: 180, Sortable: true},
		{ID: ColNamespace, Title: "Namespace", Width: 120, Sortable: true},
		{ID: ColSource, Title: "Source", Width: 140, Sortable: true},
		{ID: ColCount, Title: "Count", Width: 64, Align: ui.End, Sortable: true},
		{ID: ColAge, Title: "Age", Width: 56, Align: ui.End, Sortable: true},
	}
}

func cell(c *ui.Context, t *ui.Theme, row Row, col int, on bool) {
	switch col {
	case colType:
		kit.Line(c, t, row.Type, row.TypeTone, on)
	case colReason:
		kit.Line(c, t, row.Reason, kit.ToneNone, on)
	case colObject:
		kit.Line(c, t, row.Object, kit.ToneNone, on)
	case colMessage:
		tone := kit.ToneNone
		if row.Warning && !on {
			tone = kit.ToneBad
		}
		text(c, t, row.Message, tone, on)
	case colNamespace:
		kit.Line(c, t, row.Namespace, kit.ToneNone, on)
	case colSource:
		kit.Line(c, t, row.Source, kit.ToneMuted, on)
	case colCount:
		kit.Line(c, t, row.Count, kit.ToneMuted, on)
	case colAge:
		kit.Line(c, t, row.Age, kit.ToneMuted, on)
	}
}

func text(c *ui.Context, t *ui.Theme, value string, tone kit.Tone, on bool) {
	if value == "" {
		kit.Line(c, t, "", kit.ToneMuted, on)
		return
	}
	e := ui.Text(c, value).SingleLine().Tooltip(value)
	if t == nil {
		return
	}
	if on {
		e.TextColor(t.AccentText)
		return
	}
	if tone != kit.ToneNone {
		e.TextColor(tone.Color(t))
	}
}
