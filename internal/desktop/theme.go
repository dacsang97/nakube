package desktop

// Appearance is light or dark. Dark swaps colors only; the layout stays.
type Appearance string

const (
	AppearanceLight Appearance = "light"
	AppearanceDark  Appearance = "dark"
)

// DefaultAppearance is light. There is no theme gallery.
func DefaultAppearance() Appearance { return AppearanceLight }

// NextAppearance toggles between the two appearances.
func NextAppearance(a Appearance) Appearance {
	if a == AppearanceDark {
		return AppearanceLight
	}
	return AppearanceDark
}
