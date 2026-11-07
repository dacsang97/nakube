package desktop

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestContextColorMapsNamesToPalette(t *testing.T) {
	// Identity stability: the pin locks FNV-1a + palette order. If this ever
	// moves, every user's context colors silently change.
	if got := contextColor("prod"); got != contextPalette[4] {
		t.Fatalf("prod left palette slot 4")
	}
	// Spread: 64 generated names must use at least 5 distinct palette
	// colors, so the list reads as Lens's colored cluster list.
	seen := map[ui.Color]bool{}
	names := []string{"", "dev", "prod", "arn:aws:eks:ap-southeast-1:645419842593:cluster/ght-hz"}
	for i := 0; i < 64; i++ {
		names = append(names, fmt.Sprintf("cluster-%02d", i))
	}
	for _, name := range names {
		seen[contextColor(name)] = true
	}
	if len(seen) < 5 {
		t.Fatalf("palette collapsed: %d distinct colors over %d names", len(seen), len(names))
	}
}
