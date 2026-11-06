package ui

import "github.com/egoist/mygo/ui"

// Chip is one short label. Tone colors it; None stays quiet.
type Chip struct {
	Text string
	Tone Tone
}

// Chips draws labels on one clipped line. An empty list is an em dash,
// not a blank hole. on means the row is chosen: a translucent wash would
// disappear into the accent, so the chip sits on an opaque face instead.
func Chips(c *ui.Context, t *ui.Theme, chips []Chip, on bool) {
	if len(chips) == 0 {
		Line(c, t, "", ToneMuted, on)
		return
	}
	ui.Row(c).Gap(4).MinWidth(0).Clip().Children(func() {
		for _, chip := range chips {
			text, tone := chip.Text, chip.Tone
			if text == "" {
				continue
			}
			if tone == ToneNone {
				tone = ToneMuted
			}
			fill, ink := chipFace(t, tone, on)
			e := ui.Text(c, text).FontSize(11).NoWrap().TextColor(ink).Padding(1, 6).Radius(8).Background(fill)
			if on && t != nil {
				e.Border(1, tone.Color(t))
			}
		}
	})
}

func chipFace(t *ui.Theme, tone Tone, on bool) (fill, ink ui.Color) {
	if t == nil {
		return ui.Color{}, ui.Color{}
	}
	color := tone.Color(t)
	if !on {
		return Tint(t, color), color
	}
	ink = color
	if tone == ToneNone || tone == ToneMuted {
		ink = ui.Hex("#18181b")
	}
	return t.AccentText, ink
}
