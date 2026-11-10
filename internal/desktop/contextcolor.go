package desktop

import (
	"hash/fnv"

	"github.com/egoist/mygo/ui"
)

// contextPalette holds the fixed identity colors for kube contexts, in the
// spirit of colored cluster icons. Mid luminance so they read on both
// light and dark appearances.
var contextPalette = [8]ui.Color{
	ui.Hex("#ef4444"), // red
	ui.Hex("#f97316"), // orange
	ui.Hex("#f59e0b"), // amber
	ui.Hex("#84cc16"), // lime
	ui.Hex("#14b8a6"), // teal
	ui.Hex("#3b82f6"), // blue
	ui.Hex("#8b5cf6"), // violet
	ui.Hex("#ec4899"), // pink
}

// contextColor maps a context name to its identity color. The mapping is a
// plain hash so the same name always lands on the same color.
func contextColor(name string) ui.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return contextPalette[h.Sum32()%uint32(len(contextPalette))]
}
