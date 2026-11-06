package ui

import "github.com/egoist/mygo/ui"

// Fact is one labeled value in a detail pane. An empty value is an em dash.
type Fact struct {
	Label string
	Value string
	Tone  Tone
}

// Group is one titled block of facts, chips, or lines.
type Group struct {
	Title string
	Facts []Fact
	Chips []Chip
	Lines []string
}

// Detail is the selected row: a title, facts, then groups.
type Detail struct {
	Title  string
	Tone   Tone
	Facts  []Fact
	Groups []Group
}

// Heading paints a quiet section name.
func Heading(c *ui.Context, t *ui.Theme, text string) {
	if text == "" {
		return
	}
	ui.Text(c, text).FontSize(12).Bold().TextColor(t.TextMuted)
}

// Facts paints label and value rows. The label keeps a fixed width so the
// values line up.
func Facts(c *ui.Context, t *ui.Theme, facts []Fact) {
	for _, fact := range facts {
		label, value, tone := fact.Label, fact.Value, fact.Tone
		if value == "" {
			value = "—"
		}
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Text(c, label).TextColor(t.TextMuted).Width(108).Shrink(0)
			e := ui.Text(c, value).Grow(1).MinWidth(0)
			if tone != ToneNone {
				e.TextColor(tone.Color(t))
			}
		})
	}
}

// DetailView paints a selected object's title and body.
func DetailView(c *ui.Context, t *ui.Theme, d Detail, actions func(*ui.Context)) {
	if d.Title != "" || actions != nil {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if d.Title != "" {
				e := ui.Text(c, d.Title).FontSize(16).Bold().Grow(1).MinWidth(0)
				if d.Tone != ToneNone && t != nil {
					e.TextColor(d.Tone.Color(t))
				}
			}
			if actions != nil {
				actions(c)
			}
		})
	}
	DetailBody(c, t, d, nil)
}

// DetailBody paints facts and groups below a shell-owned title.
func DetailBody(c *ui.Context, t *ui.Theme, d Detail, actions func(*ui.Context)) {
	if actions != nil {
		actions(c)
	}
	Facts(c, t, d.Facts)
	for _, group := range d.Groups {
		if group.empty() {
			continue
		}
		title, facts, chips, lines := group.Title, group.Facts, group.Chips, group.Lines
		Heading(c, t, title)
		Facts(c, t, facts)
		if len(chips) > 0 {
			Chips(c, t, chips, false)
		}
		for _, line := range lines {
			line := line
			ui.Text(c, line)
		}
	}
}

func (g Group) empty() bool {
	return len(g.Facts) == 0 && len(g.Chips) == 0 && len(g.Lines) == 0
}
