package ui

import "github.com/egoist/mygo/ui"

// Series is the overview palette. Usage, requests, and limits stay apart
// in both themes. Resource accents color a card's title, not those series.
type Series struct {
	Usage       ui.Color
	Requests    ui.Color
	Limits      ui.Color
	Allocatable ui.Color
	Capacity    ui.Color
	CPU         ui.Color
	Memory      ui.Color
	Pods        ui.Color
}

// Colors returns the series palette. Tracks and wells come from the theme
// so a dark window does not keep a light card fill.
func Colors() Series {
	return Series{
		Usage:       ui.Hex("#c23b8a"),
		Requests:    ui.Hex("#1f9d55"),
		Limits:      ui.Hex("#3b6fe0"),
		Allocatable: ui.Hex("#d4922a"),
		Capacity:    ui.Hex("#8b93a1"),
		CPU:         ui.Hex("#2f6fed"),
		Memory:      ui.Hex("#1fa971"),
		Pods:        ui.Hex("#0f9aa8"),
	}
}

// Tint is a quiet wash of c, stronger on a dark theme so the well still reads.
func Tint(t *ui.Theme, c ui.Color) ui.Color {
	if t != nil && t.Dark {
		return ui.RGBA(c.R, c.G, c.B, 0.18)
	}
	return ui.RGBA(c.R, c.G, c.B, 0.10)
}

// Track is the unused part of a ring.
func Track(t *ui.Theme) ui.Color {
	if t != nil && t.Dark {
		return ui.RGBA(255, 255, 255, 0.10)
	}
	return ui.RGBA(15, 23, 42, 0.08)
}
