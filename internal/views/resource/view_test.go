package resource

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLoadingDrawsSkeleton(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), List{
			Title:    "Pods",
			Loading:  true,
			Columns:  []string{"Namespace", "Name", "Phase"},
			Selected: new(int),
		})
	}, 800, 600)
	if !tt.HasText("Pods") || !tt.HasText("Namespace") || !tt.HasText("Phase") {
		t.Fatalf("skeleton lost the table chrome %v", tt.Texts())
	}
	if tt.HasText("None") {
		t.Fatalf("loading list drew the empty state %v", tt.Texts())
	}
}

func TestEmptyStaysEmpty(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), List{Title: "Pods", Empty: "None"})
	}, 800, 600)
	if !tt.HasText("None") {
		t.Fatal("settled empty list hid its copy")
	}
}
