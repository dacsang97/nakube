package ui

import "github.com/egoist/mygo/ui"

// Column is one board column. Width 0 shares the room the fixed columns leave.
// ID is the sort key; it must stay stable when a screen reorders columns.
type Column struct {
	ID, Title       string
	Width, MinWidth float32
	Sortable        bool
	Align           ui.Align
}

// Selection reports the current table row and whether this frame changed it.
type Selection struct {
	Index   int
	Valid   bool
	Changed bool
}

// Board is a searchable resource list. Screens own row semantics and render
// selected details in the shell panel. A nil Query hides the field. A nil
// Filter, or no Filters, hides the switch.
type Board struct {
	Title   string
	Count   string
	Err     string
	Notice  string
	Empty   string
	Loading bool
	Query   *string
	Hint    string
	Filters []string
	Filter  *int
	Columns []Column
	Rows    int
	State   *ui.ListState
	Cell    func(c *ui.Context, row, col int)
}

// DrawBoard paints a board. It does not load, search, or sort the rows.
func DrawBoard(c *ui.Context, t *ui.Theme, b Board) Selection {
	changed := false
	ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Gap(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
			ui.Text(c, b.Title).FontSize(22).Bold()
			if b.Count != "" {
				ui.Text(c, b.Count).TextColor(t.TextMuted)
			}
		})
		if b.Query != nil || (len(b.Filters) > 0 && b.Filter != nil) {
			toolbar(c, b)
		}
		switch {
		case b.Err != "" && b.Rows == 0 && !b.Loading:
			ui.Text(c, b.Err).TextColor(t.Danger)
		case b.Loading && b.Rows == 0:
			boardSkeleton(c, t, b.Columns)
		case b.Rows == 0:
			note(c, t, b)
			text := b.Empty
			if text == "" {
				text = "None"
			}
			ui.Text(c, text).TextColor(t.TextMuted)
		default:
			note(c, t, b)
			changed = boardBody(c, b)
		}
	})
	index := selectedRow(b)
	return Selection{Index: index, Valid: index >= 0, Changed: changed}
}

func toolbar(c *ui.Context, b Board) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if b.Query != nil {
			hint := b.Hint
			if hint == "" {
				hint = "Search"
			}
			ui.SearchField(c, b.Query).Grow(1).MinWidth(160).Label(hint)
		}
		if len(b.Filters) > 0 && b.Filter != nil {
			ui.Segmented(c, b.Filter, b.Filters...).Label("Filter")
		}
	})
}

func note(c *ui.Context, t *ui.Theme, b Board) {
	if b.Err != "" {
		ui.Text(c, b.Err).TextColor(t.Danger)
	}
	if b.Notice != "" {
		ui.Text(c, b.Notice).TextColor(t.TextMuted)
	}
}

func boardBody(c *ui.Context, b Board) bool {
	return ui.Table(c, b.State, tableColumns(b.Columns), b.Rows, func(row, col int) {
		if b.Cell != nil {
			b.Cell(c, row, col)
		}
	}).Grow(1).MinWidth(0).FillHeight().Changed()
}

func selectedRow(b Board) int {
	if b.State == nil || b.State.Selected == nil || b.Rows == 0 {
		return -1
	}
	i := *b.State.Selected
	if i < 0 || i >= b.Rows {
		return -1
	}
	return i
}

func tableColumns(cols []Column) []ui.TableColumn {
	out := make([]ui.TableColumn, len(cols))
	for i, col := range cols {
		out[i] = ui.TableColumn{
			ID:       col.ID,
			Title:    col.Title,
			Width:    col.Width,
			MinWidth: col.MinWidth,
			Sortable: col.Sortable,
			Align:    col.Align,
		}
	}
	return out
}

func boardSkeleton(c *ui.Context, t *ui.Theme, cols []Column) {
	titles := make([]string, len(cols))
	for i, col := range cols {
		titles[i] = col.Title
	}
	if len(titles) == 0 {
		titles = []string{"", "", ""}
	}
	Status(c, "Loading").Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			for _, title := range titles {
				title := title
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					if title != "" {
						ui.Text(c, title).FontSize(12).TextColor(t.TextMuted).NoWrap()
						return
					}
					Bar(c, t, 72, 12)
				})
			}
		})
		for range 6 {
			ui.Row(c).Gap(16).Children(func() {
				for range titles {
					Bar(c, t, 72, 12)
				}
			})
		}
	})
}
