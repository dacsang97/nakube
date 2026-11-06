package ui

import "github.com/egoist/mygo/ui"

// Tone is how a status should read. None is ordinary text.
type Tone int

const (
	ToneNone Tone = iota
	ToneMuted
	ToneOK
	ToneWarn
	ToneBad
)

// Color is the theme color for a tone. None is the primary text color.
func (tone Tone) Color(t *ui.Theme) ui.Color {
	if t == nil {
		return ui.Color{}
	}
	switch tone {
	case ToneOK:
		return t.Success
	case ToneWarn:
		return t.Warning
	case ToneBad:
		return t.Danger
	case ToneMuted:
		return t.TextMuted
	default:
		return t.Text
	}
}
