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
	Title         string
	Count         string
	Err           string
	Notice        string
	Empty         string
	Loading       bool
	Query         *string
	Hint          string
	Filters       []string
	Filter        *int
	Columns       []Column
	Rows          int
	State         *ui.ListState
	Cell          func(c *ui.Context, row, col int)
	HeaderControl func(c *ui.Context, t *ui.Theme)
}

// PageHeader keeps a page title, optional count, and controls in one compact
// row. It wraps only when the window cannot fit the full header.
func PageHeader(c *ui.Context, t *ui.Theme, title, count string, control func(*ui.Context, *ui.Theme)) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, title).FontSize(22).Bold()
		if count != "" {
			ui.Text(c, count).TextColor(t.TextMuted)
		}
		if control != nil {
			control(c, t)
		}
	})
}

// DrawBoard paints a board. It does not load, search, or sort the rows.
func DrawBoard(c *ui.Context, t *ui.Theme, b Board) Selection {
	changed := false
	ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Gap(12).Children(func() {
		PageHeader(c, t, b.Title, b.Count, b.HeaderControl)
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
	columns := tableColumns(cols)
	if len(columns) == 0 {
		columns = []ui.TableColumn{{}, {}, {}}
	}
	for i := range columns {
		columns[i].Sortable = false
	}
	Status(c, "Loading").MinHeight(0).Children(func() {
		ui.Table(c, nil, columns, 6, func(_, _ int) {
			Bar(c, t, 0, 12)
		}).Grow(1).MinWidth(0).FillHeight()
	})
}
