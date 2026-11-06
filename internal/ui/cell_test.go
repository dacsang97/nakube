package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestChosenRowKeepsChipAndTextApart(t *testing.T) {
	light := ui.LightTheme()
	dark := ui.DarkTheme()
	for _, theme := range []*ui.Theme{light, dark} {
		fill, ink := chipFace(theme, ToneOK, true)
		if fill.A != 255 {
			t.Fatalf("chosen chip is translucent on %v", theme.Dark)
		}
		if fill != theme.AccentText {
			t.Fatal("chosen chip did not sit on an opaque face")
		}
		if ink != theme.Success {
			t.Fatal("chosen chip lost its tone")
		}
		mutedFill, mutedInk := chipFace(theme, ToneMuted, true)
		if mutedFill.A != 255 || mutedInk == theme.AccentText {
			t.Fatalf("muted chip still sinks: fill %v ink %v", mutedFill, mutedInk)
		}
	}
	if rowInk(light, ToneBad, true) != light.AccentText {
		t.Fatal("chosen status kept a color that disappears on the accent")
	}
	if rowInk(light, ToneBad, false) != light.Danger {
		t.Fatal("unselected status lost its tone")
	}
}

func rowInk(t *ui.Theme, tone Tone, on bool) ui.Color {
	if t == nil {
		return ui.Color{}
	}
	if on {
		return t.AccentText
	}
	if tone == ToneNone {
		return t.Text
	}
	return tone.Color(t)
}
