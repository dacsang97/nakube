package ui

import "github.com/egoist/mygo/ui"

// Line paints one table cell. An empty string is an em dash, not a blank.
// on means the row is chosen: its face is the accent, so a tone color
// would sink into it. The text takes the accent's own text color instead.
func Line(c *ui.Context, t *ui.Theme, text string, tone Tone, on bool) {
	if text == "" {
		text = "—"
	}
	e := ui.Text(c, text).SingleLine()
	if t == nil {
		return
	}
	if on {
		e.TextColor(t.AccentText)
		return
	}
	if tone != ToneNone {
		e.TextColor(tone.Color(t))
	}
}

// UsageBar paints value against hi. Unknown, or a zero hi, is an em dash:
// a missing reading is not drawn as an empty success bar.
func UsageBar(c *ui.Context, t *ui.Theme, value, hi float64, known bool, tip string, on bool) {
	if !known || hi <= 0 {
		Line(c, t, "", ToneMuted, on)
		return
	}
	ui.Meter(c, value, 0, hi, &ui.MeterLevels{Warning: 0.8 * hi, Critical: 0.95 * hi}).Height(8).Grow(1).MinWidth(0).Tooltip(tip).Label(tip)
}

// Chosen reports whether the table cell being built is on the chosen row.
func Chosen(state *ui.ListState, row, col int) bool {
	return state != nil && state.CellSelected(row, col)
}
