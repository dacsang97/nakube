package term

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDisconnectedTerminalOffersRestart(t *testing.T) {
	restarted := false
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), Page{
			Err:       "connection lost",
			OnRestart: func() { restarted = true },
		})
	}, 800, 600)
	for _, want := range []string{"Disconnected", "connection lost", "Restart shell"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if err := tt.Click("Restart shell"); err != nil {
		t.Fatal(err)
	}
	if !restarted {
		t.Fatal("restart action was not dispatched")
	}
}

func TestRestartingTerminalHasDistinctState(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), Page{Restarting: true})
	}, 800, 600)
	for _, want := range []string{"Restarting shell…", "Reconnecting to the pod."} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText("Connecting") {
		t.Fatal("restart fell back to the initial connection state")
	}
}
